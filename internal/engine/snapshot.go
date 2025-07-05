package engine

import (
	"encoding/json"
	"maps"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/aeromatch/internal/models"
)

// SnapshotManager handles order book snapshots
type SnapshotManager struct {
	orderBooks unsafe.Pointer // *map[string]*OrderBook (atomic)
	snapshots  unsafe.Pointer // *map[string]*OrderBookSnapshot (atomic)
	interval   time.Duration
	shutdown   chan struct{}
	sequenceID uint64 // sequence ID for snapshots (atomic)
}

// OrderBookSnapshot represents a point-in-time view of the order book
type OrderBookSnapshot struct {
	Instrument string        `json:"instrument"`
	Sequence   uint64        `json:"sequence"`
	Timestamp  int64         `json:"timestamp"`
	Bids       []PriceLevel  `json:"bids"`
	Asks       []PriceLevel  `json:"asks"`
	Stats      SnapshotStats `json:"stats"`
}

// PriceLevel represents a price level in the order book
type PriceLevel struct {
	Price    float64 `json:"price"`
	Quantity float64 `json:"quantity"`
	Orders   int     `json:"order_count"`
}

// SnapshotStats contains order book statistics
type SnapshotStats struct {
	TotalBidQuantity float64 `json:"total_bid_quantity"`
	TotalAskQuantity float64 `json:"total_ask_quantity"`
	BidOrders        int     `json:"bid_orders"`
	AskOrders        int     `json:"ask_orders"`
	Spread           float64 `json:"spread"`    // Difference between the highest bid and lowest ask
	MidPrice         float64 `json:"mid_price"` // Average of the highest bid and lowest ask
}

// SnapshotStorage defines the interface for snapshot persistence
type SnapshotStorage interface {
	SaveSnapshot(snapshot *OrderBookSnapshot) error
	LoadSnapshot(instrument string) (*OrderBookSnapshot, error)
}

// NewSnapshotManager creates a new snapshot manager
func NewSnapshotManager(interval time.Duration) *SnapshotManager {
	initialBooks := make(map[string]*OrderBook)
	initialSnapshots := make(map[string]*OrderBookSnapshot)

	return &SnapshotManager{
		orderBooks: unsafe.Pointer(&initialBooks),
		snapshots:  unsafe.Pointer(&initialSnapshots),
		interval:   interval,
		shutdown:   make(chan struct{}),
	}
}

// RegisterOrderBook adds an order book to snapshot management
func (sm *SnapshotManager) RegisterOrderBook(instrument string, book *OrderBook) {
	for {
		oldPtr := atomic.LoadPointer(&sm.orderBooks)
		oldBooks := *(*map[string]*OrderBook)(oldPtr)

		// Create copy-on-write
		newBooks := make(map[string]*OrderBook, len(oldBooks)+1)
		maps.Copy(newBooks, oldBooks)
		newBooks[instrument] = book

		// CAS to update
		if atomic.CompareAndSwapPointer(&sm.orderBooks, oldPtr, unsafe.Pointer(&newBooks)) {
			break
		}
	}
}

// Start begins periodic snapshotting
func (sm *SnapshotManager) Start() {
	go sm.snapshotLoop()
}

// Stop gracefully shuts down the snapshot manager
func (sm *SnapshotManager) Stop() {
	close(sm.shutdown)
}

// snapshotLoop runs the periodic snapshotting
func (sm *SnapshotManager) snapshotLoop() {
	ticker := time.NewTicker(sm.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sm.TakeSnapshots()
		case <-sm.shutdown:
			return
		}
	}
}

// TakeSnapshots creates snapshots for all registered order books
func (sm *SnapshotManager) TakeSnapshots() {
	booksPtr := atomic.LoadPointer(&sm.orderBooks)
	books := *(*map[string]OrderBook)(booksPtr)

	newSnapshots := make(map[string]*OrderBookSnapshot, len(books))

	for instrument, book := range books {
		snapshot := sm.takeSnapshot(instrument, book)
		newSnapshots[instrument] = snapshot
	}

	atomic.StorePointer(&sm.snapshots, unsafe.Pointer(&newSnapshots))
}

// takeSnapshot creates a snapshot for a single order book
func (sm *SnapshotManager) takeSnapshot(instrument string, book OrderBook) *OrderBookSnapshot {
	depth := book.GetMarketDepth(100) // Top 100 levels

	stats := sm.calculateStats(depth)

	return &OrderBookSnapshot{
		Instrument: instrument,
		Sequence:   atomic.AddUint64(&sm.sequenceID, 1),
		Timestamp:  time.Now().UnixNano(),
		Bids:       depth.Bids,
		Asks:       depth.Asks,
		Stats:      stats,
	}
}

// calculateStats calculates order book statistics
func (sm *SnapshotManager) calculateStats(depth *OrderBookSnapshot) SnapshotStats {
	var stats SnapshotStats

	for _, bid := range depth.Bids {
		stats.TotalBidQuantity += bid.Quantity
		stats.BidOrders += bid.Orders
	}

	for _, ask := range depth.Asks {
		stats.TotalAskQuantity += ask.Quantity
		stats.AskOrders += ask.Orders
	}

	if len(depth.Bids) > 0 && len(depth.Asks) > 0 {
		bestBid := depth.Bids[0].Price
		bestAsk := depth.Asks[0].Price
		stats.Spread = bestAsk - bestBid
		stats.MidPrice = (bestBid + bestAsk) / 2
	}

	return stats
}

// GetSnapshot returns the latest snapshot for an instrument
func (sm *SnapshotManager) GetSnapshot(instrument string) (*OrderBookSnapshot, bool) {
	snapshotsPtr := atomic.LoadPointer(&sm.snapshots)
	snapshots := *(*map[string]*OrderBookSnapshot)(snapshotsPtr)

	snapshot, exists := snapshots[instrument]
	if !exists {
		return nil, false
	}

	// Return a copy to avoid data races
	return &OrderBookSnapshot{
		Instrument: snapshot.Instrument,
		Sequence:   snapshot.Sequence,
		Timestamp:  snapshot.Timestamp,
		Bids:       copyPriceLevels(snapshot.Bids),
		Asks:       copyPriceLevels(snapshot.Asks),
		Stats:      snapshot.Stats,
	}, exists
}

// GetAllSnapshots returns snapshots for all instruments
func (sm *SnapshotManager) GetAllSnapshots() map[string]*OrderBookSnapshot {
	snapshotsPtr := atomic.LoadPointer(&sm.snapshots)
	snapshots := *(*map[string]*OrderBookSnapshot)(snapshotsPtr)

	// Create a deep copy to avoid data races
	result := make(map[string]*OrderBookSnapshot, len(snapshots))
	for instrument, snapshot := range snapshots {
		if snapshot != nil {
			result[instrument] = &OrderBookSnapshot{
				Instrument: snapshot.Instrument,
				Sequence:   snapshot.Sequence,
				Timestamp:  snapshot.Timestamp,
				Bids:       copyPriceLevels(snapshot.Bids),
				Asks:       copyPriceLevels(snapshot.Asks),
				Stats:      snapshot.Stats,
			}
		}
	}
	return result
}

// copyPriceLevels creates a deep copy of price levels
func copyPriceLevels(levels []PriceLevel) []PriceLevel {
	if levels == nil {
		return nil
	}
	copy := make([]PriceLevel, len(levels))
	for i, level := range levels {
		copy[i] = level
	}
	return copy
}

// MarshalBinary serializes snapshot to binary format
func (s *OrderBookSnapshot) MarshalBinary() ([]byte, error) {
	return json.Marshal(s)
}

// UnmarshalBinary deserializes snapshot from binary format
func (s *OrderBookSnapshot) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, s)
}

// GetPriceLevels returns price levels for a specific side
func (s *OrderBookSnapshot) GetPriceLevels(side models.OrderSide) []PriceLevel {
	if side == models.Buy {
		return s.Bids
	}
	return s.Asks
}

// GetBestPrice returns the best bid or ask price
func (s *OrderBookSnapshot) GetBestPrice(side models.OrderSide) (float64, bool) {
	levels := s.GetPriceLevels(side)
	if len(levels) == 0 {
		return 0, false
	}
	return levels[0].Price, true
}
