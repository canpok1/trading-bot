package usecase

import (
	"context"
	"fmt"
	"trading-bot/pkg/domain"
	"trading-bot/pkg/domain/model"
	"trading-bot/pkg/usecase/strategy"
	"trading-bot/pkg/usecase/trade"
)

// Strategy 戦略
type Strategy interface {
	// Buy 定期実行時の買い注文
	Buy(pair model.CurrencyPair, positions []model.Position) error

	// Sell 定期実行時の売り注文
	Sell(pair model.CurrencyPair, positions []model.Position) error

	// BuyTradeCallback 買い取引検知時の処理
	BuyTradeCallback(pair model.CurrencyPair, rate float64) error

	// SellTradeCallback 売り取引検知時の処理
	SellTradeCallback(pair model.CurrencyPair, rate float64) error

	// Wait 待機
	Wait(ctx context.Context) error
}

// StrategyType 戦略種別
type StrategyType string

const (
	None StrategyType = "none"
	// Scalping 短期の売買を繰り返す
	Scalping StrategyType = "scalping"
	// Uptrend 上昇トレンド追従
	Uptrend StrategyType = "uptrend"
	// Range レンジ相場用
	Range StrategyType = "range"
	// Inago イナゴトレード
	Inago StrategyType = "inago"
)

// MakeStrategy 戦略を生成
func MakeStrategy(t StrategyType, facade *trade.Facade, logger domain.Logger) (Strategy, error) {
	p := fmt.Sprintf("./configs/bot-%s.toml", t)
	switch t {
	case None:
		return nil, nil
	// case WatchOnly:
	// 	return strategy.NewWatchOnlyStrategy(facade, logger, p)
	// case FollowUptrend:
	// 	return strategy.NewFollowUptrendStrategy(facade, logger, p)
	// case Scalping:
	// 	config, err := strategy.NewScalpingConfig(p)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	return strategy.NewScalpingStrategy(facade, logger, config)
	// case Uptrend:
	// 	return &strategy.Uptrend{Facade: facade, Logger: logger}, nil
	case Range:
		config, err := strategy.NewRangeConfig(p)
		if err != nil {
			return nil, err
		}
		return strategy.NewRangeStrategy(facade, logger, config)
	case Inago:
		config, err := strategy.NewInagoConfig(p)
		if err != nil {
			return nil, err
		}
		return strategy.NewInagoStrategy(facade, logger, config)
	default:
		return nil, fmt.Errorf("strategy name is unknown; name = %s", t)
	}
}
