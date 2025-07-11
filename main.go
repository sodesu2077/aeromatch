package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/aeromatch/internal/config"
	"github.com/aeromatch/internal/engine"
	"github.com/aeromatch/internal/protocol"
	"github.com/aeromatch/internal/util"
)

func main() {
	// Initialize
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logging
	output := os.Stdout
	util.Init(util.LevelInfo, cfg.Logging.Format, output)

	// ----------CORE ENGINE SETUP----------
	// Create matching engine
	matchingEngine := engine.NewMatchingEngine(cfg.Engine.BufferSize)

	// Create order books for supported instruments
	instruments := []string{"BTC-USD", "ETH-USD", "AAPL", "GOOGL"}
	for _, instrument := range instruments {
		orderBook := engine.NewOrderBook(cfg.Engine.OrderBookBufferSize)
		matchingEngine.RegisterOrderBook(instrument, orderBook)
	}

	// ----------STORAGE & PERSISTENCE----------
	// TODO: Initialize persistent storage

	// NETWORK LAYER
	// Initialize gRPC server
	grpcServer, err := protocol.NewGRPCServer(
		matchingEngine,
		cfg.Server.GRPCPort,
		cfg.Server.MaxMessageSize,
	)
	if err != nil {
		log.Fatalf("Failed to create gRPC server: %v", err)
	}

	// TODO: Initialize WebSocket server

	// ----------MONITORING & OBSERVABILITY----------
	// TODO: Initialize metrics

	// ----------STARTUP SEQUENCE----------
	// TODO: Start metrics server

	// Start matching engine
	matchingEngine.Start()
	log.Println("Matching engine started")

	// Start network servers
	go grpcServer.Start()
	log.Println("gRPC server started", "port", cfg.Server.GRPCPort)

	// TODO: Load initial state if available

	// ----------HEALTH CHECK & READINESS----------
	// Perform health check
	if err := performHealthCheck(matchingEngine, grpcServer); err != nil {
		log.Fatalf("Health check failed: %v", err)
	}
	log.Println("AeroMatch is ready and accepting orders")

	// ----------GRACEFUL SHUTDOWN HANDLING----------
	// Wait for termination signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("Shutdown signal received, initiating graceful shutdown")
	// TODO: Implement graceful shutdown logic

}

// Helper functions
func performHealthCheck(engine *engine.MatchingEngine, grpcServer *protocol.GRPCServer) error {
	return nil
}
