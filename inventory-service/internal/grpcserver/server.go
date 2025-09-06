package main

import (
	"context"
	"database/sql"
	"time"

	pb "github.com/synapsis-challenge/gen/inventory/v1"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type InventoryServer struct {
	pb.UnimplementedInventoryServiceServer
	db     *sql.DB
	logger *zap.Logger
}

func NewInventoryServer(db *sql.DB, logger *zap.Logger) *InventoryServer {
	return &InventoryServer{db: db, logger: logger}
}

func (s *InventoryServer) CheckStock(ctx context.Context, req *pb.CheckStockRequest) (*pb.CheckStockResponse, error) {
	var total, reserved int
	err := s.db.QueryRowContext(ctx, `SELECT total_stock, reserved_stock FROM products WHERE sku=$1`, req.Sku).Scan(&total, &reserved)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, status.Error(codes.NotFound, "product not found")
		}
		s.logger.Error("checkstock db", zap.Error(err))
		return nil, status.Error(codes.Internal, "internal")
	}
	available := total - reserved
	return &pb.CheckStockResponse{Available: available >= int(req.Qty), AvailableQty: int32(available)}, nil
}

func (s *InventoryServer) ReserveStock(ctx context.Context, req *pb.ReserveStockRequest) (*pb.ReserveStockResponse, error) {
	if req.Qty <= 0 {
		return nil, status.Error(codes.InvalidArgument, "qty must be > 0")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		s.logger.Error("begin tx", zap.Error(err))
		return nil, status.Error(codes.Internal, "internal")
	}
	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		}
	}()

	if req.CorrelationId != "" {
		var existing string
		err = tx.QueryRowContext(ctx, `SELECT reservation_id FROM reservations WHERE reservation_id=$1`, req.CorrelationId).Scan(&existing)
		if err == nil {
			tx.Commit()
			return &pb.ReserveStockResponse{ReservationId: existing, Status: "ALREADY_RESERVED"}, nil
		} else if err != sql.ErrNoRows {
			tx.Rollback()
			s.logger.Error("idempotency lookup", zap.Error(err))
			return nil, status.Error(codes.Internal, "internal")
		}
	}

	var total, reserved int
	row := tx.QueryRowContext(ctx, `SELECT total_stock, reserved_stock FROM products WHERE sku=$1 FOR UPDATE`, req.Sku)
	if err := row.Scan(&total, &reserved); err != nil {
		tx.Rollback()
		if err == sql.ErrNoRows {
			return nil, status.Error(codes.NotFound, "product not found")
		}
		s.logger.Error("select product", zap.Error(err))
		return nil, status.Error(codes.Internal, "internal")
	}
	available := total - reserved
	if available < int(req.Qty) {
		tx.Rollback()
		return nil, status.Error(codes.FailedPrecondition, "insufficient stock")
	}

	var reservationID string
	if req.CorrelationId != "" {
		if _, err := uuid.Parse(req.CorrelationId); err == nil {
			reservationID = req.CorrelationId
		} else {
			reservationID = uuid.New().String()
		}
	} else {
		reservationID = uuid.New().String()
	}

	_, err = tx.ExecContext(ctx, `INSERT INTO reservations(reservation_id, sku, qty, status, created_at) VALUES ($1,$2,$3,$4,$5)`, reservationID, req.Sku, req.Qty, "RESERVED", time.Now())
	if err != nil {
		tx.Rollback()
		s.logger.Error("insert reservation", zap.Error(err))
		return nil, status.Error(codes.Internal, "internal")
	}

	_, err = tx.ExecContext(ctx, `UPDATE products SET reserved_stock = reserved_stock + $1 WHERE sku=$2`, req.Qty, req.Sku)
	if err != nil {
		tx.Rollback()
		s.logger.Error("update product reserved", zap.Error(err))
		return nil, status.Error(codes.Internal, "internal")
	}

	if err := tx.Commit(); err != nil {
		s.logger.Error("commit", zap.Error(err))
		return nil, status.Error(codes.Internal, "internal")
	}

	return &pb.ReserveStockResponse{ReservationId: reservationID, Status: "RESERVED"}, nil
}

func (s *InventoryServer) ReleaseStock(ctx context.Context, req *pb.ReleaseStockRequest) (*pb.ReleaseStockResponse, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		s.logger.Error("begin tx", zap.Error(err))
		return nil, status.Error(codes.Internal, "internal")
	}
	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		}
	}()

	var sku string
	var qty int
	row := tx.QueryRowContext(ctx, `SELECT sku, qty FROM reservations WHERE reservation_id=$1 AND status=$2 FOR UPDATE`, req.ReservationId, "RESERVED")
	if err := row.Scan(&sku, &qty); err != nil {
		tx.Rollback()
		if err == sql.ErrNoRows {
			return &pb.ReleaseStockResponse{Status: "NOT_FOUND_OR_ALREADY_RELEASED"}, nil
		}
		s.logger.Error("select reservation", zap.Error(err))
		return nil, status.Error(codes.Internal, "internal")
	}

	_, err = tx.ExecContext(ctx, `UPDATE reservations SET status=$1, updated_at=$2 WHERE reservation_id=$3`, "RELEASED", time.Now(), req.ReservationId)
	if err != nil {
		tx.Rollback()
		s.logger.Error("update reservation", zap.Error(err))
		return nil, status.Error(codes.Internal, "internal")
	}

	_, err = tx.ExecContext(ctx, `UPDATE products SET reserved_stock = reserved_stock - $1 WHERE sku=$2`, qty, sku)
	if err != nil {
		tx.Rollback()
		s.logger.Error("update product reserved", zap.Error(err))
		return nil, status.Error(codes.Internal, "internal")
	}

	if err := tx.Commit(); err != nil {
		s.logger.Error("commit", zap.Error(err))
		return nil, status.Error(codes.Internal, "internal")
	}

	return &pb.ReleaseStockResponse{Status: "RELEASED"}, nil
}
