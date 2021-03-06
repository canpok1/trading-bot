package main

import (
	"context"
	"log"
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
	log.Println("===== START PROGRAM ====================")
	defer log.Println("===== END PROGRAM ======================")

	var conf model.Config
	if err := envconfig.Process("BOT", &conf); err != nil {
		log.Fatal(err.Error())
	}

	var sConf model.SimulatorConfig
	const configPath = "./configs/simulator.toml"
	if _, err := toml.DecodeFile(configPath, &sConf); err != nil {
		log.Fatal(err.Error())
	}

	historical, err := os.Open(sConf.RateHistoryFile)
	if err != nil {
		log.Fatal(err.Error())
	}
	exCli, err := memory.NewExchangeMock(historical, sConf.Slippage)
	if err != nil {
		log.Fatal(err.Error())
	}

	rateRepo := memory.NewRateRepository(sConf.RateHistorySize)
	mysqlCli := mysql.NewClient(conf.DB.UserName, conf.DB.Password, conf.DB.Host, conf.DB.Port, conf.DB.Name)

	if err := mysqlCli.TruncateAll(); err != nil {
		log.Printf("failed to truncate all, %v", err)
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
	)

	if err != nil {
		log.Fatalf(err.Error())
	}

	log.Printf("strategy: %s\n", sConf.StrategyName)
	log.Printf("rate: %s\n", sConf.RateHistoryFile)
	log.Println("======================================")

	ctx := context.Background()
	for {
		if err := strategy.Trade(ctx); err != nil {
			log.Fatalf("error occured, %v\n", err)
			break
		}

		if !exCli.NextStep() {
			break
		}
	}
}
