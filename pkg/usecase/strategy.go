package usecase

import (
	"context"
	ex "trading-bot/pkg/domain/exchange"
	repo "trading-bot/pkg/domain/repository"
	"trading-bot/pkg/usecase/strategy"
)

type Strategy interface {
	Run(context.Context) error
}

type StrategyType int

const (
	Sample StrategyType = iota
)

func MakeStrategy(t StrategyType, exCli ex.Client, repoCli repo.Client) Strategy {
	switch t {
	case Sample:
		return &strategy.Sample{ExClient: exCli, RepoClient: repoCli}
	default:
		return nil
	}
}
