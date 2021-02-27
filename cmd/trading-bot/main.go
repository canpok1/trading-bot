package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"time"
	"trading-bot/pkg/domain/model"
	"trading-bot/pkg/infrastructure/coincheck"
	"trading-bot/pkg/infrastructure/memory"
	"trading-bot/pkg/infrastructure/mysql"
	"trading-bot/pkg/usecase"
	"trading-bot/pkg/usecase/trade"

	"github.com/BurntSushi/toml"
	"golang.org/x/sync/errgroup"
)

func main() {
	log.Println("===== START PROGRAM ====================")
	defer log.Println("===== END PROGRAM ======================")

	f := flag.String("f", "", "config file path")
	flag.Parse()
	log.Printf("config file: %s\n", *f)

	var conf model.Config
	if _, err := toml.DecodeFile(*f, &conf); err != nil {
		log.Fatal(err.Error())
	}

	exCli := &coincheck.Client{APIAccessKey: conf.Exchange.AccessKey, APISecretKey: conf.Exchange.SecretKey}
	rateRepo := memory.NewRateRepository(conf.RateHistorySize)
	orderRepo := mysql.NewClient(conf.DB.UserName, conf.DB.Password, conf.DB.Host, conf.DB.Port, conf.DB.Name)
	contractRepo := orderRepo
	positionRepo := orderRepo

	strategy := usecase.MakeStrategy(
		usecase.StrategyType(conf.StrategyName),
		trade.NewFacade(
			&model.CurrencyPair{
				Key:        model.CurrencyType(conf.TargetCurrency),
				Settlement: model.JPY,
			},
			exCli,
			rateRepo,
			orderRepo,
			contractRepo,
			positionRepo,
		),
	)

	if strategy == nil {
		log.Fatalf("strategy name is unknown; name = %s", conf.StrategyName)
	}

	log.Printf("strategy: %s\n", conf.StrategyName)
	log.Printf("trade interval: %dsec\n", conf.TradeIntervalSeconds)
	log.Printf("target: %s\n", conf.TargetCurrency)
	log.Println("======================================")

	rootCtx, cancel := context.WithCancel(context.Background())
	errGroup, ctx := errgroup.WithContext(rootCtx)
	errGroup.Go(func() error {
		quit := make(chan os.Signal)
		defer close(quit)
		signal.Notify(quit, os.Interrupt)
		select {
		case <-quit:
			log.Println("terminating ...")
			cancel()
		case <-ctx.Done():
		}
		return nil
	})
	errGroup.Go(func() error {
		ticker := time.NewTicker(time.Duration(conf.TradeIntervalSeconds) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := strategy.Trade(ctx); err != nil {
					log.Printf("error occured, %v", err)
				}
			case <-ctx.Done():
				return nil
			}
		}
	})

	if err := errGroup.Wait(); err != nil {
		log.Fatalf("error occured, %v", err)
	}
}
