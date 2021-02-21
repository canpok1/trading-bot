package repository

import "trading-bot/pkg/domain/model"

type Client interface {
	GetOpenPositions() ([]model.Position, error)
	GetOrders() error
	UpsertOrders([]model.Order) error
	UpdateContracts([]model.Contract) error
	SaveProfit(jpy float32) error
}
