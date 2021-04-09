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

type RangeConfig struct {
	Interval               int     `toml:"interval_seconds"`
	FundsRatio             float64 `toml:"funds_ratio"`
	TermSize               int     `toml:"term_size"`
	LossCutLowerLimitPer   float64 `toml:"loss_cut_lower_limit_per"`
	FixProfitUpperLimitPer float64 `toml:"fix_profit_upper_limit_per"`
	BBandsNBDevUp          float64 `toml:"bbands_nb_dev_up"`
	BBandsNBDevDown        float64 `toml:"bbands_nb_dev_down"`
	BBandsMaxWidthRate     float64 `toml:"bbands_max_width_rate"`
}

func NewRangeConfig(f string) (*RangeConfig, error) {
	var conf RangeConfig
	if _, err := toml.DecodeFile(f, &conf); err != nil {
		return nil, err
	}
	return &conf, nil
}

type RangeStrategy struct {
	logger domain.Logger
	facade *trade.Facade

	config *RangeConfig
}

func NewRangeStrategy(facade *trade.Facade, logger domain.Logger, config *RangeConfig) (*RangeStrategy, error) {
	return &RangeStrategy{
		logger: logger,
		facade: facade,
		config: config,
	}, nil
}

func (s *RangeStrategy) Buy(p model.CurrencyPair, positions []model.Position) error {
	rates, err := s.facade.GetRates(&p)
	if err != nil {
		return err
	}

	if len(rates) < s.config.TermSize {
		s.logger.Debug("[buy] => skip buy (rate count:%d <= required:%d)", len(rates), s.config.TermSize)
		return nil
	}

	bbUppers, bbMiddles, bbLowers := talib.BBands(
		rates,
		s.config.TermSize,
		s.config.BBandsNBDevUp,
		s.config.BBandsNBDevDown,
		talib.EMA)

	bbUpper := bbUppers[len(bbUppers)-1]
	bbMiddle := bbMiddles[len(bbMiddles)-1]
	bbLower := bbLowers[len(bbLowers)-1]

	bbWidth := bbUpper - bbLower
	bbMaxWidth := bbMiddle * s.config.BBandsMaxWidthRate
	if (bbUpper - bbLower) > bbMaxWidth {
		s.logger.Debug("[buy] => skip buy (bband width:%.5f > max:%.5f)", bbWidth, bbMaxWidth)
		return nil
	}

	rate := rates[len(rates)-1]
	if rate > bbLower {
		s.logger.Debug("[buy] => skip buy (rate:%.3f > bband lower:%.3f)", rate, bbLower)
		return nil
	}
	s.logger.Debug("[buy] => should buy (rate:%.3f <= bband lower:%.3f)", rate, bbLower)

	return s.buy(&p)
}

func (s *RangeStrategy) BuyTradeCallback(p model.CurrencyPair, rate float64) error {
	return nil
}

func (s *RangeStrategy) buy(p *model.CurrencyPair) error {
	balance, err := s.facade.GetJpyBalance()
	if err != nil {
		return err
	}
	amount := balance.Amount * s.config.FundsRatio

	s.logger.Debug("[buy] sending buy order ...")
	pos, err := s.facade.SendMarketBuyOrder(p, amount, nil)
	if err != nil {
		return err
	}
	s.logger.Debug("[buy] completed to send buy order [%v]", pos.OpenerOrder)

	return nil
}

func (s *RangeStrategy) Sell(pair model.CurrencyPair, positions []model.Position) error {
	rates, err := s.facade.GetRates(&pair)
	if err != nil {
		return err
	}

	if len(rates) < s.config.TermSize {
		s.logger.Debug("[sell] => skip sell (rate count:%d <= required:%d)", len(rates), s.config.TermSize)
		return nil
	}
	if len(positions) == 0 {
		s.logger.Debug("[sell] => skip sell (open pos nothing)")
		return nil
	}

	shouldSell, err := s.shouldSell(&pair, rates, positions)
	if err != nil {
		return err
	}
	for _, p := range positions {
		shouldFixProfit, err := s.shouldFixProfit(&pair, rates, &p)
		if err != nil {
			return err
		}
		shouldLossCut, err := s.shouldLossCut(&pair, rates, &p)
		if err != nil {
			return err
		}
		if shouldSell || shouldFixProfit || shouldLossCut {
			if err := s.sell(&pair, &p); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *RangeStrategy) SellTradeCallback(pair model.CurrencyPair, rate float64) error {
	return nil
}

func (s *RangeStrategy) shouldSell(pair *model.CurrencyPair, rates []float64, positions []model.Position) (bool, error) {
	bbUppers, _, _ := talib.BBands(
		rates,
		s.config.TermSize,
		s.config.BBandsNBDevUp,
		s.config.BBandsNBDevDown,
		talib.EMA)

	bbUpper := bbUppers[len(bbUppers)-1]

	rate := rates[len(rates)-1]
	if rate < bbUpper {
		s.logger.Debug("[sell] => skip sell (rate:%.3f < bband upper:%.3f)", rate, bbUpper)
		return false, nil
	}
	s.logger.Debug("[sell] => should sell (rate:%.3f >= bband upper:%.3f)", rate, bbUpper)
	return true, nil
}

// shouldFixProfit 利確すべきか判定
func (s *RangeStrategy) shouldFixProfit(pair *model.CurrencyPair, rates []float64, pos *model.Position) (bool, error) {
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
func (s *RangeStrategy) shouldLossCut(pair *model.CurrencyPair, rates []float64, pos *model.Position) (bool, error) {
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

func (s *RangeStrategy) sell(pair *model.CurrencyPair, p *model.Position) error {
	contracts, err := s.facade.GetContracts(p.OpenerOrder.ID)
	if err != nil {
		return err
	}
	var amount float64
	for _, c := range contracts {
		amount += c.IncreaseAmount
	}

	s.logger.Debug("[pos:%d][sell] sending sell order ...", p.ID)
	pos, err := s.facade.SendMarketSellOrder(pair, amount, p)
	if err != nil {
		return err
	}
	s.logger.Debug("[pos:%d][sell] completed to send sell order [%v]", pos.ID, pos.CloserOrder)

	return nil
}

func (s *RangeStrategy) Interval() time.Duration {
	return time.Duration(s.config.Interval) * time.Second
}

func (s *RangeStrategy) Wait(ctx context.Context) error {
	s.logger.Debug("waiting ... (%v)\n", s.config.Interval)
	return s.facade.Wait(ctx, time.Duration(s.config.Interval))
}
