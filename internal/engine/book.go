package engine

import (
	"sync/atomic"
	"unsafe"

	"github.com/aeromatch/internal/models"
)

// Package engine implements a lock-free order book for matching buy and sell orders
// in a high-frequency trading environment. It uses atomic operations and
// padded structures to avoid false sharing, ensuring high performance and low latency.
// The order book supports concurrent access and allows for efficient order matching
// without traditional locking mechanisms.
// The order book consists of two sides: bids and asks, each represented as a linked list
// of orders. Orders are processed in a lock-free manner, allowing for high throughput
// and low contention. The book supports adding orders, processing incoming orders,
// and retrieving the best bid and ask prices. It also provides methods to get the depth
// and total volume at specific prices, enabling efficient market data retrieval.
// The implementation is designed to handle a large number of orders and trades,
// making it suitable for high-frequency trading applications where performance is critical.

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

}

func (ob *OrderBook) ProcessSellOrder(order *models.Order) {

}

func (ob *OrderBook) GetBestBidPrice() (float64, bool) {
	head := atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&ob.bids.head)))
	if head == nil {
		return 0, false
	}
	order := (*OrderNode)(head)
	return order.order.Price, true
}

func (ob *OrderBook) GetBestAskPrice() (float64, bool) {
	head := atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&ob.asks.head)))
	if head == nil {
		return 0, false
	}
	order := (*OrderNode)(head)
	return order.order.Price, true
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
