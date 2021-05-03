package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"
	"trading-bot/pkg/domain"
	"trading-bot/pkg/domain/model"
	"trading-bot/pkg/infrastructure/coincheck"
	"trading-bot/pkg/infrastructure/memory"
	"trading-bot/pkg/infrastructure/mysql"
	"trading-bot/pkg/infrastructure/slack"
	"trading-bot/pkg/usecase/trade"

	"github.com/kelseyhightower/envconfig"
	"golang.org/x/sync/errgroup"
)

const (
	location  = "Asia/Tokyo"
	volumeKey = "2006-01-02T15:04:00"
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
	logger.Info("demo mode: %v\n", config.DemoMode)
	logger.Info("======================================")

	coincheckCli := coincheck.NewClient(&logger, config.Exchange.AccessKey, config.Exchange.SecretKey)
	mysqlCli := mysql.NewClient(config.DB.UserName, config.DB.Password, config.DB.Host, config.DB.Port, config.DB.Name)
	slackCli := slack.NewClient(config.SlackURL)
	bot := NewBot(&config, coincheckCli, mysqlCli, &logger, slackCli)

	rootCtx, cancel := context.WithCancel(context.Background())
	errGroup, ctx := errgroup.WithContext(rootCtx)

	errGroup.Go(bot.Fetch(ctx))
	errGroup.Go(bot.Trade(ctx))
	//errGroup.Go(bot.WatchTrade(ctx))
	errGroup.Go(bot.ReceiveTradeHandler(ctx))
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
	// デモモード（注文やSlack通知を送信しない）
	DemoMode bool `required:"true" split_words:"true"`

	// ===== エントリー判断関連 =====
	// サポートライン/レジスタンスラインの判定範囲
	TrendLinePeriod int `required:"true" split_words:"true"`
	// サポートライン/レジスタンスラインのオフセット（現在からどれくらい前を見るか）
	TrendLineOffset int `required:"true" split_words:"true"`

	// エントリー判断領域の幅の割合
	EntryAreaWidth float64 `required:"true" split_words:"true"`

	// ブレークアウトしたと判断する上げ幅
	BreakoutRatio float64 `required:"true" split_words:"true"`
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
	// 最短購入間隔
	BuyIntervalSeconds int `required:"true" split_words:"true"`

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
	// 通貨ペア
	Pair *model.CurrencyPair
	// 売レート
	SellRate float64
	// 買レート
	BuyRate float64
	// 残高（JPY）
	BalanceJPY *model.Balance
	// 残高（コイン）
	BalanceCurrency *model.Balance
	// 前回の買注文の約定レート
	BuyOrderContractRate float64
}

// CalcTotalBalanceJPY 現レートにおける合計残高（JPY換算）
func (e *ExchangeInfo) CalcTotalBalanceJPY() float64 {
	jpy := e.BalanceJPY.Amount + e.BalanceJPY.Reserved
	other := e.SellRate * (e.BalanceCurrency.Amount + e.BalanceCurrency.Reserved)
	return jpy + other
}

// HasPosition ポジションを持っているか？
func (e *ExchangeInfo) HasPosition() bool {
	return e.BalanceCurrency.Total()*e.SellRate >= 1
}

type Bot struct {
	Config       *BotConfig
	CoincheckCli *coincheck.Client
	MysqlCli     *mysql.Client
	Logger       *memory.Logger
	SlackCli     *slack.Client

	buyStandby  bool
	skipEndTime *time.Time
	botStatuses []mysql.BotStatus

	sellVolumeCache sync.Map
	buyVolumeCache  sync.Map
	sellVolumeChan  chan *coincheck.TradeHistory
	buyVolumeChan   chan *coincheck.TradeHistory
}

func NewBot(config *BotConfig, coincheckCli *coincheck.Client, mysqlCli *mysql.Client, logger *memory.Logger, slackCli *slack.Client) *Bot {
	return &Bot{
		Config:          config,
		CoincheckCli:    coincheckCli,
		MysqlCli:        mysqlCli,
		Logger:          logger,
		SlackCli:        slackCli,
		buyStandby:      false,
		skipEndTime:     nil,
		botStatuses:     []mysql.BotStatus{},
		sellVolumeCache: sync.Map{},
		buyVolumeCache:  sync.Map{},
		sellVolumeChan:  make(chan *coincheck.TradeHistory),
		buyVolumeChan:   make(chan *coincheck.TradeHistory),
	}
}

func (b *Bot) Trade(ctx context.Context) func() error {
	return func() error {
		for {
			select {
			case <-ctx.Done():
				return nil
			default:
				b.botStatuses = []mysql.BotStatus{}

				err := b.trade(ctx)
				if err != nil {
					b.Logger.Error("error occured in trade, %v", err)
				}

				if err := b.MysqlCli.UpsertBotStatuses(b.botStatuses); err != nil {
					b.Logger.Error("error occured in upsertBotInfos, %v", err)
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
		"%v[sell:%.3f,buy:%.3f] Balance[%s:%.3f,%s:%.3f] Total[jpy:%.3f]",
		info.Pair, info.SellRate, info.BuyRate,
		info.BalanceJPY.Currency, info.BalanceJPY.Amount,
		info.BalanceCurrency.Currency, info.BalanceCurrency.Amount,
		info.CalcTotalBalanceJPY(),
	)
	b.Logger.Debug("================================================================================================")

	if !info.HasPosition() {
		if err := b.MysqlCli.UpsertAccountInfo(mysql.AccountInfoTypeTotalJPY, info.CalcTotalBalanceJPY()); err != nil {
			return err
		}
	}

	traded, shouldLosscut, err := b.tradeForBuy(info)
	if err != nil {
		return err
	}

	if traded {
		info, err = b.getExchangeInfo(pair)
		if err != nil {
			return err
		}
	}

	if err := b.tradeForSell(info, shouldLosscut); err != nil {
		return err
	}

	if err := b.wait(ctx); err != nil {
		return err
	}

	return nil
}

func (b *Bot) tradeForBuy(info *ExchangeInfo) (bool, bool, error) {
	amount, shouldLosscut, err := b.calcBuyAmount(info)
	if err != nil {
		return false, shouldLosscut, err
	}
	if !shouldLosscut {
		if amount == 0 {
			return false, shouldLosscut, err
		}

		// 成行買 → 約定待ち
		if err := b.buyAndWaitForContract(info.Pair, amount); err != nil {
			return false, shouldLosscut, err
		}
	}

	// 未決済注文をキャンセル
	openOrders, err := b.CoincheckCli.GetOpenOrders(info.Pair)
	if err != nil {
		return false, shouldLosscut, err
	}
	if err := b.cancel(openOrders); err != nil {
		return false, shouldLosscut, err
	}

	return true, shouldLosscut, nil
}

func (b *Bot) tradeForSell(info *ExchangeInfo, shouldLosscut bool) error {
	botStatus := mysql.BotStatus{
		Type: "sell_rate", Value: -1, Memo: "約定待ちの売注文レート",
	}

	if !info.HasPosition() {
		b.Logger.Debug(
			"skip sell (no position, %s:%s)",
			info.Pair.Key, domain.Yellow("%.3f", info.BalanceCurrency.Total()))
		b.botStatuses = append(b.botStatuses, botStatus)
		return nil
	}

	openOrders, err := b.CoincheckCli.GetOpenOrders(info.Pair)
	if err != nil {
		return err
	}

	if len(openOrders) > 0 {
		b.Logger.Debug("skip sell (open order count:%s > 0)", domain.Yellow("%d", len(openOrders)))

		var lastOrderAt *time.Time
		for _, order := range openOrders {
			if lastOrderAt == nil || lastOrderAt.Before(order.OrderedAt) {
				lastOrderAt = &order.OrderedAt
			}
			b.Logger.Debug("open order => [%s rate:%.3f, amount:%.3f]", order.Type, *order.Rate, order.Amount)
			if botStatus.Value < 0 || botStatus.Value > *order.Rate {
				botStatus.Value = *order.Rate
			}
		}
		b.Logger.Debug("lastOrderAt[%s]", lastOrderAt.Format(time.RFC3339))

		// 一定時間約定できてないなら成行売り
		border := time.Now().Add(-12 * time.Hour)
		if lastOrderAt.Before(border) {
			b.Logger.Debug("%s (lastOrderAt[%s] < border[%s])", domain.Red("should market sell"), lastOrderAt.Format(time.RFC3339), border.Format(time.RFC3339))
			if err := b.cancel(openOrders); err != nil {
				return err
			}
			// 成行売り
			if err := b.marketSellAndWaitForContract(info.Pair, domain.Round(info.BalanceCurrency.Amount)); err != nil {
				return err
			}
			botStatus.Value = -1
		}

		b.botStatuses = append(b.botStatuses, botStatus)

		return nil
	}
	b.Logger.Debug("%s (open order count:%s == 0)", "should sell", domain.Yellow("%d", len(openOrders)))

	newInfo, err := b.getExchangeInfo(info.Pair)
	if err != nil {
		return err
	}

	// 指値売り
	rate, amount, err := b.calcSellRateAndAmount(newInfo, shouldLosscut)
	if err != nil {
		return err
	}
	if amount == 0 {
		botStatus.Value = -1
		b.botStatuses = append(b.botStatuses, botStatus)
		return nil
	}
	if err := b.sell(newInfo, rate, amount); err != nil {
		return err
	}

	botStatus.Value = rate
	b.botStatuses = append(b.botStatuses, botStatus)

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

	balanceJPY, err := b.CoincheckCli.GetBalance(model.JPY)
	if err != nil {
		return nil, err
	}

	balanceCurrency, err := b.CoincheckCli.GetBalance(pair.Key)
	if err != nil {
		return nil, err
	}

	buyOrderContractRate := 0.0
	contracts, err := b.CoincheckCli.GetContracts()
	if err != nil {
		return nil, err
	}
	for _, c := range contracts {
		if c.Side == model.BuySide && c.DecreaseCurrency == pair.Settlement && c.IncreaseCurrency == pair.Key {
			buyOrderContractRate = c.Rate
			break
		}
	}

	return &ExchangeInfo{
		Pair:                 pair,
		SellRate:             sellRate.Rate,
		BuyRate:              buyRate.Rate,
		BalanceJPY:           balanceJPY,
		BalanceCurrency:      balanceCurrency,
		BuyOrderContractRate: buyOrderContractRate,
	}, nil
}

func (b *Bot) calcBuyAmount(info *ExchangeInfo) (float64, bool, error) {
	shouldLosscut := false

	rates, err := b.MysqlCli.GetRates(info.Pair, &rateDuration)
	if err != nil {
		return 0, shouldLosscut, err
	}

	required := b.Config.TrendLinePeriod + b.Config.TrendLineOffset
	if len(rates) < required {
		b.Logger.Debug("skip buy (rate len:%s < SupportLine required:%d)", domain.Yellow("%d", len(rates)), required)
		b.buyStandby = false
		return 0, shouldLosscut, nil
	}

	// レート上昇してる？
	var isRising bool
	{
		beforeRate := rates[len(rates)-2]
		isRising = info.SellRate > beforeRate
		if isRising {
			b.Logger.Debug(
				"%s rate is rising (now:%s > before:%s)",
				domain.Green("OK"), domain.Yellow("%.3f", info.SellRate), domain.Yellow("%.3f", beforeRate))
		} else {
			b.Logger.Debug(
				"%s rate is not rising (now:%s <= before:%s)",
				domain.Red("NG"), domain.Yellow("%.3f", info.SellRate), domain.Yellow("%.3f", beforeRate))
		}
	}

	// 前回のレートがエントリーエリア（レジスタンスライン近く）？
	var isBreakout bool
	{
		l := len(rates)
		slope, intercept := trade.ResistanceLine2(rates, l-b.Config.TrendLinePeriod-b.Config.TrendLineOffset, l-b.Config.TrendLineOffset)
		resistanceLines := trade.MakeLine(slope, intercept, len(rates))
		resistanceLine := resistanceLines[len(resistanceLines)-1]
		width := resistanceLine * b.Config.EntryAreaWidth
		upper := resistanceLine + width
		lower := resistanceLine

		diff := info.SellRate - rates[len(rates)-1]
		diffBorder := info.SellRate * b.Config.BreakoutRatio

		isUpperEntryArea := (lower < info.SellRate && info.SellRate < resistanceLine+width)
		isBreakout = isUpperEntryArea && diff > diffBorder
		if isUpperEntryArea {
			if isBreakout {
				b.Logger.Debug(
					"%s breakout (diff:%s > border:%s)(lower:%s < sellRate:%s < upper:%s)(width=%.3f * %.3f)",
					domain.Green("OK"),
					domain.Yellow("%.3f", diff), domain.Yellow("%.3f", diffBorder),
					domain.Yellow("%.3f", lower), domain.Yellow("%.3f", info.SellRate), domain.Yellow("%.3f", upper),
					resistanceLine, b.Config.EntryAreaWidth,
				)
			} else {
				b.Logger.Debug(
					"%s not breakout (diff:%s <= border:%s)(lower:%s < sellRate:%s < upper:%s)(width=%.3f * %.3f)",
					domain.Red("NG"),
					domain.Yellow("%.3f", diff), domain.Yellow("%.3f", diffBorder),
					domain.Yellow("%.3f", lower), domain.Yellow("%.3f", info.SellRate), domain.Yellow("%.3f", upper),
					resistanceLine, b.Config.EntryAreaWidth,
				)
			}
		} else {
			if info.SellRate <= resistanceLine {
				b.Logger.Debug(
					"%s sellRate is not in upper entry area (sellRate:%s <= lower:%s)(width=%.3f * %.3f)",
					domain.Red("NG"), domain.Yellow("%.3f", info.SellRate), domain.Yellow("%.3f", lower),
					resistanceLine, b.Config.EntryAreaWidth,
				)
			} else {
				b.Logger.Debug(
					"%s sellRate is not in upper entry area (sellRate:%s >= upper:%s)(width=%.3f * %.3f)",
					domain.Red("NG"), domain.Yellow("%.3f", info.SellRate), domain.Yellow("%.3f", upper),
					resistanceLine, b.Config.EntryAreaWidth,
				)
			}
		}
		b.botStatuses = append(b.botStatuses, mysql.BotStatus{Type: "resistance_line_value", Value: resistanceLine, Memo: "レジスタンスラインの現在値"})
		b.botStatuses = append(b.botStatuses, mysql.BotStatus{Type: "resistance_line_slope", Value: slope, Memo: "レジスタンスラインの傾き"})
	}

	// 前回のレートがエントリーエリア（サポートライン近く）？
	var isLowerEntryArea bool
	{
		l := len(rates)
		slope, intercept := trade.SupportLine2(rates, l-b.Config.TrendLinePeriod-b.Config.TrendLineOffset, l-b.Config.TrendLineOffset)
		supportLines := trade.MakeLine(slope, intercept, len(rates))
		supportLine := supportLines[len(supportLines)-1]
		width := supportLine * b.Config.EntryAreaWidth
		upper := supportLine + width
		lower := supportLine - width
		isLowerEntryArea = (lower < info.SellRate && info.SellRate < supportLine+width)
		if isLowerEntryArea {
			b.Logger.Debug(
				"%s sellRate is in lower entry area (lower:%s < sellRate:%s < upper:%s)(width=%.3f * %.3f)",
				domain.Green("OK"), domain.Yellow("%.3f", lower), domain.Yellow("%.3f", info.SellRate), domain.Yellow("%.3f", upper),
				supportLine, b.Config.EntryAreaWidth,
			)
		} else {
			if info.SellRate <= supportLine {
				b.Logger.Debug(
					"%s sellRate is not in lower entry area (sellRate:%s <= lower:%s)(width=%.3f * %.3f)",
					domain.Red("NG"), domain.Yellow("%.3f", info.SellRate), domain.Yellow("%.3f", lower),
					supportLine, b.Config.EntryAreaWidth,
				)
			} else {
				b.Logger.Debug(
					"%s sellRate is not in lower entry area (sellRate:%s >= upper:%s)(width=%.3f * %.3f)",
					domain.Red("NG"), domain.Yellow("%.3f", info.SellRate), domain.Yellow("%.3f", upper),
					supportLine, b.Config.EntryAreaWidth,
				)
			}
		}
		b.botStatuses = append(b.botStatuses, mysql.BotStatus{Type: "support_line_value", Value: supportLine, Memo: "サポートラインの現在値"})
		b.botStatuses = append(b.botStatuses, mysql.BotStatus{Type: "support_line_slope", Value: slope, Memo: "サポートラインの傾き"})
	}

	entrySignal := (isLowerEntryArea || isBreakout) && isRising
	if entrySignal {
		b.Logger.Debug("%s (LowerEntryArea:%v, Breakout:%v, isRising:%v)",
			domain.Green("entry signal"), isLowerEntryArea, isBreakout, isRising)
	} else {
		b.Logger.Debug("%s (LowerEntryArea:%v, Breakout:%v, isRising:%v)",
			domain.Red("no entry signal"), isLowerEntryArea, isBreakout, isRising)
	}

	// 前注文よりレート下？
	averagingDown := false
	averagingDownLittle := false
	{
		if !info.HasPosition() {
			b.Logger.Debug(
				"%s can averaging down (%s:%.3f is very few)",
				domain.Green("OK"), info.Pair.Key, info.BalanceCurrency.Total())
			averagingDown = true
			averagingDownLittle = true
		} else {
			border := info.BuyOrderContractRate * b.Config.AveragingDownRatePer
			averagingDown = info.BuyRate < border
			averagingDownLittle = info.BuyRate < info.BuyOrderContractRate
			if averagingDown {
				b.Logger.Debug("%s can averaging down (buyRate:%s < border:%s) (min:%.3f)", domain.Green("OK"), domain.Yellow("%.3f", info.BuyRate), domain.Yellow("%.3f", border), info.BuyOrderContractRate)
			} else if averagingDownLittle {
				b.Logger.Debug("%s cannot averaging down (border:%s =< buyRate:%s < min:%s)", domain.Red("NG"), domain.Yellow("%.3f", border), domain.Yellow("%.3f", info.BuyRate), domain.Yellow("%.3f", info.BuyOrderContractRate))
			} else {
				b.Logger.Debug("%s cannot averaging down (buyRate:%s >= border:%s) (min:%.3f)", domain.Red("NG"), domain.Yellow("%.3f", info.BuyRate), domain.Yellow("%.3f", border), info.BuyOrderContractRate)
			}
		}
	}

	// 追加注文に使う金額(JPY)
	newOrderJPY := domain.Round(info.BalanceCurrency.Reserved * info.BuyRate)
	if newOrderJPY == 0.0 {
		newOrderJPY = domain.Round(info.CalcTotalBalanceJPY() * b.Config.FundsRatioPerOrder)
	}
	if newOrderJPY < buyJpyMin {
		b.Logger.Debug("%s cannot sending buy order, jpy is too low (%.3f < min:%.3f)", domain.Red("NG"), newOrderJPY, buyJpyMin)
		b.buyStandby = false
		return 0, shouldLosscut, nil
	}

	// 追加注文する余裕ある？
	fundsTotalJPY := info.CalcTotalBalanceJPY() * b.Config.FundsRatio
	fundsBalanceJPY := fundsTotalJPY - info.BalanceCurrency.Total()*info.SellRate
	canOrder := newOrderJPY <= fundsBalanceJPY
	if canOrder {
		b.Logger.Debug("%s can order (newOrderJPY:%s < fundsBalance:%s)", domain.Green("OK"), domain.Yellow("%.3f", newOrderJPY), domain.Yellow("%.3f", fundsBalanceJPY))
	} else {
		b.Logger.Debug("%s cannot order (newOrderJPY:%s < fundsBalance:%s)", domain.Red("NG"), domain.Yellow("%.3f", newOrderJPY), domain.Yellow("%.3f", fundsBalanceJPY))
	}

	// 取引可能時間
	tradePeriod := true
	{
		if b.skipEndTime == nil {
			b.Logger.Debug("%s now is tradePeriod (skip end :nothing)", domain.Green("OK"))
		} else {
			now := time.Now()
			tradePeriod = now.After(*b.skipEndTime)
			if tradePeriod {
				b.Logger.Debug(
					"%s now is tradePeriod (now:%v > skip end:%v)",
					domain.Green("OK"), now.Format(time.RFC3339), b.skipEndTime.Format(time.RFC3339))
			} else {
				b.Logger.Debug(
					"%s now is not tradePeriod (now:%v <= skip end:%v)",
					domain.Red("NG"), now.Format(time.RFC3339), b.skipEndTime.Format(time.RFC3339))
			}
		}
	}

	if !entrySignal || !isRising || !averagingDown || !canOrder || !tradePeriod {
		b.Logger.Debug("skip buy (entrySignal:%v, averagingDown:%v, canOrder:%v, tradePeriod:%v)",
			entrySignal, averagingDown, canOrder, tradePeriod)

		hasPosition := info.HasPosition()
		newStandby := hasPosition && averagingDownLittle && canOrder
		if !b.buyStandby && newStandby {
			b.Logger.Debug("%s (hasPosition:%v,averagingDownLittle:%v,canOrder:%v)", domain.Green("set buyStandby"), hasPosition, averagingDownLittle, canOrder)
		} else if b.buyStandby && !newStandby {
			b.Logger.Debug("%s (hasPosition:%v,averagingDownLittle:%v,canOrder:%v)", domain.Green("release buyStandby"), hasPosition, averagingDownLittle, canOrder)
		}
		b.buyStandby = newStandby

		if averagingDown && !canOrder {
			b.Logger.Debug("%s (averagingDown:%v,canOrder:%v)", domain.Red("should losscut"), averagingDown, canOrder)
			shouldLosscut = true
		}

		return 0, shouldLosscut, nil
	}
	b.Logger.Debug("%s (entrySignal:%v, averagingDown:%v, canOrder:%v, tradePeriod:%v)",
		"should buy", entrySignal, averagingDown, canOrder, tradePeriod)

	return newOrderJPY, shouldLosscut, nil
}

func (b *Bot) buyAndWaitForContract(pair *model.CurrencyPair, amount float64) error {
	b.Logger.Debug("======================================")
	defer b.Logger.Debug("======================================")

	if b.Config.DemoMode {
		b.Logger.Debug(
			"%s buy completed!!! (rate:%.3f, amount:%.3f)",
			domain.Cyan("[DEMO]"), amount,
		)
		return nil
	}

	b.Logger.Debug("sending market buy order ...")
	order, err := b.CoincheckCli.PostOrder(&model.NewOrder{
		Type:            model.MarketBuy,
		Pair:            *pair,
		MarketBuyAmount: &amount,
	})
	if err != nil {
		return err
	}
	b.Logger.Debug(domain.Green("completed!!![id:%d,%.3f]", order.ID, amount))

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
	if err := b.MysqlCli.AddEvent(&mysql.Event{
		Pair:       pair.String(),
		EventType:  mysql.BuyEvent,
		Memo:       message.Text,
		RecordedAt: time.Now(),
	}); err != nil {
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
				b.Logger.Debug(domain.Green("contracted!!![id:%d]", order.ID))
				b.buyStandby = false
				b.setSkipEndTime(time.Now().Add(time.Duration(b.Config.BuyIntervalSeconds) * time.Second))
				return nil
			}
		}
		time.Sleep(1 * time.Second)
	}
}

func (b *Bot) cancel(orders []model.Order) error {
	if b.Config.DemoMode {
		return nil
	}

	for _, o := range orders {
		if err := b.CoincheckCli.DeleteOrder(o.ID); err != nil {
			return err
		}
	}

	if len(orders) > 0 {
		message := slack.TextMessage{
			Text: "canel orders",
		}
		if err := b.SlackCli.PostMessage(message); err != nil {
			b.Logger.Error("%v", err)
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

func (b *Bot) calcSellRateAndAmount(info *ExchangeInfo, shouldLosscut bool) (rate float64, amount float64, err error) {
	amount = domain.Round(info.BalanceCurrency.Amount)

	totalJPY, err := b.MysqlCli.GetAccountInfo(mysql.AccountInfoTypeTotalJPY)
	if err != nil {
		return 0, 0, err
	}
	if totalJPY == 0 {
		b.Logger.Debug(domain.Red("account info %s on RDS is empty", mysql.AccountInfoTypeTotalJPY))
		return 0, 0, nil
	}

	usedJPY := totalJPY - info.BalanceJPY.Amount
	profit := totalJPY * b.Config.FundsRatioPerOrder * b.Config.TargetProfitPer
	if shouldLosscut {
		profit = 0
	}
	rate = (usedJPY + profit) / amount

	return
}

func (b *Bot) sell(info *ExchangeInfo, rate float64, amount float64) error {
	b.Logger.Debug("======================================")
	defer b.Logger.Debug("======================================")

	if b.Config.DemoMode {
		b.Logger.Debug(
			"%s sell completed!!! (rate:%.3f, amount:%.3f)",
			domain.Cyan("[DEMO]"), rate, amount,
		)
		return nil
	}

	b.Logger.Debug("sending sell order ...")
	order, err := b.CoincheckCli.PostOrder(&model.NewOrder{
		Type:   model.Sell,
		Pair:   *info.Pair,
		Amount: &amount,
		Rate:   &rate,
	})
	if err != nil {
		return fmt.Errorf(
			"failed to send sell order(rate:%.3f, amount:%.3f); error :%w",
			rate,
			amount,
			err)
	}
	b.Logger.Debug(domain.Green("completed!!![id:%d,%.3f,%.3f]", order.ID, *order.Rate, order.Amount))
	message := slack.TextMessage{
		Text: fmt.Sprintf(
			"sell completed!!! `%s %.3f %.3f`",
			order.Pair.String(), *order.Rate, order.Amount,
		),
	}
	if err := b.SlackCli.PostMessage(message); err != nil {
		b.Logger.Error("%v", err)
	}
	if err := b.MysqlCli.AddEvent(&mysql.Event{
		Pair:       info.Pair.String(),
		EventType:  mysql.SellEvent,
		Memo:       message.Text,
		RecordedAt: time.Now(),
	}); err != nil {
		b.Logger.Error("%v", err)
	}

	return nil
}

func (b *Bot) marketSellAndWaitForContract(pair *model.CurrencyPair, amount float64) error {
	b.Logger.Debug("======================================")
	defer b.Logger.Debug("======================================")

	b.Logger.Debug("sending market sell order ...")
	order, err := b.CoincheckCli.PostOrder(&model.NewOrder{
		Type:   model.MarketSell,
		Pair:   *pair,
		Amount: &amount,
	})
	if err != nil {
		return err
	}
	b.Logger.Debug(domain.Green("completed!!![id:%d,%.3f]", order.ID, amount))

	message := slack.TextMessage{
		Text: fmt.Sprintf(
			"market sell completed!!! `%s amount:%.3f`",
			order.Pair.String(),
			amount,
		),
	}
	if err := b.SlackCli.PostMessage(message); err != nil {
		b.Logger.Error("%v", err)
	}
	if err := b.MysqlCli.AddEvent(&mysql.Event{
		Pair:       pair.String(),
		EventType:  mysql.SellEvent,
		Memo:       message.Text,
		RecordedAt: time.Now(),
	}); err != nil {
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
				b.Logger.Debug(domain.Green("contracted!!![id:%d]", order.ID))
				return nil
			}
		}
		time.Sleep(1 * time.Second)
	}
}

func (b *Bot) setSkipEndTime(t time.Time) {
	b.skipEndTime = &t
	b.Logger.Debug("set skip end time => %s", domain.Yellow("%s", b.skipEndTime.Format(time.RFC3339)))
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

func (b *Bot) receiveTrade(h *coincheck.TradeHistory) error {
	if h.Side == model.BuySide {
		b.buyVolumeChan <- h
	} else {
		b.sellVolumeChan <- h
	}
	return nil
}

func (b *Bot) ReceiveTradeHandler(ctx context.Context) func() error {
	return func() error {
		d := time.Duration(b.Config.VolumeCheckSeconds) * time.Second
		key := time.Now().Format(volumeKey)
		for {
			select {
			case h := <-b.sellVolumeChan:
				v, err := b.CoincheckCli.GetVolumes(b.Config.GetTargetPair(model.JPY), h.Side, d)
				if err != nil {
					return err
				}
				if v > b.Config.SellMaxVolume {
					b.Logger.Debug("[receive] %s (sell volume:%.3f > max:%.3f)", domain.Green("set buyStandby"), v, b.Config.SellMaxVolume)
					b.buyStandby = true
					// 警戒期間をクリア
					b.setSkipEndTime(time.Now())
				} else {
					b.Logger.Debug("[receive] skip set buyStandby (sell volume:%.3f <= max:%.3f)", v, b.Config.SellMaxVolume)
				}

				total := 0.0
				if v1, ok := b.sellVolumeCache.Load(key); ok {
					if v2, ok := v1.(float64); ok {
						total = v2
					}
				}
				b.sellVolumeCache.Store(key, total+h.Amount)
			case h := <-b.buyVolumeChan:
				v, err := b.CoincheckCli.GetVolumes(b.Config.GetTargetPair(model.JPY), h.Side, d)
				if err != nil {
					return err
				}
				if v > b.Config.BuyMaxVolume {
					b.Logger.Debug("[receive] %s (buy volume:%.3f > max:%.3f)", domain.Red("record soaredTime"), v, b.Config.BuyMaxVolume)
					b.setSkipEndTime(time.Now().Add(time.Duration(b.Config.SoaredWarningPeriodSeconds) * time.Second))
				} else {
					b.Logger.Debug("[receive] skip record soaredTime (buy volume:%.3f <= max:%.3f)", v, b.Config.BuyMaxVolume)
				}

				total := 0.0
				if v1, ok := b.buyVolumeCache.Load(key); ok {
					if v2, ok := v1.(float64); ok {
						total = v2
					}
				}
				b.buyVolumeCache.Store(key, total+h.Amount)
			case <-ctx.Done():
				close(b.sellVolumeChan)
				close(b.buyVolumeChan)
				return nil
			}
		}
	}
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

	storeRate, err := b.CoincheckCli.GetStoreRate(pair)
	if err != nil {
		return err
	}
	sellRate, err := b.CoincheckCli.GetOrderRate(pair, model.SellSide)
	if err != nil {
		return err
	}
	buyRate, err := b.CoincheckCli.GetOrderRate(pair, model.BuySide)
	if err != nil {
		return err
	}

	if err := b.MysqlCli.AddRates(pair, sellRate.Rate, time.Now()); err != nil {
		return err
	}

	key := time.Now().Format(volumeKey)
	volumeSell := 0.0
	if v1, ok := b.sellVolumeCache.Load(key); ok {
		if v2, ok := v1.(float64); ok {
			volumeSell = v2
		}
		b.sellVolumeCache.Delete(key)
	}
	volumeBuy := 0.0
	if v1, ok := b.buyVolumeCache.Load(key); ok {
		if v2, ok := v1.(float64); ok {
			volumeBuy = v2
		}
		b.buyVolumeCache.Delete(key)
	}

	m := mysql.Market{
		Pair:         pair.String(),
		StoreRateAVG: storeRate.Rate,
		ExRateSell:   sellRate.Rate,
		ExRateBuy:    buyRate.Rate,
		ExVolumeSell: volumeSell,
		ExVolumeBuy:  volumeBuy,
		RecordedAt:   time.Now(),
	}
	if err := b.MysqlCli.AddMarket(&m); err != nil {
		return err
	}

	return nil
}
