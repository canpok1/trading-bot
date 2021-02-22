package usecase

import (
	"context"
	ex "trading-bot/pkg/domain/exchange"
	"trading-bot/pkg/domain/model"
	repo "trading-bot/pkg/domain/repository"
	"trading-bot/pkg/usecase/strategy"
)

// Strategy 戦略
type Strategy interface {
	// Trade 取引
	Trade(ctx context.Context) error
}

// StrategyType 戦略種別
type StrategyType string

const (
	// Sample サンプル戦略
	Sample StrategyType = "sample"
	// FollowUptrend 上昇トレンド追従戦略
	FollowUptrend StrategyType = "follow_uptrend"
)

// StrategyParams 戦略用パラメータ
type StrategyParams struct {
	ExCli     ex.Client
	OrderRepo repo.OrderRepository
	RateRepo  repo.RateRepository
	Pair      *model.CurrencyPair
}

// MakeStrategy 戦略を生成
func MakeStrategy(t StrategyType, p *StrategyParams) Strategy {
	switch t {
	case Sample:
		return strategy.NewSample(p.ExCli, p.Pair)
	case FollowUptrend:
		return strategy.NewFollowUptrendStrategy(p.ExCli, p.OrderRepo, p.RateRepo, p.Pair)
	default:
		return nil
	}
}
