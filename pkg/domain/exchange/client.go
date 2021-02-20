package exchange

import (
	"trading-bot/pkg/domain/model"
)

type Client interface {
	GetOrderRate(model.OrderType, model.CurrencyPair) *ResponseGetOrderRate
}

type ResponseGetOrderRate struct {
	Rate   int
	Price  int
	Amount int
}
