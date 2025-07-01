package engine

import (
	"sync/atomic"

	"github.com/aeromatch/internal/models"
)

type MatchingEngine struct {
	orderBooks map[string]*OrderBook // Instrument -> OrderBook
	incoming   chan *models.Order    // Buffered channel for order ingestion
	trades     chan *models.Trade    // Buffered channel for matched trades
	shutdown   chan struct{}
}

func NewMatchingEngine(bufferSize int) *MatchingEngine {
	return &MatchingEngine{
		orderBooks: make(map[string]*OrderBook),
		incoming:   make(chan *models.Order, bufferSize),
		trades:     make(chan *models.Trade, bufferSize*2),
		shutdown:   make(chan struct{}),
	}
}

func (m *MatchingEngine) Start() {
	go m.processOrders()
	go m.processTrades()
}

func (m *MatchingEngine) SubmitOrder(order *models.Order) {
	m.incoming <- order
}

func (m *MatchingEngine) GetTradesChannel() <-chan *models.Trade {
	return m.trades
}

func (m *MatchingEngine) processOrders() {
	// TODO: validate orders, check risk, etc.
	for {
		select {
		case order := <-m.incoming:
			go m.matchOrder(order)
		case <-m.shutdown:
			return
		}
	}
}

func (m *MatchingEngine) processTrades() {
	for _, book := range m.orderBooks {
		go func(o *OrderBook) {
			for trade := range book.processedTrades { // blocks until a trade is available
				go m.broadCastTrade(trade)
			}
		}(book)
	}

}

func (m *MatchingEngine) broadCastTrade(trade *models.Trade) {
	// TODO: Persist trade to database, notify external systems, etc.

}

func (m *MatchingEngine) matchOrder(order *models.Order) {
	book := m.getOrderBook(order.Instrument)
	book.incomingOrders <- order
}

func (m *MatchingEngine) getOrderBook(instrument string) *OrderBook {
	// Double-checked locking for lazy initialization
	book, exists := m.orderBooks[instrument]
	if !exists {
		book = NewOrderBook(1024) // Initial capacity; TODO: make configurable
		m.orderBooks[instrument] = book
	}
	return book
}

// Helper functions
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// Atomic counters
// TODO: Retrieve from a persistent store
var executionCounter uint64
var tradeIDCounter uint64

func generateTradeID() uint64 {
	return atomic.AddUint64(&tradeIDCounter, 1)
}
