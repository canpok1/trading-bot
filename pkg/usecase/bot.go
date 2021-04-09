package usecase

import (
	"context"
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

	pp, err := b.facade.GetOpenPositions()
	if err != nil {
		return err
	}

	cnt := len(pp)
	if cnt >= b.Config.PositionCountMax {
		b.logger.Debug("[buy] => skip buy (open pos count: %d >= max(%d))", cnt, b.Config.PositionCountMax)
	} else {
		if err := b.strategy.Buy(b.pair, pp); err != nil {
			return err
		}
	}

	if err := b.strategy.Sell(b.pair, pp); err != nil {
		return err
	}

	return nil
}

// Wait 待機
func (b *Bot) Wait(ctx context.Context) error {
	return b.strategy.Wait(ctx)
}

func (b *Bot) ReceiveTrade(side model.OrderSide, rate float64) error {
	if b.strategy == nil {
		return nil
	}

	if side == model.BuySide {
		pp, err := b.facade.GetOpenPositions()
		if err != nil {
			return err
		}

		cnt := len(pp)
		if cnt >= b.Config.PositionCountMax {
			b.logger.Debug("[buy] => skip buy (open pos count: %d >= max(%d))", cnt, b.Config.PositionCountMax)
			return nil
		}

		return b.strategy.BuyTradeCallback(b.pair, rate)
	} else {
		return b.strategy.SellTradeCallback(b.pair, rate)
	}
}
