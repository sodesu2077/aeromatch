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
