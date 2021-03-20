package usecase

import (
	"time"
	"trading-bot/pkg/domain/exchange"
	"trading-bot/pkg/domain/model"
	"trading-bot/pkg/infrastructure/mysql"
)

// RateLogger レート保存
type RateLogger struct {
	exCli    exchange.Client
	pair     model.CurrencyPair
	mysqlCli *mysql.Client
}

// NewRateLogger 生成
func NewRateLogger(exCli exchange.Client, pair model.CurrencyPair, mysqlCli *mysql.Client) *RateLogger {
	return &RateLogger{
		exCli:    exCli,
		pair:     pair,
		mysqlCli: mysqlCli,
	}
}

// AppendLog レート情報を追加
func (l *RateLogger) AppendLog() error {
	r, err := l.exCli.GetStoreRate(&l.pair)
	if err != nil {
		return err
	}

	now := time.Now()
	if err := l.mysqlCli.AddRates(l.pair.Key, float64(r.Rate), now); err != nil {
		return err
	}

	return nil
}
