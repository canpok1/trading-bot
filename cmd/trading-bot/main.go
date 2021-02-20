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
	Exchange Exchange
	DB       DB
}

type Exchange struct {
	AccessKey string `split_words:"true"`
	SecretKey string `split_words:"true"`
}

type DB struct {
	Host     string
	Port     int
	Name     string
	UserName string `split_words:"true"`
	Password string
}

func main() {
	log.Println("start program")

	var conf Config
	if err := envconfig.Process("BOT", &conf); err != nil {
		log.Fatal(err.Error())
	}
	log.Printf("conf = %#v", conf)

	ec := &coincheck.Client{APIAccessKey: conf.Exchange.AccessKey, APISecretKey: conf.Exchange.SecretKey}
	rc := &mysql.Client{
		UserName: conf.DB.UserName,
		Password: conf.DB.Password,
		DBName:   conf.DB.Name,
	}

	s := usecase.MakeStrategy(usecase.Sample, ec, rc)
	if err := s.Run(context.Background(), 10*time.Second); err != nil {
		log.Printf("%v\n", err)
	}

	log.Println("end program")
}
