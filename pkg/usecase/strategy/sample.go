package strategy

import (
	"context"
	"fmt"
	"log"
	ex "trading-bot/pkg/domain/exchange"
	"trading-bot/pkg/domain/model"
	repo "trading-bot/pkg/domain/repository"
)

// Sample サンプル戦略
type Sample struct {
	ExClient   ex.Client
	RepoClient repo.Client
}

// Run 取引処理
func (s *Sample) Run(ctx context.Context) error {
	rate, err := s.ExClient.GetOrderRate(model.Buy, model.BtcJpy)
	if err != nil {
		return fmt.Errorf("failed to get order rate; %w", err)
	}

	balance, err := s.ExClient.GetAccountBalance()
	if err != nil {
		return fmt.Errorf("failed to get account balance; %w", err)
	}

	log.Printf("rate: %s %.3f, balance: JPY:%.2f,BTC:%.2f\n", rate.Pair, rate.Rate, balance.Jpy, balance.Btc)

	return nil
}
