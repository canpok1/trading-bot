package usecase

import (
	"context"
	ex "trading-bot/pkg/domain/exchange"
	repo "trading-bot/pkg/domain/repository"
	"trading-bot/pkg/usecase/strategy"
)

// Strategy 戦略
type Strategy interface {
	// Tick 情報更新
	Tick(ctx context.Context) error
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

// MakeStrategy 戦略を生成
func MakeStrategy(t StrategyType, exCli ex.Client, repoCli repo.Client) Strategy {
	switch t {
	case Sample:
		return &strategy.Sample{ExClient: exCli, RepoClient: repoCli}
	case FollowUptrend:
		return &strategy.FollowUptrendStrategy{ExClient: exCli, RepoClient: repoCli}
	default:
		return nil
	}
}
