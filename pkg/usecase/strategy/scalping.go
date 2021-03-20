package strategy

import (
	"context"
	"time"
	"trading-bot/pkg/domain"
	"trading-bot/pkg/domain/model"
	"trading-bot/pkg/usecase/trade"

	"github.com/BurntSushi/toml"
	"github.com/markcheno/go-talib"
)

type ScalpingConfig struct {
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
	RsiLower               float64 `toml:"rsi_lower"`
	RsiUpper               float64 `toml:"rsi_upper"`
}

func NewScalpingConfig(f string) (*ScalpingConfig, error) {
	var conf ScalpingConfig
	if _, err := toml.DecodeFile(f, &conf); err != nil {
		return nil, err
	}
	return &conf, nil
}

func (c *ScalpingConfig) getCurrencyPair() *model.CurrencyPair {
	return &model.CurrencyPair{
		Key:        model.CurrencyType(c.TargetCurrency),
		Settlement: model.JPY,
	}
}

// Scalping スキャルピング戦略
type Scalping struct {
	logger domain.Logger
	facade *trade.Facade

	config *ScalpingConfig
}

// NewScalpingStrategy 戦略を生成
func NewScalpingStrategy(facade *trade.Facade, logger domain.Logger, config *ScalpingConfig) (*Scalping, error) {
	s := &Scalping{
		logger: logger,
		facade: facade,
		config: config,
	}

	return s, nil
}

// Trade 取引
func (s *Scalping) Trade(ctx context.Context) error {
	if err := s.facade.FetchAll(s.config.getCurrencyPair()); err != nil {
		return err
	}

	rates := s.facade.GetSellRateHistory64(s.config.getCurrencyPair())

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
	interval := time.Duration(s.config.IntervalSeconds) * time.Second

	s.logger.Debug("waiting ... (%v)\n", interval)
	ctx, cancel := context.WithTimeout(ctx, interval)
	defer cancel()

	<-ctx.Done()

	if ctx.Err() != context.Canceled && ctx.Err() != context.DeadlineExceeded {
		return ctx.Err()
	}
	return nil
}

// GetCurrency 対象通貨を取得
func (s *Scalping) GetCurrency() model.CurrencyType {
	return s.config.getCurrencyPair().Key
}

func (s *Scalping) shouldBuy(rates []float64, posCount int) (bool, error) {
	// レート情報が少ないときは判断不可
	if len(rates) <= s.config.LongTermSize {
		s.logger.Debug("[buy] => skip buy (rate count:%d <= required:%d)", len(rates), s.config.LongTermSize)
		return false, nil
	}

	if posCount >= s.config.PositionCountMax {
		s.logger.Debug("[buy] => skip buy (open pos count: %d >= max(%d))", posCount, s.config.PositionCountMax)
		return false, nil
	}

	rate := rates[len(rates)-1]

	// 売られすぎていてるなら買う
	rsis := talib.Rsi(rates, s.config.LongTermSize)
	rsi := rsis[len(rsis)-1]
	if rsi >= s.config.RsiLower {
		s.logger.Debug("[buy] => skip buy (rsi: %.3f >= lower: %.3f)", rsi, s.config.RsiLower)
		return false, nil
	}

	_, _, bbLowers := talib.BBands(
		rates,
		s.config.LongTermSize,
		s.config.BBandsNBDevUp,
		s.config.BBandsNBDevDown,
		talib.SMA)
	bbLower := bbLowers[len(bbLowers)-1]

	if rate > bbLower {
		s.logger.Debug("[buy] => skip buy (rate:%.3f > BBands lower:%.3f)", rate, bbLower)
		return false, nil
	}

	s.logger.Debug("[buy] => should buy (rsi: %.3f < %.3f, rate:%.3f <= BBands lower:%.3f)", rsi, s.config.RsiLower, rate, bbLower)
	return true, nil
}

func (s *Scalping) buy() error {
	balance, err := s.facade.GetJpyBalance()
	if err != nil {
		return err
	}
	amount := balance.Amount * s.config.FundsRatio

	s.logger.Debug("[buy] sending buy order ...")
	pos, err := s.facade.SendMarketBuyOrder(s.config.getCurrencyPair(), amount, nil)
	if err != nil {
		return err
	}
	s.logger.Debug("[buy] completed to send buy order [%v]", pos.OpenerOrder)

	return nil
}

func (s *Scalping) shouldSell(rates []float64, posCount int) (bool, error) {
	// レート情報が少ないときは判断不可
	if len(rates) < s.config.LongTermSize {
		s.logger.Debug("[sell] => skip sell (rate count:%d < required:%d)", len(rates), s.config.LongTermSize)
		return false, nil
	}

	if posCount == 0 {
		s.logger.Debug("[sell] => skip sell (open pos nothing)")
		return false, nil
	}

	rate := rates[len(rates)-1]

	// 買われすぎていたら売る
	rsis := talib.Rsi(rates, s.config.LongTermSize)
	rsi := rsis[len(rsis)-1]
	if rsi <= s.config.RsiUpper {
		s.logger.Debug("[sell] => skip sell (rsi: %.3f <= %.3f)", rsi, s.config.RsiUpper)
		return false, nil
	}

	bbUppers, _, _ := talib.BBands(
		rates,
		s.config.LongTermSize,
		s.config.BBandsNBDevUp,
		s.config.BBandsNBDevDown,
		talib.SMA)
	bbUpper := bbUppers[len(bbUppers)-1]

	if rate <= bbUpper {
		s.logger.Debug("[sell] => should sell (rate:%.3f <= BBands upper:%.3f)", rate, bbUpper)
		return false, nil
	}

	s.logger.Debug("[sell] => skip sell (rsi: %.3f > upper: %.3f, rate:%.3f > BBands upper:%.3f)", rsi, s.config.RsiUpper, rate, bbUpper)
	return true, nil
}

// shouldFixProfit 利確すべきか判定
func (s *Scalping) shouldFixProfit(rates []float64, pos *model.Position) (bool, error) {
	// レート情報が少ないときは判断不可
	if len(rates) <= s.config.LongTermSize {
		s.logger.Debug("[pos:%d][fixProfit] => skip fix profit (rate count:%d <= required:%d)", pos.ID, len(rates), s.config.LongTermSize)
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
		s.logger.Debug(
			"[pos:%d][fixProfit] => skip fix profit (sell[jpy:%.3f] <= upper limit[jpy:%.3f] = buy[jpy:%.3f] * %.3f)",
			pos.ID,
			sellJPY,
			upperLimit,
			buyJPY,
			s.config.FixProfitUpperLimitPer,
		)

		return false, nil
	}

	s.logger.Debug(
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
	if len(rates) <= s.config.LongTermSize {
		s.logger.Debug("[pos:%d][losscut] => skip loss cut (rate count:%d <= required:%d)", pos.ID, len(rates), s.config.LongTermSize)
		return false, nil
	}

	sRates := talib.Ema(rates, s.config.ShortTermSize)
	sRate := sRates[len(sRates)-1]
	lRates := talib.Ema(rates, s.config.LongTermSize)
	lRate := lRates[len(lRates)-1]

	// 上昇トレンドなら待機
	if sRate >= lRate {
		s.logger.Debug("[pos:%d][losscut] => skip loss cut, up trend now (SMA short:%.3f >= long:%.3f, pos id: %d)", pos.ID, sRate, lRate, pos.ID)
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
		s.logger.Debug(
			"[pos:%d][losscut] => skip loss cut (sell[jpy:%.3f] >= lower limit[jpy:%.3f] = buy[jpy:%.3f] * %.3f)",
			pos.ID,
			sellJPY,
			lowerLimit,
			buyJPY,
			s.config.LossCutLowerLimitPer,
		)

		return false, nil
	}

	s.logger.Debug(
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

	s.logger.Debug("[pos:%d][sell] sending sell order ...", p.ID)
	pos, err := s.facade.SendMarketSellOrder(s.config.getCurrencyPair(), amount, p)
	if err != nil {
		return err
	}
	s.logger.Debug("[pos:%d][sell] completed to send sell order [%v]", pos.ID, pos.CloserOrder)

	return nil
}
