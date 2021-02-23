package usecase

import (
	ex "trading-bot/pkg/domain/exchange"
	"trading-bot/pkg/domain/model"
	repo "trading-bot/pkg/domain/repository"
)

// RateWatcher レート監視者
type RateWatcher struct {
	rateRepo  repo.RateRepository
	exClient  ex.Client
	orderRepo repo.OrderRepository
}

// NewRateWatcher 生成
func NewRateWatcher(repo repo.RateRepository, exCli ex.Client, orderCli repo.OrderRepository) *RateWatcher {
	return &RateWatcher{
		rateRepo:  repo,
		exClient:  exCli,
		orderRepo: orderCli,
	}
}

// Watch 監視
func (w *RateWatcher) Watch(p *model.CurrencyPair) error {
	buyRate, err := w.exClient.GetOrderRate(p, model.BuySide)
	if err != nil {
		return err
	}
	if err := w.rateRepo.AddOrderRate(buyRate); err != nil {
		return err
	}

	sellRate, err := w.exClient.GetOrderRate(p, model.SellSide)
	if err != nil {
		return err
	}
	if err := w.rateRepo.AddOrderRate(sellRate); err != nil {
		return err
	}

	return nil
}
