package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	invpb "github.com/synapsis-challenge/gen/inventory/v1"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type OrderHandler struct {
	db        *sql.DB
	invClient invpb.InventoryServiceClient
	logger    *zap.Logger
}

func NewOrderHandler(db *sql.DB, invClient invpb.InventoryServiceClient, logger *zap.Logger) *OrderHandler {
	return &OrderHandler{db: db, invClient: invClient, logger: logger}
}

type createOrderReq struct {
	UserID string `json:"user_id"`
	Items  []struct {
		SKU string `json:"sku"`
		Qty int    `json:"qty"`
	} `json:"items"`
	IdempotencyKey string `json:"idempotency_key"`
}

type createOrderResp struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
}

func (h *OrderHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req createOrderReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	var existing string
	err := h.db.QueryRowContext(ctx, `SELECT id FROM orders WHERE idempotency_key=$1`, req.IdempotencyKey).Scan(&existing)
	if err == nil {
		var status string
		_ = h.db.QueryRowContext(ctx, `SELECT status FROM orders WHERE id=$1`, existing).Scan(&status)
		json.NewEncoder(w).Encode(createOrderResp{OrderID: existing, Status: status})
		return
	} else if err != sql.ErrNoRows {
		h.logger.Error("idempotency lookup", zap.Error(err))
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}

	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		h.logger.Error("begin tx", zap.Error(err))
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	var orderID string
	err = tx.QueryRowContext(ctx, `INSERT INTO orders(user_id, status, idempotency_key, created_at) VALUES ($1,$2,$3,$4) RETURNING id`, req.UserID, "PENDING", req.IdempotencyKey, time.Now()).Scan(&orderID)
	if err != nil {
		tx.Rollback()
		h.logger.Error("insert order", zap.Error(err))
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}

	for _, it := range req.Items {
		_, err := tx.ExecContext(ctx, `INSERT INTO order_items(order_id, sku, qty) VALUES ($1,$2,$3)`, orderID, it.SKU, it.Qty)
		if err != nil {
			tx.Rollback()
			h.logger.Error("insert order item", zap.Error(err))
			http.Error(w, "internal", http.StatusInternalServerError)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		h.logger.Error("commit", zap.Error(err))
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}

	reserved := []string{}
	for _, it := range req.Items {
		attempt := 0
		var res *invpb.ReserveStockResponse
		for {
			attempt++
			ctxRPC, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			rpcReq := &invpb.ReserveStockRequest{Sku: it.SKU, Qty: int32(it.Qty), CorrelationId: orderID + "-" + it.SKU}
			res, err = h.invClient.ReserveStock(ctxRPC, rpcReq)
			if err == nil {
				break
			}
			st, ok := status.FromError(err)
			if !ok || (st.Code() != codes.Unavailable && st.Code() != codes.DeadlineExceeded) {
				break
			}
			if attempt >= 3 {
				break
			}
			time.Sleep(time.Duration(attempt) * 200 * time.Millisecond)
		}
		if err != nil {
			for _, rid := range reserved {
				ctxRel, cancel := context.WithTimeout(ctx, 2*time.Second)
				_, _ = h.invClient.ReleaseStock(ctxRel, &invpb.ReleaseStockRequest{ReservationId: rid})
				cancel()
			}
			_, _ = h.db.ExecContext(ctx, `UPDATE orders SET status=$1 WHERE id=$2`, "REJECTED", orderID)
			h.logger.Error("reserve failed", zap.Error(err))
			http.Error(w, "order rejected", http.StatusConflict)
			return
		}
		reserved = append(reserved, res.ReservationId)
		_, _ = h.db.ExecContext(ctx, `INSERT INTO inventory_reservations(order_id, reservation_id, sku, qty, created_at) VALUES ($1,$2,$3,$4,$5)`, orderID, res.ReservationId, it.SKU, it.Qty, time.Now())
	}

	_, _ = h.db.ExecContext(ctx, `UPDATE orders SET status=$1 WHERE id=$2`, "CONFIRMED", orderID)
	json.NewEncoder(w).Encode(createOrderResp{OrderID: orderID, Status: "CONFIRMED"})
}
