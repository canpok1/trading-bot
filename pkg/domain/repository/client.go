package repository

import "trading-bot/pkg/domain/model"

// OrderRepository 注文用リポジトリ
type OrderRepository interface {
	AddOrder(*model.Order) error
	GetOpenOrders() ([]model.Order, error)
	UpdateOrderStatus(model.OrderStatus) error
}

// ContractRepository 約定用リポジトリ
type ContractRepository interface {
	AddContract(*model.Contract) error
}

// RateRepository レート用リポジトリ
type RateRepository interface {
	AddRate(*model.OrderRate) error
	GetCurrentRate(*model.CurrencyType) *model.OrderRate
	GetRateHistory(*model.CurrencyType) []model.OrderRate
}
