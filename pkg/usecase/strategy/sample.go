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

// Tick 情報取得処理
func (s *Sample) Tick(ctx context.Context) error {
	contracts, err := s.ExClient.GetContracts()
	if err != nil {
		return err
	}
	if err := s.RepoClient.UpdateContracts(contracts); err != nil {
		return err
	}

	orders, err := s.ExClient.GetOpenOrders()
	if err != nil {
		return err
	}
	if err := s.RepoClient.UpsertOrders(orders); err != nil {
		return err
	}

	return nil
}

// Trade 取引処理
func (s *Sample) Trade(ctx context.Context) error {
	rate, err := s.ExClient.GetOrderRate(model.Buy, model.BtcJpy)
	if err != nil {
		return fmt.Errorf("failed to get order rate; %w", err)
	}

	balance, err := s.ExClient.GetAccountBalance()
	if err != nil {
		return fmt.Errorf("failed to get account balance; %w", err)
	}

	log.Printf("rate: %s %.3f, balance: JPY:%.2f,BTC:%.2f\n", rate.Pair.String(), rate.Rate, balance.Jpy, balance.Btc)

	return nil
}
