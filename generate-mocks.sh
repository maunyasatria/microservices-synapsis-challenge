#!/usr/bin/env bash
set -e
echo "Install mockery v2 if you don't have it:"
echo "go install github.com/vektra/mockery/v2@latest"
echo
echo "Generate mock for InventoryServiceClient interface synapsis-challenge:"
echo "mockery --srcpkg github.com/synapsis-challenge/gen/inventory/v1 --name InventoryServiceClient --output ./order-service/mocks --outpkg mocks"
echo
echo "Alternatively, if you have an interface in your code named InventoryClient:"
echo "mockery --name InventoryClient --dir ./order-service/internal --output ./order-service/mocks --outpkg mocks"
