package models

import (
	"errors"
	"time"
	"unsafe"
)

type OrderType uint8

const (
	Limit    OrderType = iota // Standard limit order
	Market                    // Market order
	IOC                       // Immediate-or-Cancel
	FOK                       // Fill-or-Kill
	PostOnly                  // Maker-only order
)

type OrderSide uint8

const (
	Buy OrderSide = iota
	Sell
)

type OrderStatus uint8

const (
	New       OrderStatus = iota // Order just received
	Partial                      // Partially filled
	Filled                       // Fully filled
	Cancelled                    // Explicitly cancelled
	Rejected                     // Rejected by system
)

// Order represents a single order in the order book
type Order struct {
	// Hot Path Fields (64 bytes cache-line aligned)
	ID        uint64  // Order ID (unique per instrument)
	Price     float64 // Limit price (NaN for market orders)
	Quantity  float64 // Original quantity
	Remaining float64 // Remaining quantity
	Side      OrderSide
	Type      OrderType
	_         [30]byte // Padding to align to 64 bytes

	// Warm Path Fields (less frequently accessed)
	Instrument  string // Trading pair (e.g., "BTC-USD")
	Account     string
	Timestamp   time.Time // Order creation time
	Status      OrderStatus
	LastUpdated time.Time

	// Cold Path Fields (rarely accessed)
	ClientOID    string
	Tags         map[string]string
	MarginParams *MarginParams // For leveraged trading
}

// MarginParams contains leverage-specific parameters
type MarginParams struct {
	Leverage    float64
	IsIsolated  bool
	Liquidation float64
	BorrowCost  float64
}

// OrderEvent represents changes to an order's state
type OrderEvent struct {
	Order       *Order
	OldStatus   OrderStatus
	ExecutionID uint64
	TradePrice  float64
	TradeSize   float64
	Timestamp   time.Time
}

func SizeOfOrder() uintptr {
	return unsafe.Sizeof(Order{})
}

func (o *Order) IsActive() bool {
	return o.Status == New || o.Status == Partial
}

func (o *Order) Validate() error {
	if o.Quantity <= 0 {
		return ErrInvalidQuantity
	}
	if o.Type == Limit && (o.Price <= 0 || o.Price != o.Price) { // NaN check
		return ErrInvalidPrice
	}
	if len(o.Instrument) == 0 {
		return ErrMissingInstrument
	}
	return nil
}

// Common validation errors
var (
	ErrInvalidQuantity   = errors.New("quantity must be positive")
	ErrInvalidPrice      = errors.New("invalid price for limit order")
	ErrMissingInstrument = errors.New("missing instrument identifier")
)
