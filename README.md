## Overview

AeroMatch maintains an order book and matches buy/sell orders using a lock-free design to minimize latency and maximize throughput. It's designed for educational purposes to explore:

- Lock-free data structures and atomic operations
- Low-latency order matching algorithms  
- Concurrent order processing patterns
- Memory layout optimization for CPU cache efficiency

## Current Status

**Early Development** - This is an ongoing project with basic functionality implemented.

## Contributing

Contributions are welcome and appreciated. If you're interested in improving AeroMatch or adding new features, feel free to submit a pull request or open an issue.

## Technical Approach

- **Lock-free design** using atomic operations instead of mutexes
- **CPU cache optimization** through memory layout and padding
- **Channel-based communication** between components
- **Minimal GC pressure** through object reuse and pre-allocation

## Project layout
(not strictly set, but the general outline)


```
aeromatch/
│
├── cmd/                     # Executable entrypoints
│   └── aeromatchd/          # Main server binary
│       └── main.go
│
├── internal/                # Private application code
│   ├── engine/               # Core order matching logic
│   │   ├── book.go            # Order book data structure
│   │   ├── matching.go        # Matching algorithm
│   │   ├── atomic_queue.go    # Lock-free queue for incoming orders
│   │   └── snapshot.go        # Book snapshot & serialization
│   │
│   ├── models/               # Domain models
│   │   ├── order.go           # Order struct, order types
│   │   └── trade.go           # Trade struct
│   │
│   ├── protocol/              # Network layer (gRPC/WebSocket)
│   │   ├── grpc_server.go     # gRPC order submission API
│   │   ├── ws_server.go       # WebSocket updates for trades
│   │   └── codec.go           # Binary message encoding/decoding
│   │
│   ├── store/                 # Persistence layer
│   │   ├── memory_store.go    # In-memory storage for speed
│   │   ├── postgres_store.go  # Optional persistent store
│   │   └── snapshot_store.go  # Snapshot/restore engine state
│   │
│   ├── config/                # Config management
│   │   └── config.go
│   │
│   ├── metrics/               # Observability
│   │   ├── prometheus.go      # Prometheus metrics export
│   │   └── pprof.go           # Performance profiling endpoints
│   │
│   └── util/                  # Utility helpers
│       ├── logger.go
│       ├── errors.go
│       └── id_generator.go
│
├── pkg/                      # Public reusable packages
│   └── aeromatch/
│       ├── client.go
│       └── types.go
│
├── api/                      # API definitions
│   ├── grpc/                  # gRPC proto files
│   │   └── order.proto
│   └── openapi/               # OpenAPI spec for HTTP API
│       └── spec.yaml
│
├── scripts/                  # DevOps scripts
│   ├── run_local.sh
│   ├── benchmark.sh
│   └── load_test.sh
│
├── test/                     # Integration & benchmark tests
│   ├── engine_bench_test.go
│   ├── engine_integration_test.go
│   └── ws_load_test.go
│
├── go.mod
├── go.sum
└── README.md
```
