package main

import (
	"context"
	"os"
	"trading-bot/pkg/domain"
	"trading-bot/pkg/domain/model"
	"trading-bot/pkg/infrastructure/memory"
	"trading-bot/pkg/infrastructure/mysql"
	"trading-bot/pkg/usecase"
	"trading-bot/pkg/usecase/trade"

	"github.com/BurntSushi/toml"
	"github.com/kelseyhightower/envconfig"
)

func main() {
	logger := memory.Logger{Level: memory.Debug}

	logger.Info("===== START PROGRAM ====================")
	defer logger.Info("===== END PROGRAM ======================")

	simulator, err := setup(&logger, "./configs/simulator.toml")
	if err != nil {
		logger.Error("error occured, %v\n", err)
	}

	logger.Info("======================================")

	profit, err := simulator.Run(context.Background())
	if err != nil {
		logger.Error("error occured, %v\n", err)
	} else {
		logger.Info("profit: %.3f", profit)
	}
}

func setup(logger domain.Logger, configPath string) (*usecase.Simulator, error) {
	var conf model.Config
	if err := envconfig.Process("BOT", &conf); err != nil {
		return nil, err
	}

	var sConf model.SimulatorConfig
	if _, err := toml.DecodeFile(configPath, &sConf); err != nil {
		return nil, err
	}

	logger.Info("strategy: %s\n", sConf.StrategyName)
	logger.Info("rate: %s\n", sConf.RateHistoryFile)

	historical, err := os.Open(sConf.RateHistoryFile)
	if err != nil {
		return nil, err
	}
	exCli, err := memory.NewExchangeMock(historical, sConf.Slippage)
	if err != nil {
		return nil, err
	}

	rateRepo := memory.NewRateRepository(sConf.RateHistorySize)
	mysqlCli := mysql.NewClient(conf.DB.UserName, conf.DB.Password, conf.DB.Host, conf.DB.Port, conf.DB.Name)

	strategy, err := usecase.MakeStrategy(
		usecase.StrategyType(sConf.StrategyName),
		trade.NewFacade(
			exCli,
			rateRepo,
			mysqlCli,
			mysqlCli,
			mysqlCli,
		),
		logger,
	)

	if err != nil {
		return nil, err
	}

	return &usecase.Simulator{
		Strategy:     strategy,
		TradeRepo:    mysqlCli,
		ExchangeMock: exCli,
		Logger:       logger,
	}, nil
}
