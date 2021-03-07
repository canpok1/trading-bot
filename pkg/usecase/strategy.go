package usecase

import (
	"context"
	"fmt"
	ex "trading-bot/pkg/domain/exchange"
	"trading-bot/pkg/domain/model"
	repo "trading-bot/pkg/domain/repository"
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

// StrategyParams 戦略用パラメータ
type StrategyParams struct {
	ExCli     ex.Client
	OrderRepo repo.OrderRepository
	RateRepo  repo.RateRepository
	Pair      *model.CurrencyPair
}

// MakeStrategy 戦略を生成
func MakeStrategy(t StrategyType, facade *trade.Facade) (Strategy, error) {
	switch t {
	case WatchOnly:
		return strategy.NewWatchOnlyStrategy(facade)
	case FollowUptrend:
		return strategy.NewFollowUptrendStrategy(facade)
	case Scalping:
		return strategy.NewScalpingStrategy(facade)
	default:
		return nil, fmt.Errorf("strategy name is unknown; name = %s", t)
	}
}
