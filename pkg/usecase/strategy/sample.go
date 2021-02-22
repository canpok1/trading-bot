package strategy

import (
	"context"
	"fmt"
	"log"
	ex "trading-bot/pkg/domain/exchange"
	"trading-bot/pkg/domain/model"
)

// Sample サンプル戦略
type Sample struct {
	ExClient ex.Client
	Pair     *model.CurrencyPair
}

// NewSample サンプル戦略を生成
func NewSample(ec ex.Client, p *model.CurrencyPair) *Sample {
	return &Sample{
		ExClient: ec,
		Pair:     p,
	}
}

// Trade 取引処理
func (s *Sample) Trade(ctx context.Context) error {
	rate, err := s.ExClient.GetOrderRate(model.Buy, *s.Pair)
	if err != nil {
		return fmt.Errorf("failed to get order rate; %w", err)
	}

	balance, err := s.ExClient.GetAccountBalance()
	if err != nil {
		return fmt.Errorf("failed to get account balance; %w", err)
	}

	log.Printf("rate : %s %.3f, balance: %#v\n", rate.Pair.String(), rate.Rate, balance)

	return nil
}
