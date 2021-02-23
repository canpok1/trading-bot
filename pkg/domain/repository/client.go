package repository

import "trading-bot/pkg/domain/model"

// OrderRepository 注文用リポジトリ
type OrderRepository interface {
	AddOrder(*model.Order) error
	GetOpenOrders() ([]model.Order, error)
	UpdateOrderStatus(orderID uint64, status model.OrderStatus) error
}

// ContractRepository 約定用リポジトリ
type ContractRepository interface {
	GetContracts(orderID uint64) ([]model.Contract, error)
	UpsertContracts([]model.Contract) error
}

// RateRepository レート用リポジトリ
type RateRepository interface {
	AddOrderRate(*model.OrderRate) error
	GetCurrentRate(*model.CurrencyType, model.OrderSide) *float32
	GetRateHistory(*model.CurrencyType, model.OrderSide) []float32
	GetHistorySizeMax() int
}
