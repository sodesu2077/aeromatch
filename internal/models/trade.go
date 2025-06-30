package models

type Trade struct {
	// Hot Path (first cache line - 64 bytes)
	TradeID     uint64
	ExecutionID uint64   
	Price       float64  
	Quantity    float64  
	Timestamp   int64
	MakerOrderID uint64    
	TakerOrderID uint64  
	Fee         float64 // Fee charged for the trade  

	// Warm Path (second cache line - 64 bytes)
	Instrument   string
	Side         OrderSide
	FeeCurrency string
	Tags        map[string]string
}