package main

import (
	"context"
	"os"
	"os/signal"
	"time"
	"trading-bot/pkg/domain/model"
	"trading-bot/pkg/infrastructure/coincheck"
	"trading-bot/pkg/infrastructure/memory"
	"trading-bot/pkg/infrastructure/mysql"

	"github.com/kelseyhightower/envconfig"
	"golang.org/x/sync/errgroup"
)

const (
	location = "Asia/Tokyo"
)

func init() {
	loc, err := time.LoadLocation(location)
	if err != nil {
		loc = time.FixedZone(location, 9*60*60)
	}
	time.Local = loc
}

func main() {
	logger := memory.Logger{Level: memory.Debug}

	logger.Info("===== START PROGRAM ====================")
	defer logger.Info("===== END PROGRAM ======================")

	var config Config
	if err := envconfig.Process("", &config); err != nil {
		logger.Error(err.Error())
		return
	}

	pairs := []*model.CurrencyPair{}
	for _, s := range config.TargetPairs {
		pair, err := model.ParseToCurrencyPair(s)
		if err != nil {
			logger.Error(err.Error())
			return
		}
		pairs = append(pairs, pair)
	}

	logger.Info("pairs: %v\n", config.TargetPairs)
	logger.Info("fetch interval: %d sec\n", config.IntervalSeconds)
	logger.Info("clean interval: %d sec\n", config.CleanIntervalSeconds)
	logger.Info("======================================")

	coincheckCli := coincheck.NewPublicClient(&logger)
	mysqlCli := mysql.NewClient(config.DB.UserName, config.DB.Password, config.DB.Host, config.DB.Port, config.DB.Name)
	fetcher := NewFetcher(&config, coincheckCli, mysqlCli, &logger)
	cleaner := NewCleaner(&config, mysqlCli, &logger)

	rootCtx, cancel := context.WithCancel(context.Background())
	errGroup, ctx := errgroup.WithContext(rootCtx)

	for _, pair := range pairs {
		errGroup.Go(fetcher.Fetch(ctx, pair))
		errGroup.Go(cleaner.Clean(ctx, pair))
	}
	errGroup.Go(func() error {
		defer cancel()
		return watchSignal(ctx, &logger)
	})

	if err := errGroup.Wait(); err != nil {
		logger.Error("error occured, %v", err)
	}
}

func watchSignal(ctx context.Context, logger *memory.Logger) error {
	// OSのシグナル監視
	quit := make(chan os.Signal, 1)
	defer close(quit)
	signal.Notify(quit, os.Interrupt)
	select {
	case <-quit:
		logger.Info("terminating ...")
	case <-ctx.Done():
	}
	return nil
}

type Config struct {
	// 対象コインペア
	TargetPairs []string `required:"true" split_words:"true"`
	// 稼働間隔（秒）
	IntervalSeconds int `required:"true" split_words:"true"`
	// 削除の稼働間隔（秒）
	CleanIntervalSeconds int `required:"true" split_words:"true"`
	// 情報の有効期限（秒）
	ExpireSeconds int `required:"true" split_words:"true"`

	// DB設定
	DB model.DB `required:"true" split_words:"true"`
}

type Fetcher struct {
	Config       *Config
	CoincheckCli *coincheck.Client
	MysqlCli     *mysql.Client
	Logger       *memory.Logger
}

func NewFetcher(config *Config, coincheckCli *coincheck.Client, mysqlCli *mysql.Client, logger *memory.Logger) *Fetcher {
	return &Fetcher{
		Config:       config,
		CoincheckCli: coincheckCli,
		MysqlCli:     mysqlCli,
		Logger:       logger,
	}
}

func (f *Fetcher) Fetch(ctx context.Context, pair *model.CurrencyPair) func() error {
	return func() error {
		// レートの定期保存
		ticker := time.NewTicker(time.Duration(f.Config.IntervalSeconds) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := f.fetch(ctx, pair); err != nil {
					f.Logger.Error("failed to fetch, error: %w", err)
				}
			case <-ctx.Done():
				return nil
			}
		}
	}
}

func (f *Fetcher) fetch(ctx context.Context, pair *model.CurrencyPair) error {
	storeRate, err := f.CoincheckCli.GetStoreRate(pair)
	if err != nil {
		return err
	}
	sellRate, err := f.CoincheckCli.GetOrderRate(pair, model.SellSide)
	if err != nil {
		return err
	}
	buyRate, err := f.CoincheckCli.GetOrderRate(pair, model.BuySide)
	if err != nil {
		return err
	}
	trades, err := f.CoincheckCli.GetTrades(pair, 100)
	if err != nil {
		return err
	}

	border := time.Now().Add(time.Duration(-f.Config.IntervalSeconds) * time.Second)
	volumesSell := 0.0
	volumesBuy := 0.0
	for _, t := range trades {
		if t.CreatedAt.Before(border) {
			continue
		}
		if t.Side == model.SellSide {
			volumesSell += t.Amount
		} else {
			volumesBuy += t.Amount
		}
	}

	m := mysql.Market{
		Pair:         pair.String(),
		StoreRateAVG: storeRate.Rate,
		ExRateSell:   sellRate.Rate,
		ExRateBuy:    buyRate.Rate,
		ExVolumeSell: volumesSell,
		ExVolumeBuy:  volumesBuy,
		RecordedAt:   time.Now(),
	}
	f.Logger.Debug("%+v", m)
	if err := f.MysqlCli.AddMarket(&m); err != nil {
		return err
	}

	return nil
}

type Cleaner struct {
	Config   *Config
	MysqlCli *mysql.Client
	Logger   *memory.Logger
}

func NewCleaner(config *Config, mysqlCli *mysql.Client, logger *memory.Logger) *Cleaner {
	return &Cleaner{
		Config:   config,
		MysqlCli: mysqlCli,
		Logger:   logger,
	}
}

func (c *Cleaner) Clean(ctx context.Context, pair *model.CurrencyPair) func() error {
	return func() error {
		// 古い情報の削除
		ticker := time.NewTicker(time.Duration(c.Config.CleanIntervalSeconds) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := c.clean(ctx, pair); err != nil {
					c.Logger.Error("failed to clean, error: %w", err)
				}
			case <-ctx.Done():
				return nil
			}
		}
	}
}

func (c *Cleaner) clean(ctx context.Context, pair *model.CurrencyPair) error {
	err := c.MysqlCli.DeleteMarkets(pair, time.Duration(c.Config.ExpireSeconds)*time.Second)
	c.Logger.Debug("cleaned records")

	return err
}
