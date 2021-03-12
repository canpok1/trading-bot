package usecase

import (
	"context"
	"fmt"
	"trading-bot/pkg/domain"
	"trading-bot/pkg/infrastructure/memory"
	"trading-bot/pkg/infrastructure/mysql"
)

// Simulator シミュレーター
type Simulator struct {
	Strategy     Strategy
	MysqlCli     *mysql.Client
	ExchangeMock *memory.ExchangeMock
	Logger       domain.Logger
}

// Run シミュレーション実施
func (s *Simulator) Run(ctx context.Context) (float64, error) {
	if err := s.MysqlCli.TruncateAll(); err != nil {
		return 0, fmt.Errorf("failed to truncate all, %v", err)
	}

	for {
		if err := s.Strategy.Trade(ctx); err != nil {
			return 0, err
		}

		if !s.ExchangeMock.NextStep() {
			break
		}
	}

	return s.MysqlCli.GetProfit()
}
