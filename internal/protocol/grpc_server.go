package protocol

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	grpcapi "github.com/aeromatch/api/grpc" // Import the generated gRPC code with alias "pb"
	"github.com/aeromatch/internal/engine"
	"github.com/aeromatch/internal/models"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// gRPC server for AeroMatch order submission and market data

type GRPCServer struct {
	engine                             *engine.MatchingEngine
	server                             *grpc.Server
	listener                           net.Listener
	shutdownWg                         sync.WaitGroup // Wait for all goroutines to finish
	grpcapi.UnimplementedTradingServer                // Embed the unimplemented server to satisfy the interface
}

// NewGRPCServer creates a new gRPC server for AeroMatch
func NewGRPCServer(matchingEngine *engine.MatchingEngine, port int, maxMessageSize int) (*GRPCServer, error) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
	}

	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(maxMessageSize),
		grpc.MaxSendMsgSize(maxMessageSize),
	)

	s := &GRPCServer{
		engine:   matchingEngine,
		server:   grpcServer,
		listener: lis,
	}

	grpcapi.RegisterTradingServer(grpcServer, s)
	return s, nil
}

// Start begins serving gRPC requests
func (s *GRPCServer) Start() error {
	s.shutdownWg.Add(1)
	go func() {
		defer s.shutdownWg.Done()
		if err := s.server.Serve(s.listener); err != nil {
			// TODO: handle error
		}
	}()
	return nil
}

func (s *GRPCServer) Stop() {
	s.server.GracefulStop()
	s.shutdownWg.Wait()
}

// SubmitOrder handles order submission via gRPC
func (s *GRPCServer) SubmitOrder(ctx context.Context, req *grpcapi.OrderRequest) (*grpcapi.OrderResponse, error) {
	// Convert gRPC request to internal order model
	order, err := s.convertOrderRequest(req)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid order: %v", err)
	}

	// Validate order
	if err := order.Validate(); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "validation failed: %v", err)
	}

	// Submit to matching engine
	s.engine.SubmitOrder(order)

	return &grpcapi.OrderResponse{
		OrderId:   order.ID,
		Status:    grpcapi.OrderStatus_PENDING,
		Timestamp: order.Timestamp.UnixNano(),
	}, nil
}

// SubmitOrderStream handles streaming order submission
func (s *GRPCServer) SubmitOrderStream(stream grpcapi.Trading_SubmitOrderStreamServer) error {
	for {
		req, err := stream.Recv()
		if err != nil {
			return err
		}

		order, err := s.convertOrderRequest(req)
		if err != nil {
			// Send error response but continue processing stream
			stream.Send(&grpcapi.OrderResponse{
				OrderId: req.OrderId,
				Status:  grpcapi.OrderStatus_REJECTED,
				Error:   err.Error(),
			})
			continue
		}

		s.engine.SubmitOrder(order)

		// Send acknowledgment
		stream.Send(&grpcapi.OrderResponse{
			OrderId:   order.ID,
			Status:    grpcapi.OrderStatus_PENDING,
			Timestamp: order.Timestamp.UnixNano(),
		})
	}
}

// GetOrderBook returns current order book state
func (s *GRPCServer) GetOrderBook(ctx context.Context, req *grpcapi.OrderBookRequest) (*grpcapi.OrderBookResponse, error) {
	// TODO: This would require adding order book snapshot capabilities to the engine
	// For now, return unimplemented
	return nil, status.Errorf(codes.Unimplemented, "order book snapshot not implemented")
}

// convertOrderRequest converts gRPC OrderRequest to internal models.Order
func (s *GRPCServer) convertOrderRequest(req *grpcapi.OrderRequest) (*models.Order, error) {
	orderType, err := s.convertOrderType(req.OrderType)
	if err != nil {
		return nil, err
	}

	orderSide, err := s.convertOrderSide(req.Side)
	if err != nil {
		return nil, err
	}

	return &models.Order{
		ID:         req.OrderId,
		Price:      req.Price,
		Quantity:   req.Quantity,
		Remaining:  req.Quantity, // Initially remaining equals quantity
		Side:       orderSide,
		Type:       orderType,
		Instrument: req.Instrument,
		Timestamp:  time.Now(),
		Status:     models.New,
		ClientOID:  req.ClientOrderId,
	}, nil
}

// convertOrderType converts gRPC OrderType to models.OrderType
func (s *GRPCServer) convertOrderType(t grpcapi.OrderType) (models.OrderType, error) {
	switch t {
	case grpcapi.OrderType_LIMIT:
		return models.Limit, nil
	case grpcapi.OrderType_MARKET:
		return models.Market, nil
	case grpcapi.OrderType_IOC:
		return models.IOC, nil
	case grpcapi.OrderType_FOK:
		return models.FOK, nil
	case grpcapi.OrderType_POST_ONLY:
		return models.PostOnly, nil
	default:
		return 0, status.Errorf(codes.InvalidArgument, "unknown order type: %v", t)
	}
}

// convertOrderSide converts gRPC OrderSide to models.OrderSide
func (s *GRPCServer) convertOrderSide(side grpcapi.OrderSide) (models.OrderSide, error) {
	switch side {
	case grpcapi.OrderSide_BUY:
		return models.Buy, nil
	case grpcapi.OrderSide_SELL:
		return models.Sell, nil
	default:
		return 0, status.Errorf(codes.InvalidArgument, "unknown order side: %v", side)
	}
}

// MarketDataStream streams market data updates
func (s *GRPCServer) MarketDataStream(req *grpcapi.MarketDataRequest, stream grpcapi.Trading_MarketDataStreamServer) error {
	// Subscribe to trade channel from matching engine
	tradeChan := s.engine.GetTradesChannel()

	for {
		select {
		case trade := <-tradeChan:
			if trade.Instrument == req.Instrument {
				err := stream.Send(&grpcapi.MarketDataUpdate{
					Type:      grpcapi.MarketDataType_TRADE,
					Trade:     s.convertTradeToProto(trade),
					Timestamp: trade.Timestamp,
				})
				if err != nil {
					return err
				}
			}
		case <-stream.Context().Done():
			return stream.Context().Err()
		}
	}
}

// convertTradeToProto converts internal trade to gRPC Trade message
func (s *GRPCServer) convertTradeToProto(trade *models.Trade) *grpcapi.Trade {
	return &grpcapi.Trade{
		TradeId:      trade.TradeID,
		ExecutionId:  trade.ExecutionID,
		Price:        trade.Price,
		Quantity:     trade.Quantity,
		Timestamp:    trade.Timestamp,
		MakerOrderId: trade.MakerOrderID,
		TakerOrderId: trade.TakerOrderID,
		Instrument:   trade.Instrument,
		Side:         s.convertOrderSideToProto(trade.Side),
	}
}

// convertOrderSideToProto converts internal OrderSide to gRPC OrderSide
func (s *GRPCServer) convertOrderSideToProto(side models.OrderSide) grpcapi.OrderSide {
	if side == models.Buy {
		return grpcapi.OrderSide_BUY
	}
	return grpcapi.OrderSide_SELL
}
