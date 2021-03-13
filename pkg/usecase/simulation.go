package usecase

import (
	"context"
	"fmt"
	"trading-bot/pkg/domain"
	"trading-bot/pkg/domain/repository"
	"trading-bot/pkg/infrastructure/memory"
)

// Simulator シミュレーター
type Simulator struct {
	Strategy     Strategy
	TradeRepo    repository.TradeRepository
	ExchangeMock *memory.ExchangeMock
	Logger       domain.Logger
}

// Run シミュレーション実施
func (s *Simulator) Run(ctx context.Context) (float64, error) {
	if err := s.TradeRepo.TruncateAll(); err != nil {
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

	return s.TradeRepo.GetProfit()
}
