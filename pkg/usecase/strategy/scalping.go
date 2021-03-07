package strategy

import (
	"context"
	"log"
	"time"
	"trading-bot/pkg/domain/model"
	"trading-bot/pkg/usecase/trade"

	"github.com/BurntSushi/toml"
	"github.com/markcheno/go-talib"
)

// Scalping スキャルピング戦略
type Scalping struct {
	facade *trade.Facade

	config       *scalpingConfig
	interval     time.Duration
	currencyPair *model.CurrencyPair
}

// NewScalpingStrategy 戦略を生成
func NewScalpingStrategy(facade *trade.Facade) (*Scalping, error) {
	s := &Scalping{
		facade: facade,
	}

	if err := s.loadConfig(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Scalping) loadConfig() error {
	const configPath = "./configs/bot-scalping.toml"
	var conf scalpingConfig
	if _, err := toml.DecodeFile(configPath, &conf); err != nil {
		return err
	}
	s.config = &conf
	s.interval = time.Duration(conf.IntervalSeconds) * time.Second
	s.currencyPair = &model.CurrencyPair{
		Key:        model.CurrencyType(conf.TargetCurrency),
		Settlement: model.JPY,
	}
	return nil
}

type scalpingConfig struct {
	TargetCurrency         string  `toml:"target_currency"`
	IntervalSeconds        int     `toml:"interval_seconds"`
	PositionCountMax       int     `toml:"position_count_max"`
	FundsRatio             float32 `toml:"funds_ratio"`
	ShortTermSize          int     `toml:"short_term_size"`
	LongTermSize           int     `toml:"long_term_size"`
	LossCutLowerLimitPer   float64 `toml:"loss_cut_lower_limit_per"`
	FixProfitUpperLimitPer float64 `toml:"fis_profit_upper_limit_per"`
	BBandsNBDevUp          float64 `toml:"bbands_nb_dev_up"`
	BBandsNBDevDown        float64 `toml:"bbands_nb_dev_down"`
}

// Trade 取引
func (s *Scalping) Trade(ctx context.Context) error {
	if err := s.facade.FetchAll(s.currencyPair); err != nil {
		return err
	}

	rates := s.facade.GetSellRateHistory64(s.currencyPair)

	pp, err := s.facade.GetOpenPositions()
	if err != nil {
		return err
	}

	shouldBuy, err := s.shouldBuy(rates, len(pp))
	if err != nil {
		return err
	}
	if shouldBuy {
		if err := s.buy(); err != nil {
			return err
		}
	}

	shouldSell, err := s.shouldSell(rates, len(pp))
	if err != nil {
		return err
	}
	for _, p := range pp {
		shouldFixProfit, err := s.shouldFixProfit(rates, &p)
		if err != nil {
			return err
		}
		shouldLossCut, err := s.shouldLossCut(rates, &p)
		if err != nil {
			return err
		}
		if shouldSell || shouldFixProfit || shouldLossCut {
			if err := s.sell(&p); err != nil {
				return err
			}
		}
	}

	return nil
}

// Wait 待機
func (s *Scalping) Wait(ctx context.Context) error {
	log.Printf("waiting ... (%v)\n", s.interval)
	ctx, cancel := context.WithTimeout(ctx, s.interval)
	defer cancel()
	select {
	case <-ctx.Done():
		if ctx.Err() != context.Canceled && ctx.Err() != context.DeadlineExceeded {
			return ctx.Err()
		}
		return nil
	}
}

func (s *Scalping) shouldBuy(rates []float64, posCount int) (bool, error) {
	// レート情報が少ないときは判断不可
	if len(rates) < s.config.LongTermSize {
		log.Printf("[buy] => skip buy (rate count:%d < required:%d)", len(rates), s.config.LongTermSize)
		return false, nil
	}

	if posCount >= s.config.PositionCountMax {
		log.Printf("[buy] => skip buy (open pos count: %d >= max(%d))", posCount, s.config.PositionCountMax)
		return false, nil
	}

	rate := rates[len(rates)-1]

	_, middles, lowers := talib.BBands(
		rates,
		s.config.LongTermSize,
		s.config.BBandsNBDevUp,
		s.config.BBandsNBDevDown,
		talib.SMA)
	middle := middles[len(middles)-1]
	lower := lowers[len(lowers)-1]
	bbandsDiff := middle - lower

	buyRate, err := s.facade.GetBuyRate(s.currencyPair)
	if err != nil {
		return false, err
	}
	sellRate, err := s.facade.GetSellRate(s.currencyPair)
	if err != nil {
		return false, err
	}
	rateDiff := float64(buyRate - sellRate)

	if rateDiff > bbandsDiff {
		log.Printf("[buy] => skip buy (rate diff:%.3f > BBands diff:%.3f)", rateDiff, bbandsDiff)
		return false, err
	}

	if rate < lower {
		log.Printf("[buy] => should buy (rate:%.3f < BBands lower:%.3f)", rate, lower)
		return true, nil
	}
	log.Printf("[buy] => skip buy (rate:%.3f >= BBands lower:%.3f)", rate, lower)
	return false, nil
}

func (s *Scalping) buy() error {
	balance, err := s.facade.GetJpyBalance()
	if err != nil {
		return err
	}
	amount := balance.Amount * s.config.FundsRatio

	log.Printf("[buy] sending buy order ...")
	pos, err := s.facade.SendMarketBuyOrder(s.currencyPair, amount, nil)
	if err != nil {
		return err
	}
	log.Printf("[buy] completed to send buy order [%v]", pos.OpenerOrder)

	return nil
}

func (s *Scalping) shouldSell(rates []float64, posCount int) (bool, error) {
	// レート情報が少ないときは判断不可
	if len(rates) < s.config.LongTermSize {
		log.Printf("[sell] => skip sell (rate count:%d < required:%d)", len(rates), s.config.LongTermSize)
		return false, nil
	}

	if posCount == 0 {
		log.Printf("[sell] => skip buy (open pos nothing)")
		return false, nil
	}

	rate := rates[len(rates)-1]

	uppers, _, _ := talib.BBands(
		rates,
		s.config.LongTermSize,
		s.config.BBandsNBDevUp,
		s.config.BBandsNBDevDown,
		talib.SMA)
	upper := uppers[len(uppers)-1]

	if rate >= upper {
		log.Printf("[sell] => should sell (rate:%.3f >= BBands upper:%.3f)", rate, upper)
		return true, nil
	}
	log.Printf("[sell] => skip sell (rate:%.3f < BBands upper:%.3f)", rate, upper)
	return false, nil
}

// shouldFixProfit 利確すべきか判定
func (s *Scalping) shouldFixProfit(rates []float64, pos *model.Position) (bool, error) {
	// レート情報が少ないときは判断不可
	if len(rates) < s.config.LongTermSize {
		log.Printf("[pos:%d][fixProfit] => skip fix profit (rate count:%d < required:%d)", pos.ID, len(rates), s.config.LongTermSize)
		return false, nil
	}

	contracts, err := s.facade.GetContracts(pos.OpenerOrder.ID)
	if err != nil {
		return false, err
	}
	var buyJPY float64
	var amount float64
	for _, c := range contracts {
		buyJPY += float64(-c.DecreaseAmount)
		amount += float64(c.IncreaseAmount)
	}
	upperLimit := buyJPY * s.config.FixProfitUpperLimitPer
	sellJPY := rates[len(rates)-1] * amount

	// 上限以下なら利確しない
	if sellJPY <= upperLimit {
		log.Printf(
			"[pos:%d][fixProfit] => skip fix profit (sell[jpy:%.3f] <= upper limit[jpy:%.3f] = buy[jpy:%.3f] * %.3f)",
			pos.ID,
			sellJPY,
			upperLimit,
			buyJPY,
			s.config.FixProfitUpperLimitPer,
		)

		return false, nil
	}

	log.Printf(
		"[pos:%d][losscut] => should fix profit (sell[jpy:%.3f] > upper limit[jpy:%.3f] = buy[jpy:%.3f] * %.3f)",
		pos.ID,
		sellJPY,
		upperLimit,
		buyJPY,
		s.config.FixProfitUpperLimitPer,
	)
	return true, nil
}

// ShouldLossCut ロスカットすべきか判定
func (s *Scalping) shouldLossCut(rates []float64, pos *model.Position) (bool, error) {
	// レート情報が少ないときは判断不可
	if len(rates) < s.config.LongTermSize {
		log.Printf("[pos:%d][losscut] => skip loss cut (rate count:%d < required:%d)", pos.ID, len(rates), s.config.LongTermSize)
		return false, nil
	}

	sRates := talib.Ema(rates, s.config.ShortTermSize)
	sRate := sRates[len(sRates)-1]
	lRates := talib.Ema(rates, s.config.LongTermSize)
	lRate := lRates[len(lRates)-1]

	// 上昇トレンドなら待機
	if sRate >= lRate {
		log.Printf("[pos:%d][losscut] => skip loss cut, up trend now (SMA short:%.3f >= long:%.3f, pos id: %d)", pos.ID, sRate, lRate, pos.ID)
		return false, nil
	}

	contracts, err := s.facade.GetContracts(pos.OpenerOrder.ID)
	if err != nil {
		return false, err
	}
	var buyJPY float64
	var amount float64
	for _, c := range contracts {
		buyJPY += float64(-c.DecreaseAmount)
		amount += float64(c.IncreaseAmount)
	}
	lowerLimit := buyJPY * s.config.LossCutLowerLimitPer
	sellJPY := rates[len(rates)-1] * amount

	// 下限以上ならロスカットしない
	if sellJPY >= lowerLimit {
		log.Printf(
			"[pos:%d][losscut] => skip loss cut (sell[jpy:%.3f] >= lower limit[jpy:%.3f] = buy[jpy:%.3f] * %.3f)",
			pos.ID,
			sellJPY,
			lowerLimit,
			buyJPY,
			s.config.LossCutLowerLimitPer,
		)

		return false, nil
	}

	log.Printf(
		"[pos:%d][losscut] => should loss cut (sell[jpy:%.3f] < lower limit[jpy:%.3f] = buy[jpy:%.3f] * %.3f)",
		pos.ID,
		sellJPY,
		lowerLimit,
		buyJPY,
		s.config.LossCutLowerLimitPer,
	)
	return true, nil
}

func (s *Scalping) sell(p *model.Position) error {
	contracts, err := s.facade.GetContracts(p.OpenerOrder.ID)
	if err != nil {
		return err
	}
	var amount float32
	for _, c := range contracts {
		amount += c.IncreaseAmount
	}

	log.Printf("[pos:%d][sell] sending sell order ...", p.ID)
	pos, err := s.facade.SendMarketSellOrder(s.currencyPair, amount, p)
	if err != nil {
		return err
	}
	log.Printf("[pos:%d][sell] completed to send sell order [%v]", pos.ID, pos.CloserOrder)

	return nil
}
