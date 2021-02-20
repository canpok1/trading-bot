package repository

import "trading-bot/pkg/domain/model"

type Client interface {
	GetOpenPositions() ([]model.Position, error)
	GetOrders() error
	UpsertOrders() error
	SaveProfit(jpy float32) error
}
