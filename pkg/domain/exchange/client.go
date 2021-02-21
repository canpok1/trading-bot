package exchange

import (
	"trading-bot/pkg/domain/model"
)

// Client 取引所クライアント
type Client interface {
	GetOrderRate(model.OrderType, model.CurrencyPair) (*model.OrderRate, error)
	GetAccountBalance() (*model.Balance, error)
	GetOpenOrders() ([]model.Order, error)
	GetContracts() ([]model.Contract, error)
	PostOrder(*model.NewOrder) (*model.Order, error)
	DeleteOrder(id uint64) error
}
