package engine

import (
	"sync/atomic"
	"time"

	"github.com/aeromatch/internal/models"
)

type MatchingEngine struct {
	orderBooks map[string]*OrderBook // Instrument -> OrderBook
	incoming   chan *models.Order    // Buffered channel for order ingestion
	trades     chan *models.Trade    // Output channel for matched trades
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
}

func (m *MatchingEngine) SubmitOrder(order *models.Order) {
	m.incoming <- order
}

func (m *MatchingEngine) GetTradesChannel() <-chan *models.Trade {
	return m.trades
}

func (m *MatchingEngine) processOrders() {
	for {
		select {
		case order := <-m.incoming:
			go m.matchOrder(order)
		case <-m.shutdown:
			return
		}
	}
}

func (m *MatchingEngine) matchOrder(order *models.Order) {
	book := m.getOrderBook(order.Instrument)

	switch order.Side {
	case models.Buy:
		m.matchBuyOrder(book, order)
	case models.Sell:
		m.matchSellOrder(book, order)
	}
}

func (m *MatchingEngine) matchBuyOrder(book *OrderBook, order *models.Order) {
	remainingQty := order.Quantity

	for remainingQty > 0 {
		bestAsk, ok := book.GetBestAsk()
		if !ok {
			break // No more asks to match
		}

		if order.Type != models.Market && order.Price < bestAsk.Price {
			break // Price doesn't cross
		}

		// Calculate fill quantity
		fillQty := min(remainingQty, bestAsk.Remaining)
		fillPrice := bestAsk.Price

		// Execute trade
		trade := m.createTrade(bestAsk, order, fillPrice, fillQty)
		m.trades <- trade

		// Update quantities
		remainingQty -= fillQty
		bestAsk.Remaining -= fillQty
		order.Remaining -= fillQty
		

		// Remove exhausted order
		if bestAsk.Remaining <= 0 {
			book.removeAsk(bestAsk)
		}

		// Handle order types
		if order.Type == models.IOC && remainingQty > 0 {
			break // Immediate-or-Cancel: cancel remaining
		}

	}

	// Add remaining quantity to book if not fully filled
	if remainingQty > 0 && order.Type != models.IOC && order.Type != models.FOK {
		book.AddBid(order)
	}
}

func (m *MatchingEngine) matchSellOrder(book *OrderBook, order *models.Order) {
	remainingQty := order.Quantity

	for remainingQty > 0 {
		bestBid, ok := book.GetBestBid()
		if !ok {
			break // No more bids to match
		}

		if order.Type != models.Market && order.Price > bestBid.Price {
			break // Price doesn't cross
		}

		// Calculate fill quantity
		fillQty := min(remainingQty, bestBid.Remaining)
		fillPrice := bestBid.Price // Price-time priority

		// Execute trade
		trade := m.createTrade(bestBid, order, fillPrice, fillQty)
		m.trades <- trade

		// Update quantities
		remainingQty -= fillQty
		bestBid.Remaining -= fillQty
		order.Remaining -= fillQty

		// Remove exhausted order
		if bestBid.Remaining <= 0 {
			book.removeBid(bestBid)
		}

		// Handle order types
		if order.Type == models.IOC && remainingQty > 0 {
			break // Immediate-or-Cancel: cancel remaining
		}
	}

	// Add remaining quantity to book if not fully filled
	if remainingQty > 0 && order.Type != models.IOC && order.Type != models.FOK {
		book.AddAsk(order)
	}
}

func (m *MatchingEngine) createTrade(maker, taker *models.Order, price, qty float64) *models.Trade {
	return &models.Trade{
		TradeID:      generateTradeID(),
		ExecutionID:  atomic.AddUint64(&executionCounter, 1),
		Price:        price,
		Quantity:     qty,
		Timestamp:    time.Now().UnixNano(),
		MakerOrderID: maker.ID,
		TakerOrderID: taker.ID,
		Instrument:   maker.Instrument,
		Side:         taker.Side,
	}
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
