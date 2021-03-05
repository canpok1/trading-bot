package usecase

import (
	"fmt"
	"os"
	"time"
	"trading-bot/pkg/domain/exchange"
	"trading-bot/pkg/domain/model"
)

// RateLogger レート保存
type RateLogger struct {
	exCli       exchange.Client
	pair        *model.CurrencyPair
	logFilePath string
}

// NewRateLogger 生成
func NewRateLogger(exCli exchange.Client, pair *model.CurrencyPair, logFilePath string) *RateLogger {
	return &RateLogger{
		exCli:       exCli,
		pair:        pair,
		logFilePath: logFilePath,
	}
}

// AppendLog レート情報を追加
func (l *RateLogger) AppendLog() error {
	file, err := os.OpenFile(l.logFilePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return err
	}
	if stat.Size() == 0 {
		fmt.Fprintf(file, "日時,買いレート,売りレート\n")
	}

	buyOrder, err := l.exCli.GetOrderRate(l.pair, model.BuySide)
	if err != nil {
		return err
	}
	sellOrder, err := l.exCli.GetOrderRate(l.pair, model.SellSide)
	if err != nil {
		return err
	}

	now := time.Now().Format(time.RFC3339)
	fmt.Fprintf(file, "%s,%.5f,%.5f\n", now, buyOrder.Rate, sellOrder.Rate)

	return nil
}
