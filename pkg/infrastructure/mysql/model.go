package mysql

import "trading-bot/pkg/domain/model"

type Order struct {
	ID           uint64
	OrderType    string
	Pair         string
	Amount       float32
	Rate         float32
	StopLossRate float32
	Status       int
}

func NewOrder(org *model.Order) *Order {
	return &Order{
		ID:           org.ID,
		OrderType:    string(org.Type),
		Pair:         org.Pair.String(),
		Amount:       org.Amount,
		Rate:         *org.Rate,
		StopLossRate: *org.StopLossRate,
		Status:       0,
	}
}

type Position struct {
	ID            uint64
	OpenerOrderID uint64
	CloserOrderID *uint64
	Status        int
}
