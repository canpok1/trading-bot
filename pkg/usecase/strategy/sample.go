package strategy

import (
	"context"
	"fmt"
	"log"
	"time"
	ex "trading-bot/pkg/domain/exchange"
	"trading-bot/pkg/domain/model"
	repo "trading-bot/pkg/domain/repository"
)

type Sample struct {
	ExClient   ex.Client
	RepoClient repo.Client
}

func (s *Sample) Run(ctx context.Context, interval time.Duration) error {
	log.Println("start sample run")

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := s.run(); err != nil {
				return fmt.Errorf("failed to Run; %w", err)
			}
		case <-ctx.Done():
			log.Println("end sample run")
			return nil
		}
	}
}

func (s *Sample) run() error {
	rate, err := s.ExClient.GetOrderRate(model.Buy, model.BtcJpy)
	if err != nil {
		log.Printf("failed to get order rate; %v", err)
	} else {
		log.Printf("rate = %#v\n", rate)
	}

	balance, err := s.ExClient.GetAccountBalance()
	if err != nil {
		log.Printf("failed to get account balance; %v", err)
	} else {
		log.Printf("balance = %#v\n", balance)
	}

	//s.RepoClient.Update()

	return nil
}
