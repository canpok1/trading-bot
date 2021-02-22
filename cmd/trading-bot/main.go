package main

import (
	"context"
	"log"
	"time"
	"trading-bot/pkg/domain/model"
	"trading-bot/pkg/infrastructure/coincheck"
	"trading-bot/pkg/infrastructure/memory"
	"trading-bot/pkg/infrastructure/mysql"
	"trading-bot/pkg/usecase"

	"github.com/kelseyhightower/envconfig"
	"golang.org/x/sync/errgroup"
)

type Config struct {
	StrategyName         string `split_words:"true" required:"true"`
	TradeIntervalSeconds int    `split_words:"true" required:"true"`
	WatchIntervalSeconds int    `split_words:"true" required:"true"`
	RateHistorySize      int    `split_words:"true" required:"true"`
	TargetCurrency       string `split_words:"true" required:"true"`
	WarmupTimeSeconds    int    `split_words:"true" required:"true"`
	Exchange             Exchange
	DB                   DB
}

type Exchange struct {
	AccessKey string `split_words:"true" required:"true"`
	SecretKey string `split_words:"true" required:"true"`
}

type DB struct {
	Host     string `required:"true"`
	Port     int    `required:"true"`
	Name     string `required:"true"`
	UserName string `split_words:"true" required:"true"`
	Password string `required:"true"`
}

func main() {
	log.Println("===== START PROGRAM ====================")
	defer log.Println("===== END PROGRAM ====================")

	var conf Config
	if err := envconfig.Process("BOT", &conf); err != nil {
		log.Fatal(err.Error())
	}

	exCli := &coincheck.Client{APIAccessKey: conf.Exchange.AccessKey, APISecretKey: conf.Exchange.SecretKey}
	rateRepo := memory.NewRateRepository(conf.RateHistorySize)
	orderRepo := mysql.NewClient(conf.DB.UserName, conf.DB.Password, conf.DB.Host, conf.DB.Port, conf.DB.Name)

	watcher := usecase.NewRateWatcher(rateRepo, exCli, orderRepo)
	strategy := usecase.MakeStrategy(
		usecase.StrategyType(conf.StrategyName),
		&usecase.StrategyParams{
			ExCli:     exCli,
			OrderRepo: orderRepo,
			RateRepo:  rateRepo,
			Pair: &model.CurrencyPair{
				Key:        model.CurrencyType(conf.TargetCurrency),
				Settlement: model.JPY,
			},
		},
	)

	if strategy == nil {
		log.Fatalf("strategy name is unknown; name = %s", conf.StrategyName)
	}

	pair := model.CurrencyPair{
		Key:        model.CurrencyType(conf.TargetCurrency),
		Settlement: model.JPY,
	}

	log.Printf("strategy: %s\n", conf.StrategyName)
	log.Printf("trade interval: %dsec\n", conf.TradeIntervalSeconds)
	log.Printf("watch interval: %dsec\n", conf.WatchIntervalSeconds)
	log.Printf("target: %s\n", conf.TargetCurrency)
	log.Println("======================================")

	errGroup, ctx := errgroup.WithContext(context.Background())
	errGroup.Go(func() error {
		ticker := time.NewTicker(time.Duration(conf.WatchIntervalSeconds) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := watcher.Watch(&pair); err != nil {
					return err
				}
			case <-ctx.Done():
				return nil
			}
		}
	})
	errGroup.Go(func() error {
		log.Printf("warming up (%d sec) ...\n", conf.WarmupTimeSeconds)
		time.Sleep(time.Duration(conf.WarmupTimeSeconds) * time.Second)

		ticker := time.NewTicker(time.Duration(conf.TradeIntervalSeconds) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := strategy.Trade(ctx); err != nil {
					return err
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
