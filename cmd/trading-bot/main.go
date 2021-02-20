package main

import (
	"context"
	"log"
	"time"
	"trading-bot/pkg/infrastructure/coincheck"
	"trading-bot/pkg/infrastructure/mysql"
	"trading-bot/pkg/usecase"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	StrategyName    string `split_words:"true" required:"true"`
	IntervalSeconds int    `split_words:"true" required:"true"`
	Exchange        Exchange
	DB              DB
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

	ec := &coincheck.Client{APIAccessKey: conf.Exchange.AccessKey, APISecretKey: conf.Exchange.SecretKey}
	rc := &mysql.Client{
		UserName: conf.DB.UserName,
		Password: conf.DB.Password,
		DBName:   conf.DB.Name,
	}
	s := usecase.MakeStrategy(usecase.StrategyType(conf.StrategyName), ec, rc)
	if s == nil {
		log.Fatalf("strategy name is unknown; name = %s", conf.StrategyName)
	}
	log.Printf("strategy: %s\n", conf.StrategyName)
	log.Printf("interval: %dsec\n", conf.IntervalSeconds)
	log.Println("======================================")

	ctx := context.Background()
	func() {
		ticker := time.NewTicker(time.Duration(conf.IntervalSeconds) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := s.Run(ctx); err != nil {
					log.Printf("failed to Run; %v\n", err)
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}
