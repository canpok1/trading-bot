package usecase

import (
	"context"
	"fmt"
	"trading-bot/pkg/domain"
	"trading-bot/pkg/usecase/strategy"
	"trading-bot/pkg/usecase/trade"
)

// Strategy 戦略
type Strategy interface {
	// Trade 取引
	Trade(ctx context.Context) error
	Wait(ctx context.Context) error
}

// StrategyType 戦略種別
type StrategyType string

const (
	// WatchOnly 定期取得のみ（売買しない）
	WatchOnly StrategyType = "watch_only"
	// FollowUptrend 上昇トレンド追従戦略
	FollowUptrend StrategyType = "follow_uptrend"
	// Scalping 短期の売買を繰り返す
	Scalping StrategyType = "scalping"
)

// MakeStrategy 戦略を生成
func MakeStrategy(t StrategyType, facade *trade.Facade, logger domain.Logger) (Strategy, error) {
	switch t {
	case WatchOnly:
		return strategy.NewWatchOnlyStrategy(facade, logger)
	case FollowUptrend:
		return strategy.NewFollowUptrendStrategy(facade, logger)
	case Scalping:
		return strategy.NewScalpingStrategy(facade, logger)
	default:
		return nil, fmt.Errorf("strategy name is unknown; name = %s", t)
	}
}
