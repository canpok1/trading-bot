package exchange

import (
	"trading-bot/pkg/domain/model"
)

type Client interface {
	GetOrderRate(model.OrderType, model.CurrencyPair) (*model.OrderRate, error)
	GetAccountBalance() (*model.Balance, error)
}
