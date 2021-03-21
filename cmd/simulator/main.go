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

	"github.com/kelseyhightower/envconfig"
)

func main() {
	logger := memory.Logger{Level: memory.Debug}

	logger.Info("===== START PROGRAM ====================")
	defer logger.Info("===== END PROGRAM ======================")

	simulator, err := setup(&logger, "./configs/simulator.toml")
	if err != nil {
		logger.Error("error occured, %v\n", err)
		return
	}

	logger.Info("======================================")

	profit, err := simulator.Run(context.Background())
	if err != nil {
		logger.Error("error occured, %v\n", err)
		return
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
	if err := envconfig.Process("BOT", &sConf); err != nil {
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

	mysqlCli := mysql.NewClient(conf.DB.UserName, conf.DB.Password, conf.DB.Host, conf.DB.Port, conf.DB.Name)

	facade := trade.NewFacade(
		exCli,
		mysqlCli,
		mysqlCli,
		mysqlCli,
		mysqlCli,
		nil,
	)
	strategy, err := usecase.MakeStrategy(
		usecase.StrategyType(sConf.StrategyName),
		facade,
		logger,
	)

	if err != nil {
		return nil, err
	}

	bot := usecase.NewBot(logger, facade, strategy, &usecase.BotConfig{
		Currency:         model.CurrencyType(conf.TargetCurrency),
		IntervalSeconds:  0,
		PositionCountMax: conf.PositionCountMax,
	})

	pair := model.CurrencyPair{
		Key:        model.CurrencyType(conf.TargetCurrency),
		Settlement: model.JPY,
	}
	fetcher := usecase.NewFetcher(exCli, pair, mysqlCli)

	return &usecase.Simulator{
		Bot:          bot,
		Fetcher:      fetcher,
		TradeRepo:    mysqlCli,
		ExchangeMock: exCli,
		Logger:       logger,
	}, nil
}
