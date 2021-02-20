package strategy

import (
	"context"
	"log"
	ex "trading-bot/pkg/domain/exchange"
	"trading-bot/pkg/domain/model"
	repo "trading-bot/pkg/domain/repository"
)

type Sample struct {
	ExClient   ex.Client
	RepoClient repo.Client
}

func (s *Sample) Run(ctx context.Context) error {
	log.Println("start sample run")

	s.ExClient.GetOrderRate(model.Buy, model.BtcJpy)
	s.RepoClient.Update()

	log.Println("end sample run")
	return nil
}
