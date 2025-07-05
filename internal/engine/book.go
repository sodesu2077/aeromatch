package engine

import (
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/aeromatch/internal/models"
)

const (
	cacheLineSize = 64
	paddedSize    = (cacheLineSize / unsafe.Sizeof(uint64(0))) - 1
)

// Padded uint64 to avoid false sharing
type PaddedUint64 struct {
	value uint64
	_     [paddedSize]uint64
}

// Lock-free order book with bids and asks
type OrderBook struct {
	bidSeq          PaddedUint64
	askSeq          PaddedUint64
	bids            *OrderSide
	asks            *OrderSide
	incomingOrders  chan *models.Order
	processedTrades chan *models.Trade
}

// Side of the order book (bids or asks)
// TODO: Implement balanced binary search tree or a skip list;
type OrderSide struct {
	head    *OrderNode
	tail    *OrderNode
	counter int32
}

// Node in the order book for each order
type OrderNode struct {
	order    *models.Order
	next     unsafe.Pointer
	quantity int64
}

func NewOrderBook(bufferSize int) *OrderBook {
	return &OrderBook{
		bids: &OrderSide{
			head: nil, tail: nil,
		},
		asks: &OrderSide{
			head: nil, tail: nil,
		},
		incomingOrders:  make(chan *models.Order, bufferSize),
		processedTrades: make(chan *models.Trade, bufferSize*2),
	}
}

func (ob *OrderBook) AddOrder(order *models.Order) {
	ob.incomingOrders <- order
}

func (ob *OrderBook) ProcessOrders() {
	for order := range ob.incomingOrders {
		switch order.Side {
		case models.Buy:
			ob.ProcessBuyOrder(order)
		case models.Sell:
			ob.ProcessSellOrder(order)
		}
	}
}

func (ob *OrderBook) ProcessBuyOrder(order *models.Order) {
	remainingQty := order.Quantity

	for remainingQty > 0 {
		bestAsk, ok := ob.GetBestAsk()
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
		trade := ob.createTradeDraft(bestAsk, order, fillPrice, fillQty)
		ob.processedTrades <- trade

		// Update quantities
		remainingQty -= fillQty
		bestAsk.Remaining -= fillQty
		order.Remaining -= fillQty

		// Remove exhausted order
		if bestAsk.Remaining <= 0 {
			ob.removeAsk(bestAsk)
		}

		// Handle order types
		if order.Type == models.IOC && remainingQty > 0 {
			break // Immediate-or-Cancel: cancel remaining
		}

	}

	// Add remaining quantity to book if not fully filled
	if remainingQty > 0 && order.Type != models.IOC && order.Type != models.FOK {
		ob.AddBid(order)
	}
}

func (ob *OrderBook) ProcessSellOrder(order *models.Order) {
	remainingQty := order.Quantity

	for remainingQty > 0 {
		bestBid, ok := ob.GetBestBid()
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
		trade := ob.createTradeDraft(bestBid, order, fillPrice, fillQty)
		ob.processedTrades <- trade

		// Update quantities
		remainingQty -= fillQty
		bestBid.Remaining -= fillQty
		order.Remaining -= fillQty

		// Remove exhausted order
		if bestBid.Remaining <= 0 {
			ob.removeBid(bestBid)
		}

		// Handle order types
		if order.Type == models.IOC && remainingQty > 0 {
			break // Immediate-or-Cancel: cancel remaining
		}
	}

	// Add remaining quantity to book if not fully filled
	if remainingQty > 0 && order.Type != models.IOC && order.Type != models.FOK {
		ob.AddAsk(order)
	}
}

func (ob *OrderBook) createTradeDraft(maker, taker *models.Order, price, qty float64) *models.Trade {
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

func (ob *OrderBook) AddBid(order *models.Order) {
	// Create a new order node
	newNode := &OrderNode{
		order:    order,
		next:     nil,
		quantity: int64(order.Quantity),
	}

	for {
		// Load the current tail of the bids list
		tail := atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&ob.bids.tail)))

		if atomic.CompareAndSwapPointer((*unsafe.Pointer)(unsafe.Pointer(&ob.bids.tail)), tail, unsafe.Pointer(newNode)) {
			if tail != nil {
				// Link the new node to the previous tail
				atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&(*OrderNode)(tail).next)), unsafe.Pointer(newNode))
			} else {
				// If the list was empty, set head to the new node
				atomic.CompareAndSwapPointer((*unsafe.Pointer)(unsafe.Pointer(&ob.bids.head)), nil, unsafe.Pointer(newNode))
			}
			return
		}
	}
}

func (ob *OrderBook) AddAsk(order *models.Order) {
	// Create a new order node
	newNode := &OrderNode{
		order:    order,
		next:     nil,
		quantity: int64(order.Quantity),
	}

	for {
		// Load the current tail of the asks list
		tail := atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&ob.asks.tail)))

		if atomic.CompareAndSwapPointer((*unsafe.Pointer)(unsafe.Pointer(&ob.asks.tail)), tail, unsafe.Pointer(newNode)) {
			if tail != nil {
				// Link the new node to the previous tail.
				// This is safe because only one goroutine can succeed in the CAS above.
				// So when we get here, 'tail' is guaranteed to be the previous tail.
				// However, there will be a momentary inconsistency where the new node is not yet linked to the previous tail.
				// This is acceptable in a lock-free design as other readers will eventually see the updated list.
				// Readers must always traverse from head to tail to see the complete list.
				// This ensures that even if they see an intermediate state, they will eventually see the fully linked list.
				atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&(*OrderNode)(tail).next)), unsafe.Pointer(newNode))
			} else {
				// If the list was empty, set head to the new node
				atomic.CompareAndSwapPointer((*unsafe.Pointer)(unsafe.Pointer(&ob.asks.head)), nil, unsafe.Pointer(newNode))
			}
			return
		}
	}
}

func (ob *OrderBook) removeBid(order *models.Order) {

}

func (ob *OrderBook) removeAsk(order *models.Order) {

}

func (ob *OrderBook) GetBestBid() (*models.Order, bool) {
	head := atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&ob.bids.head)))
	if head == nil {
		return nil, false
	}
	order := (*OrderNode)(head)
	return order.order, true
}

func (ob *OrderBook) GetBestAsk() (*models.Order, bool) {
	head := atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&ob.asks.head)))
	if head == nil {
		return nil, false
	}
	order := (*OrderNode)(head)
	return order.order, true
}

func (ob *OrderBook) GetMarketDepth(level int32) *OrderBookSnapshot {
	snapshot := &OrderBookSnapshot{
		Bids: make([]PriceLevel, 0, level),
		Asks: make([]PriceLevel, 0, level),
	}

	// TODO: implement the logic to populate snapshot.Bids and snapshot.Asks

	return snapshot
}
func (os *OrderSide) GetDepth(price float64) int32 {
	var count int32
	current := (*OrderNode)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&os.head))))

	for current != nil {
		if current.order.Price == price {
			count++
		}
		current = (*OrderNode)(atomic.LoadPointer(&current.next))
	}

	return count
}

func (os *OrderSide) GetTotalVolume(price float64) int64 {
	var volume int64
	current := (*OrderNode)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&os.head))))

	for current != nil {
		if current.order.Price == price {
			volume += atomic.LoadInt64(&current.quantity)
		}
		current = (*OrderNode)(atomic.LoadPointer(&current.next))
	}

	return volume
}
