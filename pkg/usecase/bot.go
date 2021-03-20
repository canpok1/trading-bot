package usecase

import (
	"context"
	"time"
	"trading-bot/pkg/domain"
	"trading-bot/pkg/domain/model"
	"trading-bot/pkg/usecase/trade"
)

type Bot struct {
	logger   domain.Logger
	facade   *trade.Facade
	strategy Strategy
	Config   *BotConfig
}

type BotConfig struct {
	Currency         model.CurrencyType
	IntervalSeconds  int
	PositionCountMax int
}

func NewBot(l domain.Logger, f *trade.Facade, s Strategy, config *BotConfig) *Bot {
	return &Bot{
		logger:   l,
		facade:   f,
		strategy: s,
		Config:   config,
	}
}

func (b *Bot) Trade(ctx context.Context) error {
	pair := model.CurrencyPair{
		Key:        b.Config.Currency,
		Settlement: model.JPY,
	}
	if err := b.facade.FetchAll(&pair); err != nil {
		return err
	}

	rr := b.facade.GetSellRateHistory64(&pair)

	pp, err := b.facade.GetOpenPositions()
	if err != nil {
		return err
	}

	if err := b.strategy.Buy(rr, pp); err != nil {
		return err
	}

	if err := b.strategy.Sell(rr, pp); err != nil {
		return err
	}

	return nil
}

// Wait 待機
func (b *Bot) Wait(ctx context.Context) error {
	interval := time.Duration(b.Config.IntervalSeconds) * time.Second

	b.logger.Debug("waiting ... (%v)\n", interval)
	ctx, cancel := context.WithTimeout(ctx, interval)
	defer cancel()

	<-ctx.Done()

	if ctx.Err() != context.Canceled && ctx.Err() != context.DeadlineExceeded {
		return ctx.Err()
	}
	return nil
}
