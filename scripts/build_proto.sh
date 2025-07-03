#!/bin/bash

set -e

echo "Generating protobuf files..."

protoc \
    --go_out=. \
    --go-grpc_out=. \
    --go_opt=paths=source_relative \
    --go-grpc_opt=paths=source_relative \
    api/grpc/order.proto

echo "Generated:"
echo "  - api/grpc/order.pb.go"
echo "  - api/grpc/order_grpc.pb.go"