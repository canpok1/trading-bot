package usecase

import (
	"time"
	"trading-bot/pkg/domain/exchange"
	"trading-bot/pkg/domain/model"
	"trading-bot/pkg/domain/repository"
)

// Fetcher 情報取得
type Fetcher struct {
	pair   model.CurrencyPair
	exCli  exchange.Client
	rdsCli repository.TradeRepository
}

// NewFetcher 生成
func NewFetcher(exCli exchange.Client, pair model.CurrencyPair, rdsCli repository.TradeRepository) *Fetcher {
	return &Fetcher{
		exCli:  exCli,
		pair:   pair,
		rdsCli: rdsCli,
	}
}

// Fetch 各種情報を取得
func (f *Fetcher) Fetch() error {
	r, err := f.exCli.GetOrderRate(&f.pair, model.SellSide)
	if err != nil {
		return err
	}

	now := time.Now()
	if err := f.rdsCli.AddRates(&f.pair, float64(r.Rate), now); err != nil {
		return err
	}

	if err := f.fetchContracts(); err != nil {
		return err
	}

	if err := f.fetchOrders(); err != nil {
		return err
	}

	return nil
}

//fetchContracts 約定情報を更新
func (f *Fetcher) fetchContracts() error {
	oo, err := f.rdsCli.GetOpenOrders()
	if err != nil {
		return err
	}
	if len(oo) == 0 {
		return nil
	}

	cc, err := f.exCli.GetContracts()
	if err != nil {
		return err
	}

	targets := []model.Contract{}
	for _, c := range cc {
		for _, o := range oo {
			if c.OrderID == o.ID {
				targets = append(targets, c)
				break
			}
		}
	}

	if err := f.rdsCli.UpsertContracts(targets); err != nil {
		return err
	}

	return nil
}

// FetchOrders 注文情報を更新
func (f *Fetcher) fetchOrders() error {
	openOrders, err := f.exCli.GetOpenOrders(&f.pair)
	if err != nil {
		return err
	}

	registeredOrders, err := f.rdsCli.GetOpenOrders()
	if err != nil {
		return err
	}

	for _, o := range registeredOrders {
		opened := false
		for _, openOrder := range openOrders {
			if openOrder.ID == o.ID {
				opened = true
				break
			}
		}

		if opened {
			continue
		}
		if err := f.rdsCli.UpdateStatus(o.ID, model.Closed); err != nil {
			return err
		}
	}

	return nil
}
