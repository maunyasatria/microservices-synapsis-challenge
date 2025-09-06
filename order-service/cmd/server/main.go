package main

import (
	"database/sql"
	"flag"
	"net/http"

	invpb "github.com/synapsis-challenge/gen/inventory/v1"

	"github.com/go-chi/chi/v5"
	_ "github.com/lib/pq"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func main() {
	addr := flag.String("addr", ":8080", "http listen")
	dsn := flag.String("dsn", "postgres://order_user:secret@localhost:5432/orders?sslmode=disable", "Postgres DSN")
	invAddr := flag.String("inv", "localhost:50051", "inventory gRPC addr")
	flag.Parse()

	logger, _ := zap.NewProduction()
	defer logger.Sync()

	db, err := sql.Open("postgres", *dsn)
	if err != nil {
		logger.Fatal("open db", zap.Error(err))
	}

	conn, err := grpc.Dial(*invAddr, grpc.WithInsecure())
	if err != nil {
		logger.Fatal("grpc dial", zap.Error(err))
	}
	invClient := invpb.NewInventoryServiceClient(conn)

	r := chi.NewRouter()
	h := NewOrderHandler(db, invClient, logger)
	r.Post("/orders", h.CreateOrder)

	logger.Info("order service starting", zap.String("addr", *addr))
	if err := http.ListenAndServe(*addr, r); err != nil {
		logger.Fatal("http serve", zap.Error(err))
	}
}
