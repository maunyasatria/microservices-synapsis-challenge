# Microservices Go Skeleton

This archive contains two minimal services:
- inventory-service (gRPC) on :50051
- order-service (HTTP REST) on :8080

Quickstart:

1. Generate protos (requires protoc + go plugins):
   protoc --go_out=paths=source_relative:gen --go-grpc_out=paths=source_relative:gen proto/*.proto

2. Build and run with docker-compose:
   docker-compose up --build

3. Run migrations: connect to the databases and apply SQL in each migrations folder.

4. Example order:
   curl -X POST http://localhost:8080/orders -d '{"user_id":"00000000-0000-0000-0000-000000000000","items":[{"sku":"sku-1","qty":1}],"idempotency_key":"key-1"}' -H 'Content-Type: application/json'

Unit tests & mocks:

- Use mockery v2 to generate mocks for the InventoryServiceClient interface (order-service).
  Example:
    mockery --name InventoryServiceClient --dir $(go env GOPATH)/pkg/mod --output ./order-service/mocks --outpkg mocks

- Use sqlmock for DB-layer unit tests.

