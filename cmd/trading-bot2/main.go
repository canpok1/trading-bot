package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"
	"trading-bot/pkg/domain/model"
	"trading-bot/pkg/infrastructure/coincheck"
	"trading-bot/pkg/infrastructure/memory"
	"trading-bot/pkg/infrastructure/mysql"
	"trading-bot/pkg/infrastructure/slack"
	"trading-bot/pkg/usecase/log"
	"trading-bot/pkg/usecase/trade"

	"github.com/kelseyhightower/envconfig"
	"golang.org/x/sync/errgroup"
)

const (
	location = "Asia/Tokyo"
)

var (
	rateDuration = 24 * time.Hour
	buyJpyMin    = 500.0
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

	var config BotConfig
	if err := envconfig.Process("BOT", &config); err != nil {
		logger.Error(err.Error())
		return
	}

	logger.Info("currency: %s\n", config.TargetCurrency)
	logger.Info("interval: %d sec\n", config.IntervalSeconds)
	logger.Info("======================================")

	coincheckCli := coincheck.NewClient(&logger, config.Exchange.AccessKey, config.Exchange.SecretKey)
	mysqlCli := mysql.NewClient(config.DB.UserName, config.DB.Password, config.DB.Host, config.DB.Port, config.DB.Name)
	slackCli := slack.NewClient(config.SlackURL)
	bot := NewBot(&config, coincheckCli, mysqlCli, &logger, slackCli)

	rootCtx, cancel := context.WithCancel(context.Background())
	errGroup, ctx := errgroup.WithContext(rootCtx)

	errGroup.Go(bot.Fetch(ctx))
	errGroup.Go(bot.Trade(ctx))
	errGroup.Go(bot.WatchTrade(ctx))
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
	quit := make(chan os.Signal)
	defer close(quit)
	signal.Notify(quit, os.Interrupt)
	select {
	case <-quit:
		logger.Info("terminating ...")
	case <-ctx.Done():
	}
	return nil
}

type BotConfig struct {
	// ===== 基本設定 =====
	// 購入対象コイン
	TargetCurrency string `required:"true" split_words:"true"`
	// 稼働間隔（秒）
	IntervalSeconds int `required:"true" split_words:"true"`
	// 取引所設定
	Exchange model.Exchange `required:"true" split_words:"true"`
	// DB設定
	DB model.DB `required:"true" split_words:"true"`
	// SlackのIncomingWebhookのURL
	SlackURL string `required:"true" split_words:"true"`

	// ===== エントリー判断関連 =====
	// サポートラインの判定範囲1（現在に近い方）
	SupportLinePeriod1 int `required:"true" split_words:"true"`
	// サポートラインの判定範囲2（現在から遠い方）
	SupportLinePeriod2 int `required:"true" split_words:"true"`
	// レートがどの程度下がったらナンピンするか
	AveragingDownRatePer float64 `required:"true" split_words:"true"`
	// 売最大出来高（最大を超えたら買い準備に移行）
	SellMaxVolume float64 `required:"true" split_words:"true"`
	// 買最大出来高（最大を超えたら急騰時刻を記録）
	BuyMaxVolume float64 `required:"true" split_words:"true"`
	// 出来高の監視対象の時間幅（直近何秒までの出来高を見るか？）
	VolumeCheckSeconds int `required:"true" split_words:"true"`
	// 急騰を警戒する時間
	SoaredWarningPeriodSeconds int `required:"true" split_words:"true"`

	// ===== 注文関連 =====
	// 注文用資金
	FundsRatio float64 `required:"true" split_words:"true"`
	// 注文用資金（1注文分）
	FundsRatioPerOrder float64 `required:"true" split_words:"true"`
	// 目標利益率
	TargetProfitPer float64 `required:"true" split_words:"true"`
}

func (c *BotConfig) GetTargetPair(Settlement model.CurrencyType) *model.CurrencyPair {
	return &model.CurrencyPair{
		Key:        model.CurrencyType(c.TargetCurrency),
		Settlement: Settlement,
	}
}

type ExchangeInfo struct {
	Pair *model.CurrencyPair
	// 売レート
	SellRate float64
	// 買レート
	BuyRate float64
	// 残高（JPY）
	BalanceJPY *model.Balance
	// 残高（コイン）
	BalanceCurrency *model.Balance
	// 買注文で支払った金額（JPY）
	UsedJPY float64
	// 買注文に取得したコイン量
	ObtainedCurrency float64
}

func (e *ExchangeInfo) GetTotalBalanceJPY() float64 {
	jpy := e.BalanceJPY.Amount + e.BalanceJPY.Reserved
	other := e.SellRate * (e.BalanceCurrency.Amount + e.BalanceCurrency.Reserved)
	return jpy + other
}

type Bot struct {
	Config       *BotConfig
	CoincheckCli *coincheck.Client
	MysqlCli     *mysql.Client
	Logger       *memory.Logger
	SlackCli     *slack.Client

	buyStandby bool
	soaredTime *time.Time
}

func NewBot(config *BotConfig, coincheckCli *coincheck.Client, mysqlCli *mysql.Client, logger *memory.Logger, slackCli *slack.Client) *Bot {
	return &Bot{
		Config:       config,
		CoincheckCli: coincheckCli,
		MysqlCli:     mysqlCli,
		Logger:       logger,
		SlackCli:     slackCli,
		buyStandby:   false,
		soaredTime:   nil,
	}
}

func (b *Bot) Trade(ctx context.Context) func() error {
	return func() error {
		for {
			select {
			case <-ctx.Done():
				return nil
			default:
				if err := b.trade(ctx); err != nil {
					b.Logger.Error("error occured in Trade, %v", err)
				}
			}
		}
	}
}

func (b *Bot) trade(ctx context.Context) error {
	pair := b.Config.GetTargetPair(model.JPY)

	info, err := b.getExchangeInfo(pair)
	if err != nil {
		return err
	}

	b.Logger.Debug("================================================================================================")
	b.Logger.Debug(
		"%v[sell:%.3f,buy:%.3f] Balance[%s:%.3f,%s:%.3f] Used[jpy:%.3f]",
		info.Pair, info.SellRate, info.BuyRate,
		info.BalanceJPY.Currency, info.BalanceJPY.Amount,
		info.BalanceCurrency.Currency, info.BalanceCurrency.Amount,
		info.UsedJPY,
	)
	b.Logger.Debug("================================================================================================")

	traded, err := b.tradeForBuy(info)
	if err != nil {
		return err
	}

	if traded {
		info, err = b.getExchangeInfo(pair)
		if err != nil {
			return err
		}
	}

	if err := b.tradeForSell(info); err != nil {
		return err
	}

	if err := b.wait(ctx); err != nil {
		return err
	}

	return nil
}

func (b *Bot) tradeForBuy(info *ExchangeInfo) (bool, error) {
	amount, err := b.calcBuyAmount(info)
	if err != nil {
		return false, err
	}
	if amount == 0 {
		return false, err
	}

	// 成行買 → 約定待ち
	if err := b.buyAndWaitForContract(info.Pair, amount); err != nil {
		return false, err
	}

	// 未決済注文をキャンセル
	openOrders, err := b.CoincheckCli.GetOpenOrders(info.Pair)
	if err != nil {
		return false, err
	}
	if err := b.cancel(openOrders); err != nil {
		return false, err
	}

	return true, nil
}

func (b *Bot) tradeForSell(info *ExchangeInfo) error {
	openOrders, err := b.CoincheckCli.GetOpenOrders(info.Pair)
	if err != nil {
		return err
	}

	if info.UsedJPY == 0.0 {
		b.Logger.Debug("skip sell (buy JPY:%s == 0.000)", log.Yellow("%.3f", info.UsedJPY))
		return nil
	}

	if len(openOrders) > 0 {
		b.Logger.Debug("skip sell (open order count:%s > 0)", log.Yellow("%d", len(openOrders)))
		for _, order := range openOrders {
			b.Logger.Debug("open order => [%s rate:%.3f, amount:%.3f]", order.Type, *order.Rate, order.Amount)
		}
		return nil
	}
	b.Logger.Debug("%s (open order count:%s == 0)", "should sell", log.Yellow("%d", len(openOrders)))

	// 指値売り
	newInfo, err := b.getExchangeInfo(info.Pair)
	if err != nil {
		return err
	}
	if err := b.sell(newInfo); err != nil {
		return err
	}
	return nil
}

func (b *Bot) getExchangeInfo(pair *model.CurrencyPair) (*ExchangeInfo, error) {
	sellRate, err := b.CoincheckCli.GetOrderRate(pair, model.SellSide)
	if err != nil {
		return nil, err
	}
	buyRate, err := b.CoincheckCli.GetOrderRate(pair, model.BuySide)
	if err != nil {
		return nil, err
	}

	contracts, err := b.CoincheckCli.GetContracts()
	if err != nil {
		return nil, err
	}

	balanceJPY, err := b.CoincheckCli.GetBalance(model.JPY)
	if err != nil {
		return nil, err
	}

	balanceCurrency, err := b.CoincheckCli.GetBalance(pair.Key)
	if err != nil {
		return nil, err
	}

	usedJPY, obtainedCurrency := trade.CalcAmount(pair, contracts, balanceCurrency.Reserved+balanceCurrency.Amount, 1)

	return &ExchangeInfo{
		Pair:             pair,
		SellRate:         sellRate.Rate,
		BuyRate:          buyRate.Rate,
		BalanceJPY:       balanceJPY,
		BalanceCurrency:  balanceCurrency,
		UsedJPY:          usedJPY,
		ObtainedCurrency: obtainedCurrency,
	}, nil
}

func (b *Bot) calcBuyAmount(info *ExchangeInfo) (float64, error) {
	rates, err := b.MysqlCli.GetRates(info.Pair, &rateDuration)
	if err != nil {
		return 0, err
	}
	required := b.Config.SupportLinePeriod1 + b.Config.SupportLinePeriod2
	if len(rates) < required {
		b.Logger.Debug("skip buy (rate len:%s < SupportLine required:%d)", log.Yellow("%d", len(rates)), required)
		return 0, nil
	}

	// サポートラインより下？
	supportLines, slope := trade.SupportLine(rates, b.Config.SupportLinePeriod1, b.Config.SupportLinePeriod2)
	supportLine := supportLines[len(supportLines)-1]
	supportLineCrossed := info.SellRate < supportLine
	if supportLineCrossed {
		b.Logger.Debug("%s support line crossed (sell:%s < support:%s)", log.Green("OK"), log.Yellow("%.3f", info.SellRate), log.Yellow("%.3f", supportLine))
	} else {
		b.Logger.Debug("%s support line not crossed (sell:%s >= support:%s)", log.Red("NG"), log.Yellow("%.3f", info.SellRate), log.Yellow("%.3f", supportLine))
	}

	// サポートラインは右肩上がり？
	supportLineRising := slope >= 0
	if supportLineRising {
		b.Logger.Debug("%s support line is rising (slope:%s >= 0)", log.Green("OK"), log.Yellow("%.3f", slope))
	} else {
		b.Logger.Debug("%s support line is not rising (slope:%s < 0)", log.Red("NG"), log.Yellow("%.3f", slope))
	}

	// 前注文よりレート下？
	averagingDown := false
	averagingDownLittle := false
	if info.ObtainedCurrency == 0.0 {
		b.Logger.Debug("%s can averaging down (%s: nothing)", log.Green("OK"), info.Pair.Key)
		averagingDown = true
		averagingDownLittle = true
	} else {
		orderRateAVG := info.UsedJPY / info.ObtainedCurrency
		border := orderRateAVG * b.Config.AveragingDownRatePer
		averagingDown = info.BuyRate < border
		averagingDownLittle = info.BuyRate < orderRateAVG
		if averagingDown {
			b.Logger.Debug("%s can averaging down (buyRate:%s < border:%s) (AVG:%.3f)", log.Green("OK"), log.Yellow("%.3f", info.BuyRate), log.Yellow("%.3f", border), orderRateAVG)
		} else if averagingDownLittle {
			b.Logger.Debug("%s cannot averaging down (border:%s =< buyRate:%s < AVG:%s)", log.Red("NG"), log.Yellow("%.3f", border), log.Yellow("%.3f", info.BuyRate), log.Yellow("%.3f", orderRateAVG))
		} else {
			b.Logger.Debug("%s cannot averaging down (buyRate:%s >= border:%s) (AVG:%.3f)", log.Red("NG"), log.Yellow("%.3f", info.BuyRate), log.Yellow("%.3f", border), orderRateAVG)
		}
	}

	totalJPY := info.GetTotalBalanceJPY()
	fundsJPY := totalJPY * b.Config.FundsRatio
	usedJPY := totalJPY - info.BalanceJPY.Amount

	// 追加注文に使う金額(JPY)
	newOrderJPY := info.UsedJPY
	if newOrderJPY == 0.0 {
		newOrderJPY = totalJPY * b.Config.FundsRatioPerOrder
	}
	if newOrderJPY < buyJpyMin {
		b.Logger.Debug("cannot sending buy order, jpy is too low (%.3f < min:%.3f)", newOrderJPY, buyJpyMin)
		b.buyStandby = false
		return 0, nil
	}

	// 追加注文する余裕ある？
	fundsBalanceJPY := fundsJPY - usedJPY
	canOrder := newOrderJPY <= fundsBalanceJPY
	if canOrder {
		b.Logger.Debug("%s can order (newOrderJPY:%s < fundsBalance:%s)", log.Green("OK"), log.Yellow("%.3f", newOrderJPY), log.Yellow("%.3f", fundsBalanceJPY))
	} else {
		b.Logger.Debug("%s cannot order (newOrderJPY:%s < fundsBalance:%s)", log.Red("NG"), log.Yellow("%.3f", newOrderJPY), log.Yellow("%.3f", fundsBalanceJPY))
	}

	// 急騰時期？
	priceStable := true
	if b.soaredTime == nil {
		b.Logger.Debug("%s not soared warning (soared time :nothing)", log.Green("OK"))
	} else {
		now := time.Now()
		soaredEndTime := b.soaredTime.Add(time.Duration(b.Config.SoaredWarningPeriodSeconds) * time.Second)
		priceStable = now.After(soaredEndTime)
		if priceStable {
			b.Logger.Debug(
				"%s price stable (soared:%v < end:%v < now:%v)",
				log.Green("OK"), b.soaredTime.Format(time.RFC3339), soaredEndTime.Format(time.RFC3339), now.Format(time.RFC3339))
		} else {
			b.Logger.Debug(
				"%s price not stable (soared:%v < now:%v < end:%v)",
				log.Red("NG"), b.soaredTime.Format(time.RFC3339), now.Format(time.RFC3339), soaredEndTime.Format(time.RFC3339))
		}
	}

	if !supportLineCrossed || !supportLineRising || !averagingDown || !canOrder || !priceStable {
		b.Logger.Debug("skip buy (supportLineCrossed:%v, supportLineRising:%v, averagingDown:%v, canOrder:%v, priceStable:%v)",
			supportLineCrossed, supportLineRising, averagingDown, canOrder, priceStable)

		hasPosition := info.ObtainedCurrency > 0
		newStandby := hasPosition && averagingDownLittle && canOrder
		if !b.buyStandby && newStandby {
			b.Logger.Debug("%s (hasPosition:%v,averagingDownLittle:%v,canOrder:%v)", log.Green("set buyStandby"), hasPosition, averagingDownLittle, canOrder)
		} else if b.buyStandby && !newStandby {
			b.Logger.Debug("%s (hasPosition:%v,averagingDownLittle:%v,canOrder:%v)", log.Green("release buyStandby"), hasPosition, averagingDownLittle, canOrder)
		}
		b.buyStandby = newStandby

		return 0, nil
	}
	b.Logger.Debug("%s (supportLineCrossed:%v, supportLineRising:%v, averagingDown:%v, canOrder:%v, priceStable:%v)",
		"should buy", supportLineCrossed, supportLineRising, averagingDown, canOrder, priceStable)

	return newOrderJPY, nil
}

func (b *Bot) buyAndWaitForContract(pair *model.CurrencyPair, amount float64) error {
	b.Logger.Debug("======================================")
	defer b.Logger.Debug("======================================")

	b.Logger.Debug("sending buy order ...")
	order, err := b.CoincheckCli.PostOrder(&model.NewOrder{
		Type:            model.MarketBuy,
		Pair:            *pair,
		MarketBuyAmount: &amount,
	})
	if err != nil {
		return err
	}
	b.Logger.Debug(log.Green("completed!!![id:%d,%.3f]", order.ID, amount))

	message := slack.TextMessage{
		Text: fmt.Sprintf(
			"buy completed!!! `%s amount:%.3f`",
			order.Pair.String(),
			amount,
		),
	}
	if err := b.SlackCli.PostMessage(message); err != nil {
		b.Logger.Error("%v", err)
	}

	b.Logger.Debug("wait for contract[id:%d]...", order.ID)
	// 約定を待つ
	for {
		cc, err := b.CoincheckCli.GetContracts()
		if err != nil {
			return err
		}
		for _, c := range cc {
			if c.OrderID == order.ID {
				b.Logger.Debug(log.Green("contracted!!![id:%d]", order.ID))
				b.buyStandby = false
				return nil
			}
		}
		time.Sleep(1 * time.Second)
	}
}

func (b *Bot) cancel(orders []model.Order) error {
	for _, o := range orders {
		if err := b.CoincheckCli.DeleteOrder(o.ID); err != nil {
			return err
		}
	}

	for _, o := range orders {
		for {
			canceled, err := b.CoincheckCli.GetCancelStatus(o.ID)
			if err != nil {
				return err
			}
			if canceled {
				break
			}
			time.Sleep(1 * time.Second)
		}
	}

	return nil
}

func (b *Bot) sell(info *ExchangeInfo) error {
	b.Logger.Debug("======================================")
	defer b.Logger.Debug("======================================")

	profit := info.UsedJPY * b.Config.TargetProfitPer
	rate := (info.UsedJPY + profit) / info.ObtainedCurrency

	b.Logger.Debug("sending sell order ...")
	order, err := b.CoincheckCli.PostOrder(&model.NewOrder{
		Type:   model.Sell,
		Pair:   *info.Pair,
		Amount: &info.ObtainedCurrency,
		Rate:   &rate,
	})
	if err != nil {
		return fmt.Errorf(
			"failed to send sell order; buyJPY:%.3f, rate:%.3f, amount:%.3f, error :%w",
			info.UsedJPY,
			rate,
			info.ObtainedCurrency,
			err)
	}
	b.Logger.Debug(log.Green("completed!!![id:%d,%.3f,%.3f]", order.ID, *order.Rate, order.Amount))
	message := slack.TextMessage{
		Text: fmt.Sprintf(
			"sell completed!!! `%s %.3f %.3f`",
			order.Pair.String(), *order.Rate, order.Amount,
		),
	}
	if err := b.SlackCli.PostMessage(message); err != nil {
		b.Logger.Error("%v", err)
	}
	return nil
}

func (b *Bot) WatchTrade(ctx context.Context) func() error {
	return func() error {
		// 取引履歴の監視
		pair := b.Config.GetTargetPair(model.JPY)
		for {
			select {
			case <-ctx.Done():
				return nil
			default:
				if err := b.CoincheckCli.SubscribeTradeHistory(ctx, pair, b.receiveTrade); err != nil {
					if !strings.Contains(err.Error(), "i/o timeout") {
						b.Logger.Error("error occured in WatchTrade, %v", err)
					}
				}
			}
		}
	}
}

func (b *Bot) receiveTrade(side model.OrderSide, rate float64) error {
	d := time.Duration(b.Config.VolumeCheckSeconds) * time.Second
	v, err := b.CoincheckCli.GetVolumes(b.Config.GetTargetPair(model.JPY), side, d)
	if err != nil {
		return err
	}

	if side == model.BuySide {
		if v > b.Config.BuyMaxVolume {
			b.Logger.Debug("[receive] %s (buy volume:%.3f > max:%.3f)", log.Red("record soaredTime"), v, b.Config.BuyMaxVolume)
			now := time.Now()
			b.soaredTime = &now
		} else {
			b.Logger.Debug("[receive] skip record soaredTime (buy volume:%.3f <= max:%.3f)", v, b.Config.BuyMaxVolume)
		}
		return nil
	}

	if v > b.Config.SellMaxVolume {
		b.Logger.Debug("[receive] %s (sell volume:%.3f > max:%.3f)", log.Green("set buyStandby"), v, b.Config.SellMaxVolume)
		b.buyStandby = true
		// 警戒期間を残り5分に設定
		soaredTime := time.Now().Add(-time.Duration(b.Config.SoaredWarningPeriodSeconds)*time.Second + 5*time.Minute)
		b.soaredTime = &soaredTime
	} else {
		b.Logger.Debug("[receive] skip set buyStandby (sell volume:%.3f <= max:%.3f)", v, b.Config.SellMaxVolume)
	}

	return nil
}

func (b *Bot) wait(ctx context.Context) error {
	b.Logger.Debug("wait ... (%d sec)", b.Config.IntervalSeconds)

	waitCount := 0
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			if b.buyStandby && waitCount >= 1 {
				b.Logger.Debug("stop wait (buy standby)")
				return nil
			}
			if waitCount >= b.Config.IntervalSeconds {
				return nil
			}
			time.Sleep(1 * time.Second)
			waitCount++
		}
	}
}

func (b *Bot) Fetch(ctx context.Context) func() error {
	return func() error {
		// レートの定期保存
		ticker := time.NewTicker(time.Duration(b.Config.IntervalSeconds) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := b.fetch(ctx); err != nil {
					b.Logger.Error("failed to fetch, error: %w", err)
				}
			case <-ctx.Done():
				return nil
			}
		}
	}
}

func (b *Bot) fetch(ctx context.Context) error {
	pair := b.Config.GetTargetPair(model.JPY)

	sellRate, err := b.CoincheckCli.GetOrderRate(pair, model.SellSide)
	if err != nil {
		return err
	}

	if err := b.MysqlCli.AddRates(pair, sellRate.Rate, time.Now()); err != nil {
		return err
	}

	return nil
}
