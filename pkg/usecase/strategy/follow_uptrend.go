package strategy

import (
	"context"
	"fmt"
	"time"
	"trading-bot/pkg/domain"
	"trading-bot/pkg/domain/model"
	"trading-bot/pkg/usecase/trade"

	"github.com/BurntSushi/toml"
	"github.com/markcheno/go-talib"
)

const ()

// FollowUptrendStrategy 上昇トレンド追従戦略
type FollowUptrendStrategy struct {
	configPath   string
	logger       domain.Logger
	facade       *trade.Facade
	interval     time.Duration
	currencyPair *model.CurrencyPair

	fundsRatio            float32
	upRate                float32
	lossCutLowerLimitPer  float32
	contractCheckInterval time.Duration
	positionCountMax      int
	shortTermSize         int
	longTermSize          int
}

// NewFollowUptrendStrategy 戦略を生成
func NewFollowUptrendStrategy(facade *trade.Facade, logger domain.Logger, configPath string) (*FollowUptrendStrategy, error) {
	s := &FollowUptrendStrategy{
		configPath: configPath,
		logger:     logger,
		facade:     facade,
	}

	if err := s.loadConfig(); err != nil {
		return nil, err
	}

	return s, nil
}

// Trade 取引実施
func (f *FollowUptrendStrategy) Trade(ctx context.Context) error {
	if err := f.facade.FetchAll(f.currencyPair); err != nil {
		return err
	}

	buyRate, sellRate, err := f.getRate()
	if err != nil {
		return err
	}
	f.logger.Debug("===== rate[%s] buy: %.3f sell: %.3f =====\n", f.currencyPair, buyRate, sellRate)

	pp, err := f.facade.GetOpenPositions()
	if err != nil {
		return err
	}

	if len(pp) < f.positionCountMax {
		f.logger.Debug("[check] open poss [count: %d] < %d => check new order", len(pp), f.positionCountMax)
		if err := f.checkNewOrder(); err != nil {
			return err
		}
	} else {
		f.logger.Debug("[check] open poss [count: %d] >= %d => skip new order", len(pp), f.positionCountMax)
	}

	for _, p := range pp {
		if err := f.checkPosition(&p); err != nil {
			return err
		}
	}

	return nil
}

func (f *FollowUptrendStrategy) checkNewOrder() error {
	if !f.IsBuySignal(f.facade.GetSellRateHistory(f.currencyPair)) {
		f.logger.Debug("[check] no buy signal => skip new order")
		return nil
	}

	// buyRate, sellRate, err := f.getRate()
	// if err != nil {
	// 	return err
	// }
	// if sellRate < buyRate*lossCutLowerLimitPer {
	// 	log.Printf("[check] sellRate is too low (sell:%.3f, buy:%.3f)=> skip new order", sellRate, buyRate)
	// 	return nil
	// }

	balance, err := f.facade.GetJpyBalance()
	if err != nil {
		return err
	}
	amount := balance.Amount * f.fundsRatio

	f.logger.Debug("[trade] sending buy order ...")
	pos1, err := f.facade.SendMarketBuyOrder(f.currencyPair, amount, nil)
	if err != nil {
		return err
	}
	f.logger.Debug("[trade] completed to send buy order [%v]", pos1.OpenerOrder)

	if _, err := f.waitContract(pos1.OpenerOrder.ID); err != nil {
		return err
	}

	f.logger.Debug("[trade] sending sell order ...")
	//pos2, err := f.sendSellOrder(pos1)
	pos2, err := f.sendSellOrder(pos1)
	if err != nil {
		return err
	}
	f.logger.Debug("[trade] completed to send sell order [%v]", *pos2.CloserOrder)

	return nil
}

func (f *FollowUptrendStrategy) checkPosition(pos *model.Position) error {
	if pos.OpenerOrder.Status == model.Open {
		f.logger.Debug("[check] position[id:%d]: OpenerOrder is OPEN => wait for contract new order[%s rate:%s amount:%.5f] ...",
			pos.ID,
			pos.OpenerOrder.Type,
			toDisplayStr(pos.OpenerOrder.Rate, "--"),
			pos.OpenerOrder.Amount)
		return nil
	}
	if pos.CloserOrder == nil {
		f.logger.Debug("[check] position[id:%d]: Closer Order is nothing => sending sell order ...", pos.ID)
		pos2, err := f.sendSellOrder(pos)
		if err != nil {
			return err
		}
		f.logger.Debug("[trade] completed to send order [%s rate:%s amount:%.5f]",
			pos2.CloserOrder.Type,
			toDisplayStr(pos2.CloserOrder.Rate, "--"),
			pos2.CloserOrder.Amount,
		)
		return nil
	}

	if pos.CloserOrder.Status == model.Open {
		f.logger.Debug("[check] position[id:%d]: Closer Order is OPEN => check for resend order[%s rate:%s amount:%.5f] ...",
			pos.ID,
			pos.CloserOrder.Type,
			toDisplayStr(pos.CloserOrder.Rate, "--"),
			pos.CloserOrder.Amount,
		)

		shouldLossCut := f.ShouldLossCut(f.facade.GetSellRateHistory(f.currencyPair), pos.CloserOrder)
		if shouldLossCut {
			f.logger.Debug("[trade] sending cancel order ...")
			pos2, err := f.facade.CancelSettleOrder(pos)
			if err != nil {
				return err
			}
			f.logger.Debug("[trade] completed to send cancel order[order_id:%d]", pos.CloserOrder.ID)

			f.logger.Debug("[trade] sending loss cut sell order ...")
			pos3, err := f.sendLossCutSellOrder(pos2)
			if err != nil {
				return err
			}
			f.logger.Debug("[trade] completed to send loss cut sell order[%s rate:%s amount:%.5f]",
				pos3.CloserOrder.Type,
				toDisplayStr(pos.CloserOrder.Rate, "--"),
				pos.CloserOrder.Amount)
		}
	}

	return nil
}

func (f *FollowUptrendStrategy) getRate() (buyRate, sellRate float32, err error) {
	b, err := f.facade.GetBuyRate(f.currencyPair)
	if err != nil {
		return 0, 0, nil
	}
	s, err := f.facade.GetSellRate(f.currencyPair)
	if err != nil {
		return 0, 0, nil
	}

	return b, s, nil
}

func (f *FollowUptrendStrategy) isUptrend() bool {
	// 一定期間の間の売レートの上昇回数が半分を超えてたら上昇トレンドと判断
	count := 0
	rates := f.facade.GetSellRateHistory(f.currencyPair)
	size := len(rates)
	for i := 1; i < size; i++ {
		if rates[i-1] < rates[i] {
			count++
		}
	}

	if count > ((size - 1) / 2) {
		f.logger.Debug("[check] rise count: %d / %d => UP trend", count, size-1)
		return true
	}
	f.logger.Debug("[check] rise count: %d / %d => not UP trend", count, size-1)
	return false
}

// func (f *FollowUptrendStrategy) sendBuyOrder() (*model.Position, error) {
// 	return f.facade.SendMarketBuyOrder(fundJpy, nil)
// }

func (f *FollowUptrendStrategy) waitContract(orderID uint64) ([]model.Contract, error) {
	f.logger.Debug("[trade] waiting for contract ...")
	for {
		if err := f.facade.FetchAll(f.currencyPair); err != nil {
			return nil, err
		}

		order, err := f.facade.GetOrder(orderID)
		if err != nil {
			return nil, err
		}
		if order != nil && order.Status == model.Closed {
			break
		}

		f.logger.Debug("[trade] not contracted, waiting for contract ...")
		time.Sleep(f.contractCheckInterval)
	}
	contracts, err := f.facade.GetContracts(orderID)
	if err != nil {
		return nil, err
	}

	f.logger.Debug("[trade] contracted!!! [%v]", contracts)
	return contracts, nil
}

func (f *FollowUptrendStrategy) sendSellOrder(p *model.Position) (*model.Position, error) {
	contracts, err := f.facade.GetContracts(p.OpenerOrder.ID)
	if err != nil {
		return nil, err
	}
	var rate float32
	var amount float32
	for _, c := range contracts {
		rate += c.Rate
		amount += c.IncreaseAmount
	}
	rate = rate / float32(len(contracts))

	return f.facade.SendSellOrder(f.currencyPair, amount, rate*(1.0+f.upRate), p)
}

func (f *FollowUptrendStrategy) sendLossCutSellOrder(p *model.Position) (*model.Position, error) {
	contracts, err := f.facade.GetContracts(p.OpenerOrder.ID)
	if err != nil {
		return nil, err
	}
	var amount float32
	for _, c := range contracts {
		amount += c.IncreaseAmount
	}

	return f.facade.SendMarketSellOrder(f.currencyPair, amount, p)
}

func toDisplayStr(v *float32, def string) string {
	if v == nil {
		return def
	}
	return fmt.Sprintf("%.3f", *v)
}

// IsBuySignal 買いシグナルかを判定
func (f *FollowUptrendStrategy) IsBuySignal(rates []float32) bool {
	// レート情報が少ないときは判断不可
	if len(rates) < f.longTermSize {
		f.logger.Debug("[check] buy signal, rate count: count:%d < required:%d => not buy signal", len(rates), f.longTermSize)
		return false
	}

	rr := []float64{}
	for _, r := range rates {
		rr = append(rr, float64(r))
	}

	sRates := talib.Ema(rr, f.shortTermSize)
	sRate := sRates[len(sRates)-1]
	lRates := talib.Ema(rr, f.longTermSize)
	lRate := lRates[len(lRates)-1]

	// 下降トレンド（短期の移動平均＜長期の移動平均）
	if sRate < lRate {
		f.logger.Debug("[check] SMA short:%.3f < long:%.3f => not UP trend", sRate, lRate)
		return false
	}

	// 上昇続いてる？
	count := 0
	size := len(rates)
	begin := size - f.longTermSize
	end := size - 1
	for i := begin + 1; i <= end; i++ {
		if rates[i-1] < rates[i] {
			count++
		}
	}
	if count < (f.longTermSize / 2) {
		f.logger.Debug("[check] rise count: %d / %d => not UP trend", count, f.longTermSize)
		return false
	}
	f.logger.Debug("[check] rise count: %d / %d => UP trend", count, f.longTermSize)
	return true
}

// ShouldLossCut ロスカットすべきか判定
func (f *FollowUptrendStrategy) ShouldLossCut(rates []float32, sellOrder *model.Order) bool {
	// レート情報が少ないときは判断不可
	if len(rates) < f.longTermSize {
		f.logger.Debug("[check] buy signal, rate count: count:%d < required:%d => not buy signal", len(rates), f.longTermSize)
		return false
	}

	rr := []float64{}
	for _, r := range rates {
		rr = append(rr, float64(r))
	}

	sRates := talib.Ema(rr, f.shortTermSize)
	sRate := sRates[len(sRates)-1]
	lRates := talib.Ema(rr, f.longTermSize)
	lRate := lRates[len(lRates)-1]

	// 上昇トレンドになりそうなら待機
	if sRate >= lRate {
		f.logger.Debug("[check] SMA short:%.3f >= long:%.3f => UP trend => skip loss cut", sRate, lRate)
		return false
	}

	if sellOrder.Rate == nil {
		return false
	}
	// 下限を下回ったらロスカット
	currentRate := rates[len(rates)-1]
	lowerLimit := *sellOrder.Rate * f.lossCutLowerLimitPer
	if lowerLimit <= currentRate {
		f.logger.Debug(
			"[check] order[rate:%.3f] * %.3f = lowerLimit:%.3f <= %s[rate:%.3f] => skip loss cut\n",
			*sellOrder.Rate,
			f.lossCutLowerLimitPer,
			lowerLimit,
			sellOrder.Pair.String(),
			currentRate)
		return false
	}
	f.logger.Debug(
		"[check] order[rate:%.3f] * %.3f = lowerLimit:%.3f > %s[rate:%.3f] => should loss cut\n",
		*sellOrder.Rate,
		f.lossCutLowerLimitPer,
		lowerLimit,
		sellOrder.Pair.String(),
		currentRate)
	return true
}

// Wait 待機
func (f *FollowUptrendStrategy) Wait(ctx context.Context) error {
	f.logger.Debug("waiting ... (%v)\n", f.interval)
	ctx, cancel := context.WithTimeout(ctx, f.interval)
	defer cancel()
	select {
	case <-ctx.Done():
		if ctx.Err() != context.Canceled && ctx.Err() != context.DeadlineExceeded {
			return ctx.Err()
		}
		return nil
	}
}

func (f *FollowUptrendStrategy) loadConfig() error {
	var conf followUptrendConfig
	if _, err := toml.DecodeFile(f.configPath, &conf); err != nil {
		return err
	}

	f.interval = time.Duration(conf.IntervalSeconds) * time.Second
	f.currencyPair = &model.CurrencyPair{
		Key:        model.CurrencyType(conf.TargetCurrency),
		Settlement: model.JPY,
	}
	f.fundsRatio = conf.FundsRatio
	f.upRate = conf.UpRate
	f.lossCutLowerLimitPer = conf.LossCutLowerLimitPer
	f.contractCheckInterval = time.Duration(conf.ContractCheckIntervalSeconds) * time.Second
	f.positionCountMax = conf.PositionCountMax
	f.shortTermSize = conf.ShortTermSize
	f.longTermSize = conf.LongTermSize

	return nil
}

type followUptrendConfig struct {
	IntervalSeconds              int     `toml:"interval_seconds"`
	TargetCurrency               string  `toml:"target_currency"`
	FundsRatio                   float32 `toml:"funds_ratio"`
	UpRate                       float32 `toml:"up_rate"`
	LossCutLowerLimitPer         float32 `toml:"loss_cut_lower_limit_per"`
	ContractCheckIntervalSeconds int     `toml:"contract_check_interval_seconds"`
	PositionCountMax             int     `toml:"position_count_max"`
	ShortTermSize                int     `toml:"short_term_size"`
	LongTermSize                 int     `toml:"long_term_size"`
}
