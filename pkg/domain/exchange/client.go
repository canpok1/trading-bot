package exchange

import (
	"time"
	"trading-bot/pkg/domain/model"
)

// Client 取引所クライアント
type Client interface {
	GetStoreRate(*model.CurrencyPair) (*model.StoreRate, error)
	GetOrderRate(*model.CurrencyPair, model.OrderSide) (*model.OrderRate, error)
	GetBalance(currency model.CurrencyType) (*model.Balance, error)
	GetOpenOrders(*model.CurrencyPair) ([]model.Order, error)
	GetContracts() ([]model.Contract, error)
	PostOrder(*model.NewOrder) (*model.Order, error)
	DeleteOrder(id uint64) error
	GetVolumes(*model.CurrencyPair, model.OrderSide, time.Duration) (float64, error)
}
