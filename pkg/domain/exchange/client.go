package exchange

import (
	"trading-bot/pkg/domain/model"
)

type Client interface {
	GetOrderRate(model.OrderType, model.CurrencyPair) (*model.OrderRate, error)
	GetAccountBalance() (*model.Balance, error)
	GetOrderTransactions() ([]model.OrderTransaction, error)
	PostOrder(*model.NewOrder) (*model.Order, error)
	DeleteOrder(id uint64) error
}
