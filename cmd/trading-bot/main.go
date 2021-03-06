package main

import (
	"context"
	"fmt"
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

	"github.com/kelseyhightower/envconfig"
	"golang.org/x/sync/errgroup"
)

const (
	rateHistorySize = 5000
)

func main() {
	log.Println("===== START PROGRAM ====================")
	defer log.Println("===== END PROGRAM ======================")

	var conf model.Config
	if err := envconfig.Process("BOT", &conf); err != nil {
		log.Fatal(err.Error())
	}

	exCli := &coincheck.Client{APIAccessKey: conf.Exchange.AccessKey, APISecretKey: conf.Exchange.SecretKey}
	rateRepo := memory.NewRateRepository(rateHistorySize)
	orderRepo := mysql.NewClient(conf.DB.UserName, conf.DB.Password, conf.DB.Host, conf.DB.Port, conf.DB.Name)
	contractRepo := orderRepo
	positionRepo := orderRepo

	strategyType := usecase.StrategyType(os.Args[1])
	strategy, err := usecase.MakeStrategy(
		strategyType,
		trade.NewFacade(
			exCli,
			rateRepo,
			orderRepo,
			contractRepo,
			positionRepo,
		),
	)

	if err != nil {
		log.Fatalf(err.Error())
	}

	log.Printf("strategy: %s\n", strategyType)
	log.Printf("rate log interval: %dsec\n", conf.RateLogIntervalSeconds)
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
		if conf.RateLogIntervalSeconds == 0 {
			return nil
		}

		// レートの定期保存
		jst := time.FixedZone("Asia/Tokyo", 9*60*60)
		beginTime := time.Now().UTC().In(jst).Format("20060102_150405_JST")
		loggers := []usecase.RateLogger{}

		for _, pair := range []model.CurrencyPair{model.BtcJpy, model.MonaJpy} {
			path := fmt.Sprintf("./data/simulator/historical_%s_%s.csv", pair.String(), beginTime)
			loggers = append(loggers, *usecase.NewRateLogger(exCli, &pair, path))
		}

		ticker := time.NewTicker(time.Duration(conf.RateLogIntervalSeconds) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				for _, logger := range loggers {
					if err := logger.AppendLog(); err != nil {
						return fmt.Errorf("failed to logging rate, error: %w", err)
					}
				}
			case <-ctx.Done():
				return nil
			}
		}
	})

	errGroup.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				return nil
			default:
				if err := strategy.Trade(ctx); err != nil {
					log.Printf("error occured in trade, %v", err)
				}
				if err := strategy.Wait(ctx); err != nil {
					log.Printf("error occured in wait, %v", err)
				}
			}
		}
	})

	if err := errGroup.Wait(); err != nil {
		log.Fatalf("error occured, %v", err)
	}
}
