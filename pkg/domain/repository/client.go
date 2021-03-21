package repository

import (
	"time"
	"trading-bot/pkg/domain/model"
)

// RateRepository レート用リポジトリ
type RateRepository interface {
	AddRates(*model.CurrencyPair, float64, time.Time) error
	GetRate(*model.CurrencyPair) (float64, error)
	GetRates(*model.CurrencyPair, *time.Duration) ([]float64, error)
}

// OrderRepository 注文用リポジトリ
type OrderRepository interface {
	GetOrder(uint64) (*model.Order, error)
	GetOpenOrders() ([]model.Order, error)
	UpdateStatus(orderID uint64, status model.OrderStatus) error
}

// ContractRepository 約定用リポジトリ
type ContractRepository interface {
	GetContracts(orderID uint64) ([]model.Contract, error)
	UpsertContracts([]model.Contract) error
}

// PositionRepository ポジション用リポジトリ
type PositionRepository interface {
	AddNewOrder(*model.Order) (*model.Position, error)
	AddSettleOrder(uint64, *model.Order) (*model.Position, error)
	CancelSettleOrder(uint64) (*model.Position, error)
	GetOpenPositions() ([]model.Position, error)
}

type TradeRepository interface {
	GetOrder(uint64) (*model.Order, error)
	GetOpenOrders() ([]model.Order, error)
	UpdateStatus(orderID uint64, status model.OrderStatus) error
	GetContracts(orderID uint64) ([]model.Contract, error)
	UpsertContracts([]model.Contract) error
	AddNewOrder(*model.Order) (*model.Position, error)
	AddSettleOrder(uint64, *model.Order) (*model.Position, error)
	CancelSettleOrder(uint64) (*model.Position, error)
	GetOpenPositions() ([]model.Position, error)
	TruncateAll() error
	GetProfit() (float64, error)
	AddRates(*model.CurrencyPair, float64, time.Time) error
	GetRate(*model.CurrencyPair) (float64, error)
	GetRates(*model.CurrencyPair, *time.Duration) ([]float64, error)
}
