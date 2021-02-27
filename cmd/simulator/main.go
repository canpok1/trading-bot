package main

import (
	"context"
	"flag"
	"log"
	"trading-bot/pkg/domain/model"
	"trading-bot/pkg/infrastructure/memory"
	"trading-bot/pkg/infrastructure/mysql"
	"trading-bot/pkg/usecase"
	"trading-bot/pkg/usecase/trade"

	"github.com/BurntSushi/toml"
)

func main() {
	log.Println("===== START PROGRAM ====================")
	defer log.Println("===== END PROGRAM ======================")

	f := flag.String("f", "", "config file path")
	flag.Parse()
	log.Printf("config file: %s\n", *f)

	var conf model.SimulatorConfig
	if _, err := toml.DecodeFile(*f, &conf); err != nil {
		log.Fatal(err.Error())
	}

	exCli := memory.NewExchangeMock()
	rateRepo := memory.NewRateRepository(conf.RateHistorySize)
	mysqlCli := mysql.NewClient(conf.DB.UserName, conf.DB.Password, conf.DB.Host, conf.DB.Port, conf.DB.Name)

	if err := mysqlCli.TruncateAll(); err != nil {
		log.Printf("failed to truncate all, %v", err)
		return
	}

	strategy := usecase.MakeStrategy(
		usecase.StrategyType(conf.StrategyName),
		trade.NewFacade(
			&model.CurrencyPair{
				Key:        model.CurrencyType(conf.TargetCurrency),
				Settlement: model.JPY,
			},
			exCli,
			rateRepo,
			mysqlCli,
			mysqlCli,
			mysqlCli,
		),
	)

	if strategy == nil {
		log.Fatalf("strategy name is unknown; name = %s", conf.StrategyName)
	}

	log.Printf("strategy: %s\n", conf.StrategyName)
	log.Printf("target: %s\n", conf.TargetCurrency)
	log.Println("======================================")

	ctx := context.Background()
	for {
		if err := strategy.Trade(ctx); err != nil {
			log.Printf("error occured, %v", err)
			break
		}

		if !exCli.HasNextStep() {
			break
		}

		exCli.NextStep()
	}
}
