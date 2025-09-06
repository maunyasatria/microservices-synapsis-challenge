package main

import (
	"context"
	"database/sql/driver"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	invpb "github.com/synapsis-challenge/gen/inventory/v1"
)

// mock Inventory client (simple hand-written small mock)
type MockInvClient struct {
	mock.Mock
}

func (m *MockInvClient) ReserveStock(ctx context.Context, in *invpb.ReserveStockRequest, opts ...interface{}) (*invpb.ReserveStockResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*invpb.ReserveStockResponse), args.Error(1)
}
