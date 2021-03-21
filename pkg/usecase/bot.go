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
	pair     model.CurrencyPair

	Config *BotConfig
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
		pair: model.CurrencyPair{
			Key:        config.Currency,
			Settlement: model.JPY,
		},
		Config: config,
	}
}

func (b *Bot) Trade(ctx context.Context) error {
	if b.strategy == nil {
		return nil
	}

	rr, err := b.facade.GetRates(&model.CurrencyPair{
		Key:        b.Config.Currency,
		Settlement: model.JPY,
	})
	if err != nil {
		return err
	}

	pp, err := b.facade.GetOpenPositions()
	if err != nil {
		return err
	}

	cnt := len(pp)
	if cnt >= b.Config.PositionCountMax {
		b.logger.Debug("[buy] => skip buy (open pos count: %d >= max(%d))", cnt, b.Config.PositionCountMax)
	} else {
		if err := b.strategy.Buy(b.pair, rr, pp); err != nil {
			return err
		}
	}

	if err := b.strategy.Sell(b.pair, rr, pp); err != nil {
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
