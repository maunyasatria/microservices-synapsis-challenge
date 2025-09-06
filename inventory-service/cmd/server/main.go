package main

import (
	"context"
	"database/sql"
	"flag"
	"net"

	_ "github.com/lib/pq"
	pb "github.com/synapsis-challenge/gen/inventory/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func main() {
	addr := flag.String("addr", ":50051", "gRPC listen address")
	dsn := flag.String("dsn", "postgres://inv_user:secret@localhost:5432/inventory?sslmode=disable", "Postgres DSN")
	flag.Parse()

	logger, _ := zap.NewProduction()
	defer logger.Sync()

	db, err := sql.Open("postgres", *dsn)
	if err != nil {
		logger.Fatal("failed to open db", zap.Error(err))
	}
	if err := db.PingContext(context.Background()); err != nil {
		logger.Fatal("db ping failed", zap.Error(err))
	}

	lis, err := net.Listen("tcp", *addr)
	if err != nil {
		logger.Fatal("failed listen", zap.Error(err))
	}

	grpcServer := grpc.NewServer()
	srv := NewInventoryServer(db, logger)
	pb.RegisterInventoryServiceServer(grpcServer, srv)

	logger.Info("inventory server starting", zap.String("addr", *addr))
	if err := grpcServer.Serve(lis); err != nil {
		logger.Fatal("grpc serve", zap.Error(err))
	}
}
