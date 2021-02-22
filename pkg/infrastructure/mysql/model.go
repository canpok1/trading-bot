package mysql

import "trading-bot/pkg/domain/model"

type Order struct {
	ID           uint64
	OrderType    string
	Pair         string
	Amount       float32
	Rate         *float32
	StopLossRate *float32
	Status       int
}

func NewOrder(org *model.Order) *Order {
	return &Order{
		ID:           org.ID,
		OrderType:    string(org.Type),
		Pair:         org.Pair.String(),
		Amount:       org.Amount,
		Rate:         org.Rate,
		StopLossRate: org.StopLossRate,
		Status:       int(model.Open),
	}
}

type Contract struct {
	ID               uint64
	OrderID          uint64
	Side             int
	IncreaseCurrency string
	IncreaseAmount   float32
	DecreaseCurrency string
	DecreaseAmount   float32
	FeeCurrency      string
	FeeAmount        float32
	Liquidity        int
}

func NewContract(org *model.Contract) *Contract {
	return &Contract{
		ID:               org.ID,
		OrderID:          org.OrderID,
		Side:             int(org.Side),
		IncreaseCurrency: string(org.IncreaseCurrency),
		IncreaseAmount:   org.IncreaseAmount,
		DecreaseCurrency: string(org.DecreaseCurrency),
		DecreaseAmount:   org.DecreaseAmount,
		FeeCurrency:      string(org.FeeCurrency),
		FeeAmount:        org.Fee,
		Liquidity:        int(org.Liquidity),
	}
}
