package strategy

import (
	"context"
	"log"
	"time"
	"trading-bot/pkg/domain/model"
	"trading-bot/pkg/usecase/trade"

	"github.com/BurntSushi/toml"
)

// WatchOnlyStrategy 定期取得のみ
type WatchOnlyStrategy struct {
	facade         *trade.Facade
	interval       time.Duration
	targetCurrency *model.CurrencyPair
}

// NewWatchOnlyStrategy 戦略を生成
func NewWatchOnlyStrategy(f *trade.Facade) (*WatchOnlyStrategy, error) {
	s := &WatchOnlyStrategy{
		facade: f,
	}

	if err := s.loadConfig(); err != nil {
		return nil, err
	}

	return s, nil
}

// Trade 取引処理
func (s *WatchOnlyStrategy) Trade(ctx context.Context) error {
	if err := s.loadConfig(); err != nil {
		return err
	}

	if err := s.facade.FetchAll(s.targetCurrency); err != nil {
		return err
	}

	log.Printf("buy rate: %v\n", s.facade.GetBuyRateHistory(s.targetCurrency))
	log.Printf("sell rate: %v\n", s.facade.GetSellRateHistory(s.targetCurrency))

	pp, err := s.facade.GetOpenPositions()
	if err != nil {
		return err
	}
	log.Printf("open position count: %d\n", len(pp))
	for _, p := range pp {
		log.Printf("open position id: %d\n", p.ID)
		{
			cc, err := s.facade.GetContracts(p.OpenerOrder.ID)
			if err != nil {
				return err
			}
			log.Printf("open order id: %d, contract count: %d\n", p.OpenerOrder.ID, len(cc))
			for _, c := range cc {
				log.Printf("contract: %#v\n", c)
			}
		}
		{
			cc, err := s.facade.GetContracts(p.CloserOrder.ID)
			if err != nil {
				return err
			}
			log.Printf("close order id: %d, contract count: %d\n", p.CloserOrder.ID, len(cc))
			for _, c := range cc {
				log.Printf("contract: %#v\n", c)
			}
		}
	}

	return nil
}

// Wait 待機
func (s *WatchOnlyStrategy) Wait(ctx context.Context) error {
	log.Printf("waiting ... (%v)\n", s.interval)
	ctx, cancel := context.WithTimeout(ctx, s.interval)
	defer cancel()
	select {
	case <-ctx.Done():
		if ctx.Err() != context.Canceled && ctx.Err() != context.DeadlineExceeded {
			return ctx.Err()
		}
		return nil
	}
}

func (s *WatchOnlyStrategy) loadConfig() error {
	const configPath = "./configs/bot-watch-only.toml"
	var conf watchOnlyConfig
	if _, err := toml.DecodeFile(configPath, &conf); err != nil {
		return err
	}

	s.interval = time.Duration(conf.IntervalSeconds) * time.Second
	s.targetCurrency = &model.CurrencyPair{
		Key:        model.CurrencyType(conf.TargetCurrency),
		Settlement: model.JPY,
	}

	return nil
}

type watchOnlyConfig struct {
	TargetCurrency  string `toml:"target_currency"`
	IntervalSeconds int    `toml:"interval_seconds"`
}
