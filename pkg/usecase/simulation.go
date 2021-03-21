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
	Bot          *Bot
	Fetcher      *Fetcher
	ExchangeMock *memory.ExchangeMock
	TradeRepo    repository.TradeRepository
	Logger       domain.Logger
}

// Run シミュレーション実施
func (s *Simulator) Run(ctx context.Context) (float64, error) {
	if err := s.TradeRepo.TruncateAll(); err != nil {
		return 0, fmt.Errorf("failed to truncate all, %v", err)
	}

	for {
		if err := s.Fetcher.Fetch(); err != nil {
			return 0, err
		}

		if err := s.Bot.Trade(ctx); err != nil {
			return 0, err
		}

		if !s.ExchangeMock.NextStep() {
			break
		}
	}

	return s.TradeRepo.GetProfit()
}
