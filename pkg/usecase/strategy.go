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
	// Trade 取引
	Trade(ctx context.Context) error

	// Wait 待機
	Wait(ctx context.Context) error

	// GetCurrency 通貨種別を取得
	GetCurrency() model.CurrencyType
}

// StrategyType 戦略種別
type StrategyType string

const (
	// WatchOnly 定期取得のみ（売買しない）
	WatchOnly StrategyType = "watch-only"
	// FollowUptrend 上昇トレンド追従戦略
	FollowUptrend StrategyType = "follow-uptrend"
	// Scalping 短期の売買を繰り返す
	Scalping StrategyType = "scalping"
)

// MakeStrategy 戦略を生成
func MakeStrategy(t StrategyType, facade *trade.Facade, logger domain.Logger) (Strategy, error) {
	p := fmt.Sprintf("./configs/bot-%s.toml", t)
	switch t {
	case WatchOnly:
		return strategy.NewWatchOnlyStrategy(facade, logger, p)
	case FollowUptrend:
		return strategy.NewFollowUptrendStrategy(facade, logger, p)
	case Scalping:
		config, err := strategy.NewScalpingConfig(p)
		if err != nil {
			return nil, err
		}
		return strategy.NewScalpingStrategy(facade, logger, config)
	default:
		return nil, fmt.Errorf("strategy name is unknown; name = %s", t)
	}
}
