package usecase

import (
	"fmt"
	"trading-bot/pkg/domain"
	"trading-bot/pkg/domain/model"
	"trading-bot/pkg/usecase/strategy"
	"trading-bot/pkg/usecase/trade"
)

// Strategy 戦略
type Strategy interface {
	// Buy 買い注文
	Buy(pair model.CurrencyPair, rates []float64, positions []model.Position) error

	// Sell 売り注文
	Sell(pair model.CurrencyPair, rates []float64, positions []model.Position) error
}

// StrategyType 戦略種別
type StrategyType string

const (
	None StrategyType = "none"
	// Scalping 短期の売買を繰り返す
	Scalping StrategyType = "scalping"
	// Uptrend 上昇トレンド追従
	Uptrend StrategyType = "uptrend"
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
	case Scalping:
		config, err := strategy.NewScalpingConfig(p)
		if err != nil {
			return nil, err
		}
		return strategy.NewScalpingStrategy(facade, logger, config)
	case Uptrend:
		return &strategy.Uptrend{Facade: facade, Logger: logger}, nil
	default:
		return nil, fmt.Errorf("strategy name is unknown; name = %s", t)
	}
}
