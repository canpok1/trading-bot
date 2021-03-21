package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"
	"trading-bot/pkg/domain"
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
	rateDuration = 10 * time.Minute
)

func main() {
	logger := memory.Logger{Level: memory.Debug}

	logger.Info("===== START PROGRAM ====================")
	defer logger.Info("===== END PROGRAM ======================")

	var config model.Config
	if err := envconfig.Process("BOT", &config); err != nil {
		logger.Error(err.Error())
		return
	}
	strategyType := usecase.StrategyType(os.Args[1])

	logger.Info("strategy: %s\n", strategyType)
	logger.Info("rate log interval: %dsec\n", config.RateLogIntervalSeconds)
	logger.Info("======================================")

	bot, fetchers, err := setup(&logger, &config, strategyType)
	if err != nil {
		logger.Error(err.Error())
		return
	}

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
		// レートの定期保存
		ticker := time.NewTicker(time.Duration(config.RateLogIntervalSeconds) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				for _, f := range fetchers {
					if err := f.Fetch(); err != nil {
						return fmt.Errorf("failed to fetch, error: %w", err)
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
				if err := bot.Trade(ctx); err != nil {
					logger.Error("error occured in trade, %v", err)
				}
				if err := bot.Wait(ctx); err != nil {
					logger.Error("error occured in wait, %v", err)
				}
			}
		}
	})

	if err := errGroup.Wait(); err != nil {
		logger.Error("error occured, %v", err)
	}
}

func setup(logger domain.Logger, config *model.Config, strategyType usecase.StrategyType) (*usecase.Bot, []usecase.Fetcher, error) {
	exCli := &coincheck.Client{APIAccessKey: config.Exchange.AccessKey, APISecretKey: config.Exchange.SecretKey}
	mysqlCli := mysql.NewClient(config.DB.UserName, config.DB.Password, config.DB.Host, config.DB.Port, config.DB.Name)

	d := rateDuration
	facade := trade.NewFacade(
		exCli,
		mysqlCli,
		mysqlCli,
		mysqlCli,
		mysqlCli,
		&d,
	)

	strategy, err := usecase.MakeStrategy(
		strategyType,
		facade,
		logger,
	)
	if err != nil {
		return nil, nil, err
	}

	bot := usecase.NewBot(logger, facade, strategy, &usecase.BotConfig{
		Currency:         model.CurrencyType(config.TargetCurrency),
		IntervalSeconds:  config.RateLogIntervalSeconds,
		PositionCountMax: config.PositionCountMax,
	})

	fetchers := []usecase.Fetcher{}
	if config.RateLogIntervalSeconds != 0 {
		for _, pair := range []model.CurrencyPair{model.BtcJpy, model.MonaJpy} {
			fetchers = append(fetchers, *usecase.NewFetcher(exCli, pair, mysqlCli))
		}
	}

	return bot, fetchers, nil
}
