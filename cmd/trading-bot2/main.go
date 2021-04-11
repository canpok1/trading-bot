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
	"trading-bot/pkg/usecase/log"
	"trading-bot/pkg/usecase/trade"

	"github.com/kelseyhightower/envconfig"
	"golang.org/x/sync/errgroup"
)

var (
	rateDuration = 24 * time.Hour
)

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
	bot := NewBot(&config, coincheckCli, mysqlCli, &logger)

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

	// ===== エントリー判断関連 =====
	// サポートラインの判定範囲1（現在に近い方）
	SupportLinePeriod1 int `required:"true" split_words:"true"`
	// サポートラインの判定範囲2（現在から遠い方）
	SupportLinePeriod2 int `required:"true" split_words:"true"`
	// レートがどの程度下がったらナンピンするか
	AveragingDownRatePer float64 `required:"true" split_words:"true"`
	// 最大出来高（最大を超えたら買い準備に移行）
	MaxVolume float64 `required:"true" split_words:"true"`
	// 出来高の監視対象の時間幅（直近何秒までの出来高を見るか？）
	VolumeCheckSeconds int `required:"true" split_words:"true"`

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
	BalanceJPY float64
	// 買注文に利用中の金額（JPY）
	BuyJPY float64
	// 買注文に取得したコイン量
	BuyCurrencyAmount float64
}

type Bot struct {
	Config       *BotConfig
	CoincheckCli *coincheck.Client
	MysqlCli     *mysql.Client
	Logger       *memory.Logger

	buyStandby bool
}

func NewBot(config *BotConfig, coincheckCli *coincheck.Client, mysqlCli *mysql.Client, logger *memory.Logger) *Bot {
	return &Bot{
		Config:       config,
		CoincheckCli: coincheckCli,
		MysqlCli:     mysqlCli,
		Logger:       logger,
		buyStandby:   false,
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
	shouldBuy, err := b.shouldBuy(info)
	if err != nil {
		return false, err
	}
	if !shouldBuy {
		return false, err
	}

	// 成行買 → 約定待ち
	if err := b.buyAndWaitForContract(info.Pair, info.BalanceJPY+info.BuyJPY); err != nil {
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

	if info.BuyJPY == 0.0 {
		b.Logger.Debug("skip sell (buy JPY:%s == 0.000)", log.Yellow("%.3f", info.BuyJPY))
		return nil
	}

	if len(openOrders) > 0 {
		b.Logger.Debug("skip sell (open order count:%s > 0)", log.Yellow("%d", len(openOrders)))
		for _, order := range openOrders {
			b.Logger.Debug("open order => [%s rate:%.3f, amount:%.3f]", order.Type, *order.Rate, order.Amount)
		}
		return nil
	}
	b.Logger.Debug("%s (open order count:%s == 0)", log.Green("should sell"), log.Yellow("%d", len(openOrders)))

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
	buyContracts := trade.GetLastBuyContracts(&sellRate.Pair, contracts)

	buyJPY := 0.0
	buyCurrencyAmount := 0.0
	for _, c := range buyContracts {
		buyJPY += (-1) * c.DecreaseAmount
		buyCurrencyAmount += c.IncreaseAmount
	}

	balanceJPY, err := b.CoincheckCli.GetBalance(model.JPY)
	if err != nil {
		return nil, err
	}

	return &ExchangeInfo{
		Pair:              pair,
		SellRate:          sellRate.Rate,
		BuyRate:           buyRate.Rate,
		BalanceJPY:        balanceJPY.Amount,
		BuyJPY:            buyJPY,
		BuyCurrencyAmount: buyCurrencyAmount,
	}, nil
}

func (b *Bot) shouldBuy(info *ExchangeInfo) (bool, error) {
	rates, err := b.MysqlCli.GetRates(info.Pair, &rateDuration)
	if err != nil {
		return false, err
	}
	required := b.Config.SupportLinePeriod1 + b.Config.SupportLinePeriod2
	if len(rates) < required {
		b.Logger.Debug("skip buy (rate len:%s < SupportLine required:%d)", log.Yellow("%d", len(rates)), required)
		return false, nil
	}

	// サポートラインより下？
	supportLines := trade.SupportLine(rates, b.Config.SupportLinePeriod1, b.Config.SupportLinePeriod2)
	supportLine := supportLines[len(supportLines)-1]
	supportLineCrossed := info.SellRate < supportLine
	if supportLineCrossed {
		b.Logger.Debug("%s support line crossed (sell:%s < support:%s)", log.Green("OK"), log.Yellow("%.3f", info.SellRate), log.Yellow("%.3f", supportLine))
	} else {
		b.Logger.Debug("%s support line not crossed (sell:%s >= support:%s)", log.Red("NG"), log.Yellow("%.3f", info.SellRate), log.Yellow("%.3f", supportLine))
	}

	// 前注文よりレート下？
	averagingDown := false
	averagingDownLittle := false
	if info.BuyCurrencyAmount == 0.0 {
		b.Logger.Debug("%s can averaging down (%s: nothing)", log.Green("OK"), info.Pair.Key)
		averagingDown = true
		averagingDownLittle = true
	} else {
		orderRateAVG := info.BuyJPY / info.BuyCurrencyAmount
		border := orderRateAVG * b.Config.AveragingDownRatePer
		averagingDown = info.BuyRate < border
		averagingDownLittle = info.BuyRate < orderRateAVG
		if averagingDown {
			b.Logger.Debug("%s can averaging down (buyRate:%s < border:%s)", log.Green("OK"), log.Yellow("%.3f", info.BuyRate), log.Yellow("%.3f", border))
		} else if averagingDownLittle {
			b.Logger.Debug("%s cannot averaging down (border:%s =< buyRate:%s < AVG:%s)", log.Red("NG"), log.Yellow("%.3f", border), log.Yellow("%.3f", info.BuyRate), log.Yellow("%.3f", orderRateAVG))
		} else {
			b.Logger.Debug("%s cannot averaging down (buyRate:%s >= border:%s)", log.Red("NG"), log.Yellow("%.3f", info.BuyRate), log.Yellow("%.3f", border))
		}
	}

	// 注文する余裕ある？
	totalJPY := info.BalanceJPY + info.BuyJPY
	borderJPY := totalJPY * b.Config.FundsRatio
	canOrder := info.BuyJPY < borderJPY
	if canOrder {
		b.Logger.Debug("%s can order (buyAmount JPY:%s < border:%s)", log.Green("OK"), log.Yellow("%.3f", info.BuyJPY), log.Yellow("%.3f", borderJPY))
	} else {
		b.Logger.Debug("%s cannot order (buyAmount JPY:%s >= border:%s)", log.Red("NG"), log.Yellow("%.3f", info.BuyJPY), log.Yellow("%.3f", borderJPY))
	}

	if !supportLineCrossed || !averagingDown || !canOrder {
		b.Logger.Debug("skip buy (supportLineCrossed:%v, averagingDown:%v, canOrder:%v)", supportLineCrossed, averagingDown, canOrder)

		hasPosition := info.BuyCurrencyAmount > 0
		newStandby := hasPosition && averagingDownLittle && canOrder
		if !b.buyStandby && newStandby {
			b.Logger.Debug("%s (hasPosition:%v,averagingDownLittle:%v,canOrder:%v)", log.Green("set buyStandby"), hasPosition, averagingDownLittle, canOrder)
		} else if b.buyStandby && !newStandby {
			b.Logger.Debug("%s (hasPosition:%v,averagingDownLittle:%v,canOrder:%v)", log.Green("release buyStandby"), hasPosition, averagingDownLittle, canOrder)
		}
		b.buyStandby = newStandby

		return false, nil
	}
	b.Logger.Debug("%s (supportLineCrossed:%v, averagingDown:%v, canOrder:%v)", log.Green("should buy"), supportLineCrossed, averagingDown, canOrder)
	return true, nil
}

func (b *Bot) buyAndWaitForContract(pair *model.CurrencyPair, totalJPY float64) error {
	b.Logger.Debug("======================================")
	defer b.Logger.Debug("======================================")

	amount := totalJPY * b.Config.FundsRatioPerOrder

	b.Logger.Debug("sending buy order ...")
	order, err := b.CoincheckCli.PostOrder(&model.NewOrder{
		Type:            model.MarketBuy,
		Pair:            *pair,
		MarketBuyAmount: &amount,
	})
	if err != nil {
		return err
	}
	b.Logger.Debug(log.Green("completed!!![id:%d,%.3f]", order.ID, order.Amount))

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

	profit := info.BuyJPY * b.Config.TargetProfitPer
	rate := (info.BuyJPY + profit) / info.BuyCurrencyAmount

	b.Logger.Debug("sending sell order ...")
	order, err := b.CoincheckCli.PostOrder(&model.NewOrder{
		Type:   model.Sell,
		Pair:   *info.Pair,
		Amount: &info.BuyCurrencyAmount,
		Rate:   &rate,
	})
	if err != nil {
		return fmt.Errorf(
			"failed to send sell order; buyJPY:%.3f, rate:%.3f, amount:%.3f, error :%w",
			info.BuyJPY,
			rate,
			info.BuyCurrencyAmount,
			err)
	}
	b.Logger.Debug(log.Green("completed!!![id:%d,%.3f,%.3f]", order.ID, *order.Rate, order.Amount))
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
	if side != model.SellSide {
		return nil
	}

	d := time.Duration(b.Config.VolumeCheckSeconds) * time.Second
	v, err := b.CoincheckCli.GetVolumes(b.Config.GetTargetPair(model.JPY), side, d)
	if err != nil {
		return err
	}
	if v > b.Config.MaxVolume {
		b.Logger.Debug("[receive] %s (sell volume:%.3f > max:%.3f)", log.Green("set buyStandby"), v, b.Config.MaxVolume)
		b.buyStandby = true
	} else {
		b.Logger.Debug("[receive] skip set buyStandby (sell volume:%.3f <= max:%.3f)", v, b.Config.MaxVolume)
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
