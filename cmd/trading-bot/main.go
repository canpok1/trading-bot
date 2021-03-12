package main

import (
	"context"
	"fmt"
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
	logger := memory.Logger{Level: memory.Debug}

	logger.Info("===== START PROGRAM ====================")
	defer logger.Info("===== END PROGRAM ======================")

	var conf model.Config
	if err := envconfig.Process("BOT", &conf); err != nil {
		logger.Error(err.Error())
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
		&logger,
	)

	if err != nil {
		logger.Error(err.Error())
	}

	logger.Info("strategy: %s\n", strategyType)
	logger.Info("rate log interval: %dsec\n", conf.RateLogIntervalSeconds)
	logger.Info("======================================")

	rootCtx, cancel := context.WithCancel(context.Background())
	errGroup, ctx := errgroup.WithContext(rootCtx)
	errGroup.Go(func() error {
		quit := make(chan os.Signal)
		defer close(quit)
		signal.Notify(quit, os.Interrupt)
		select {
		case <-quit:
			logger.Info("terminating ...")
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
		rLoggers := []usecase.RateLogger{}

		for _, pair := range []model.CurrencyPair{model.BtcJpy, model.MonaJpy} {
			path := fmt.Sprintf("./data/simulator/historical_%s_%s.csv", pair.String(), beginTime)
			rLoggers = append(rLoggers, *usecase.NewRateLogger(exCli, &pair, path))
		}

		ticker := time.NewTicker(time.Duration(conf.RateLogIntervalSeconds) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				for _, rLogger := range rLoggers {
					if err := rLogger.AppendLog(); err != nil {
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
					logger.Error("error occured in trade, %v", err)
				}
				if err := strategy.Wait(ctx); err != nil {
					logger.Error("error occured in wait, %v", err)
				}
			}
		}
	})

	if err := errGroup.Wait(); err != nil {
		logger.Error("error occured, %v", err)
	}
}
