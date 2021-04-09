package strategy

import (
	"context"
	"fmt"
	"math"
	"time"
	"trading-bot/pkg/domain"
	"trading-bot/pkg/domain/model"
	"trading-bot/pkg/usecase/trade"

	"github.com/BurntSushi/toml"
	"github.com/markcheno/go-talib"
)

type InagoConfig struct {
	Interval               int     `toml:"interval_seconds"`
	FundsRatio             float64 `toml:"funds_ratio"`
	LossCutLowerLimitPer   float64 `toml:"loss_cut_lower_limit_per"`
	FixProfitUpperLimitPer float64 `toml:"fix_profit_upper_limit_per"`

	// 売り判断時のROCの確認範囲
	SellROCPeriod int `toml:"sell_roc_period"`

	// 連続で買い注文を出せる最短間隔（秒）
	BuyIntervalSeconds int `toml:"buy_interval_seconds"`

	// サポートラインの判定範囲1（現在に近い方）
	SupportLinePeriod1 int `toml:"support_line_period_1"`
	// サポートラインの判定範囲2（現在から遠い方）
	SupportLinePeriod2 int `toml:"support_line_period_2"`

	// 最大出来高（最大を超えたら売準備に移行）
	MaxVolume float64 `toml:"max_volume"`
	// 出来高の監視対象の時間幅（直近何秒までの出来高を見るか？）
	VolumeCheckSeconds int `toml:"volume_check_seconds"`

	// レートがどの程度下がったらナンピンするか
	AveragingDownRatePer float64 `toml:"averaging_down_rate_per"`

	//LongTermSize           int     `toml:"long_term_size"`
	//ShortTermSize          int     `toml:"short_term_size"`
	//CrossCheckWidth        int     `toml:"cross_check_width"`
}

func (c *InagoConfig) valid() error {
	if c.Interval == 0 {
		return fmt.Errorf("Interval is empty, %v", c.Interval)
	}
	if c.FundsRatio == 0 {
		return fmt.Errorf("FundsRatio is empty, %v", c.FundsRatio)
	}
	if c.LossCutLowerLimitPer == 0 {
		return fmt.Errorf("LossCutLowerLimitPer is empty, %v", c.LossCutLowerLimitPer)
	}
	if c.FixProfitUpperLimitPer == 0 {
		return fmt.Errorf("FixProfitUpperLimitPer is empty, %v", c.FixProfitUpperLimitPer)
	}
	if c.SellROCPeriod == 0 {
		return fmt.Errorf("SellROCPeriod is empty, %v", c.SellROCPeriod)
	}
	if c.BuyIntervalSeconds == 0 {
		return fmt.Errorf("BuyIntervalSeconds is empty, %v", c.BuyIntervalSeconds)
	}
	if c.SupportLinePeriod1 == 0 {
		return fmt.Errorf("SupportLinePeriod1 is empty, %v", c.SupportLinePeriod1)
	}
	if c.SupportLinePeriod2 == 0 {
		return fmt.Errorf("SupportLinePeriod2 is empty, %v", c.SupportLinePeriod2)
	}
	if c.MaxVolume == 0 {
		return fmt.Errorf("MaxVolume is empty, %v", c.MaxVolume)
	}
	if c.VolumeCheckSeconds == 0 {
		return fmt.Errorf("VolumeCheckSeconds is empty, %v", c.VolumeCheckSeconds)
	}
	if c.AveragingDownRatePer == 0 {
		return fmt.Errorf("AveragingDownRatePer is empty, %v", c.AveragingDownRatePer)
	}
	return nil
}

func NewInagoConfig(f string) (*InagoConfig, error) {
	var conf InagoConfig
	if _, err := toml.DecodeFile(f, &conf); err != nil {
		return nil, err
	}
	if err := conf.valid(); err != nil {
		return nil, fmt.Errorf("[%s] validation error: %w", f, err)
	}
	return &conf, nil
}

type InagoStrategy struct {
	logger domain.Logger
	facade *trade.Facade

	config *InagoConfig

	sellStandby bool

	longEma  float64
	shortEma float64
}

func NewInagoStrategy(facade *trade.Facade, logger domain.Logger, config *InagoConfig) (*InagoStrategy, error) {
	return &InagoStrategy{
		logger:      logger,
		facade:      facade,
		config:      config,
		sellStandby: false,
	}, nil
}

func (s *InagoStrategy) Buy(pair model.CurrencyPair, positions []model.Position) error {
	rr, err := s.facade.GetRates(&pair)
	if err != nil {
		return err
	}

	supportLineCrossed, err := s.supportLineCross(pair, rr, s.config.SupportLinePeriod1, s.config.SupportLinePeriod2)
	if err != nil {
		return err
	}
	canAveragingDown, err := s.canAveragingDown(pair, positions)
	if err != nil {
		return err
	}
	canOrder := s.canOrder(positions)

	if !supportLineCrossed || !canAveragingDown || !canOrder {
		s.logger.Debug("[buy] => skip (supportLineCrossed:%v, canAveragingDown:%v, canOrder:%v)", supportLineCrossed, canAveragingDown, canOrder)
		return nil
	}
	s.logger.Debug("[buy] => should buy (supportLineCrossed:%v, canAveragingDown:%v, canOrder:%v)", supportLineCrossed, canAveragingDown, canOrder)
	return s.buy(&pair)
}

func (s *InagoStrategy) canOrder(positions []model.Position) bool {
	border := time.Now().Add(-1 * time.Duration(s.config.BuyIntervalSeconds) * time.Second)
	for _, pos := range positions {
		if pos.OpenerOrder.OrderedAt.After(border) {
			s.logger.Debug("[buy] => cannot be ordered(already ordred within %dsec)(%v)", s.config.BuyIntervalSeconds, pos.OpenerOrder.OrderedAt)
			return false
		}
	}

	s.logger.Debug("[buy] => can be ordered(not ordred within %dsec)", s.config.BuyIntervalSeconds)
	return true
}

func (s *InagoStrategy) canAveragingDown(pair model.CurrencyPair, positions []model.Position) (bool, error) {
	buyRate, err := s.facade.GetBuyRate(&pair)
	if err != nil {
		return false, nil
	}

	minRate := buyRate * 1000
	for _, pos := range positions {
		cc, err := s.facade.GetContracts(pos.OpenerOrder.ID)
		if err != nil {
			return false, err
		}
		for _, c := range cc {
			minRate = math.Min(minRate, c.Rate)
		}
	}
	borderRate := minRate * s.config.AveragingDownRatePer

	if len(positions) == 0 {
		s.logger.Debug("[buy] => can averaging down (buyRate:%.3f, position min rate:nothing)", buyRate)
		return true, nil
	}
	if buyRate < borderRate {
		s.logger.Debug("[buy] => can averaging down (buyRate:%.3f < borderRate:%.3f, minRate:%.3f)", buyRate, borderRate, minRate)
		return true, nil
	}
	s.logger.Debug("[buy] => cannot averaging down (buyRate:%.3f >= borderRate:%.3f, minRate:%.3f)", buyRate, borderRate, minRate)
	return false, nil
}

func (s *InagoStrategy) supportLineCross(pair model.CurrencyPair, rates []float64, term1Period, term2Period int) (bool, error) {
	if len(rates) < term1Period+term2Period {
		s.logger.Debug("[buy] => skip check support line crossed (rate len:%d < required:%d)", len(rates), term1Period+term2Period)
		return false, nil
	}

	term1End := len(rates) - 1
	term1Begin := term1End - (term1Period - 1)
	term1Min, term1MinIdx := minRate(rates[term1Begin : term1End-1])
	term1MinIdx = term1MinIdx + term1Begin

	term2End := term1Begin - 1
	term2Begin := term2End - (term2Period - 1)
	term2Min, term2MinIdx := minRate(rates[term2Begin : term2End-1])

	slope := (term1Min - term2Min) / float64(term1MinIdx-term2MinIdx)

	supportLine := term1Min + slope*float64(len(rates)-1-term1MinIdx)
	sellRate, err := s.facade.GetSellRate(&pair)
	if err != nil {
		return false, err
	}

	if sellRate < supportLine {
		s.logger.Debug(
			"[buy] => support line crossed(sell:%.3f < support:%.3f)(min[%d]=%.3f)->(min[%d]=%.3f)",
			sellRate, supportLine,
			term2MinIdx, term2Min,
			term1MinIdx, term1Min,
		)
		return true, nil
	}
	s.logger.Debug(
		"[buy] => support line not crossed(sell:%.3f >= support:%.3f)(min[%d]=%.3f)->(min[%d]=%.3f)",
		sellRate, supportLine,
		term2MinIdx, term2Min,
		term1MinIdx, term1Min,
	)
	return false, nil
}

func maxRate(rates []float64) (float64, int) {
	max := rates[0]
	maxIndex := 0
	for i := 0; i < len(rates); i++ {
		rate := rates[i]
		if rate > max {
			max = rate
			maxIndex = i
		}
	}
	return max, maxIndex
}

func minRate(rates []float64) (float64, int) {
	min := rates[0]
	minIndex := 0
	for i := 0; i < len(rates); i++ {
		rate := rates[i]
		if rate < min {
			min = rate
			minIndex = i
		}
	}
	return min, minIndex
}

func (s *InagoStrategy) isGoldenCross(pair model.CurrencyPair, rates []float64, longTermSize, shortTermSize int) bool {
	longs := talib.Ema(rates, longTermSize)
	shorts := talib.Ema(rates, shortTermSize)

	beforeL := s.longEma
	beforeS := s.shortEma
	afterL := longs[len(longs)-1]
	afterS := shorts[len(shorts)-1]

	goldenCross := beforeL > beforeS && afterL < afterS
	s.longEma = afterL
	s.shortEma = afterS

	if !goldenCross {
		s.logger.Debug("[buy] => not golden cross (S:%.3f,L:%.3f,diff:%.3f)->(S:%.3f,L:%.3f,diff:%.3f)", beforeS, beforeL, beforeS-beforeL, afterS, afterL, afterS-afterL)
		return false
	}
	s.logger.Debug("[buy] => golden cross !!! (S:%.3f,L:%.3f,diff:%.3f)->(S:%.3f,L:%.3f,diff:%.3f)", beforeS, beforeL, beforeS-beforeL, afterS, afterL, afterS-afterL)
	return true
}

func (s *InagoStrategy) buy(p *model.CurrencyPair) error {
	balance, err := s.facade.GetJpyBalance()
	if err != nil {
		return err
	}
	amount := balance.Amount * s.config.FundsRatio

	s.logger.Debug("======================================")
	s.logger.Debug("[buy] sending buy order ...")
	pos, err := s.facade.SendMarketBuyOrder(p, amount, nil)
	if err != nil {
		return err
	}

	s.logger.Debug("[buy] completed to send buy order => [%v]", pos.OpenerOrder)
	s.logger.Debug("======================================")

	for {
		cc, err := s.facade.GetContracts(pos.OpenerOrder.ID)
		if err != nil {
			return err
		}

		if len(cc) > 0 {
			break
		}

		// 約定待ちのため1秒待機
		time.Sleep(1 * time.Second)
	}

	return err
}

func (s *InagoStrategy) BuyTradeCallback(p model.CurrencyPair, rate float64) error {
	// 取引数が一定数を超えたら売る準備
	v, err := s.facade.GetVolumes(&p, model.BuySide, time.Duration(s.config.VolumeCheckSeconds)*time.Second)
	if err != nil {
		return err
	}

	poss, err := s.facade.GetOpenPositions()
	if err != nil {
		return err
	}
	if len(poss) == 0 {
		s.logger.Debug("[sell] => not standby (no positions)(volume :%.3f, max:%.3f)(period:%dsec)", v, s.config.MaxVolume, s.config.VolumeCheckSeconds)
	} else if v > s.config.MaxVolume {
		s.logger.Debug("[sell] => standby (volume :%.3f > max:%.3f)(period:%dsec)", v, s.config.MaxVolume, s.config.VolumeCheckSeconds)
		s.sellStandby = true
	} else {
		s.logger.Debug("[sell] => not standby (volume :%.3f <= max:%.3f)(period:%dsec)", v, s.config.MaxVolume, s.config.VolumeCheckSeconds)
	}
	return nil
}

func (s *InagoStrategy) SellTradeCallback(pair model.CurrencyPair, rate float64) error {
	// レートが全ポジションより低くなったらスタンバイ解除
	pp, err := s.facade.GetOpenPositions()
	if err != nil {
		return err
	}
	if len(pp) == 0 {
		return nil
	}

	posRates := []float64{}
	for _, p := range pp {
		cc, err := s.facade.GetContracts(p.OpenerOrder.ID)
		if err != nil {
			return err
		}
		for _, c := range cc {
			if rate >= c.Rate {
				return nil
			}
			posRates = append(posRates, c.Rate)
		}
	}

	if s.sellStandby {
		s.logger.Debug("[sell] => release standby (sell rate :%.3f < position rate) (position rates:%v)", rate, posRates)
		s.sellStandby = false
	}

	return nil
}

func (s *InagoStrategy) Sell(pair model.CurrencyPair, positions []model.Position) error {
	if len(positions) == 0 {
		s.logger.Debug("[sell] => skip sell (open pos nothing)")
		return nil
	}

	buyJPY := 0.0
	currencyAmount := 0.0
	amounts := map[uint64]float64{}
	for _, p := range positions {
		cc, err := s.facade.GetContracts(p.OpenerOrder.ID)
		if err != nil {
			return err
		}

		amount := 0.0
		for _, c := range cc {
			buyJPY += (-1) * c.DecreaseAmount
			amount += c.IncreaseAmount
		}

		amounts[p.ID] = amount
		currencyAmount += amount
	}

	sellRate, err := s.facade.GetSellRate(&pair)
	if err != nil {
		return err
	}

	rates, err := s.facade.GetRates(&pair)
	if err != nil {
		return err
	}
	if len(rates) < s.config.SellROCPeriod {
		s.logger.Debug("[sell] => skip sell (rates len:%d < required:%d)", len(rates), s.config.SellROCPeriod)
		return nil
	}
	rocs := talib.Roc(rates, s.config.SellROCPeriod)
	roc := rocs[len(rocs)-1]

	sellJPY := sellRate * currencyAmount
	fixLimit := buyJPY * s.calcFixLimitRate(roc)
	losscutLimit := buyJPY * s.calcLosscutLimitRate(roc)

	skip := false
	if sellJPY >= fixLimit {
		s.logger.Debug("[sell] => fix profit (sell:%.3f >= fix:%.3f) (sellRate:%.3f, buyJPY:%.3f, buyRateAVG:%.3f)", sellJPY, fixLimit, sellRate, buyJPY, buyJPY/currencyAmount)
	} else if sellJPY <= losscutLimit {
		s.logger.Debug("[sell] => losscut (sell:%.3f <= losscut:%.3f) (sellRate:%.3f, buyJPY:%.3f, buyRateAVG:%.3f)", sellJPY, losscutLimit, sellRate, buyJPY, buyJPY/currencyAmount)
	} else {
		s.logger.Debug("[sell] => skip sell (losscut:%.3f < sell:%.3f < fix:%.3f) (sellRate:%.3f, buyJPY:%.3f, buyRateAVG:%.3f)", losscutLimit, sellJPY, fixLimit, sellRate, buyJPY, buyJPY/currencyAmount)
		skip = true
	}
	if skip {
		if !s.sellStandby && sellJPY > buyJPY {
			s.logger.Debug("[sell] => standby (sellJPY:%.3f > buyJPY:%.3f)", sellJPY, buyJPY)
			s.sellStandby = true
		}
		return nil
	}

	s.logger.Debug("======================================")
	for _, p := range positions {
		s.logger.Debug("[pos:%d][sell] sending sell order ...", p.ID)
		pos, err := s.facade.SendMarketSellOrder(&pair, amounts[p.ID], &p)
		if err != nil {
			return err
		}
		s.logger.Debug("[pos:%d][sell] completed to send sell order => [%v]", pos.ID, pos.CloserOrder)
	}
	s.logger.Debug("======================================")

	s.sellStandby = false

	return err
}

func (s *InagoStrategy) calcFixLimitRate(roc float64) float64 {
	return s.config.FixProfitUpperLimitPer
}

func (s *InagoStrategy) calcLosscutLimitRate(roc float64) float64 {
	return s.config.LossCutLowerLimitPer
}

func (s *InagoStrategy) Wait(ctx context.Context) error {
	s.logger.Debug("wait ... (%d sec)", s.config.Interval)

	waitCount := 0
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			if s.sellStandby && waitCount >= 1 {
				s.logger.Debug("stop wait (sell standby)")
				return nil
			}

			if waitCount >= s.config.Interval {
				return nil
			}

			if err := s.facade.Wait(ctx, 1*time.Second); err != nil {
				return err
			}
			waitCount++
		}
	}
}
