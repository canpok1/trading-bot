package main

import (
	"context"
	"os"
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

	var conf model.Config
	if err := envconfig.Process("BOT", &conf); err != nil {
		logger.Error(err.Error())
	}

	var sConf model.SimulatorConfig
	const configPath = "./configs/simulator.toml"
	if _, err := toml.DecodeFile(configPath, &sConf); err != nil {
		logger.Error(err.Error())
	}

	historical, err := os.Open(sConf.RateHistoryFile)
	if err != nil {
		logger.Error(err.Error())
	}
	exCli, err := memory.NewExchangeMock(historical, sConf.Slippage)
	if err != nil {
		logger.Error(err.Error())
	}

	rateRepo := memory.NewRateRepository(sConf.RateHistorySize)
	mysqlCli := mysql.NewClient(conf.DB.UserName, conf.DB.Password, conf.DB.Host, conf.DB.Port, conf.DB.Name)

	if err := mysqlCli.TruncateAll(); err != nil {
		logger.Info("failed to truncate all, %v", err)
		return
	}

	strategy, err := usecase.MakeStrategy(
		usecase.StrategyType(sConf.StrategyName),
		trade.NewFacade(
			exCli,
			rateRepo,
			mysqlCli,
			mysqlCli,
			mysqlCli,
		),
		&logger,
	)

	if err != nil {
		logger.Error(err.Error())
	}

	logger.Info("strategy: %s\n", sConf.StrategyName)
	logger.Info("rate: %s\n", sConf.RateHistoryFile)
	logger.Info("======================================")

	ctx := context.Background()
	for {
		if err := strategy.Trade(ctx); err != nil {
			logger.Error("error occured, %v\n", err)
			break
		}

		if !exCli.NextStep() {
			break
		}
	}
}
