package strategy

import (
	"context"
	"fmt"
	"log"
	"time"
	"trading-bot/pkg/domain/model"
	"trading-bot/pkg/usecase/trade"

	"github.com/markcheno/go-talib"
)

const (
	// 買い注文1回に使う日本円残高の割合
	fundsRatio float32 = 0.3

	// 売り注文時のレート上乗せ分(%)
	// 売り注文レート = 買い注文のレート × (1 + upRate)
	upRate float32 = 0.005

	// 売り注文をロスカットする下限の割合
	// 現レートが下限を下回るとロスカットする
	// 下限 = 注文レート * 下限の割合
	lossCutLowerLimitPer float32 = 0.990

	// 約定チェック間隔
	contractCheckInterval = 2 * time.Second

	// 同時に持てるポジションの最大数
	positionCountMax = 1

	// 短期を確認する際の確認対象レート数
	shortTermSize = 5

	// 長期を確認する際の確認対象レート数
	longTermSize = 30
)

// FollowUptrendStrategy 上昇トレンド追従戦略
type FollowUptrendStrategy struct {
	facade   *trade.Facade
	analyzer *RateAnalyzer
}

// NewFollowUptrendStrategy 戦略を生成
func NewFollowUptrendStrategy(facade *trade.Facade) *FollowUptrendStrategy {
	return &FollowUptrendStrategy{
		facade: facade,
		analyzer: &RateAnalyzer{
			ShortTermSize: shortTermSize,
			LongTermSize:  longTermSize,
		},
	}
}

// Trade 取引実施
func (f *FollowUptrendStrategy) Trade(ctx context.Context) error {
	if err := f.facade.FetchAll(); err != nil {
		return err
	}

	buyRate, sellRate, err := f.getRate()
	if err != nil {
		return err
	}
	log.Printf("===== rate[%s] buy: %.3f sell: %.3f =====\n", f.facade.GetCurrencyPair(), buyRate, sellRate)

	pp, err := f.facade.GetOpenPositions()
	if err != nil {
		return err
	}

	if len(pp) < positionCountMax {
		log.Printf("[check] open poss [count: %d] < %d => check new order", len(pp), positionCountMax)
		if err := f.checkNewOrder(); err != nil {
			return err
		}
	} else {
		log.Printf("[check] open poss [count: %d] >= %d => skip new order", len(pp), positionCountMax)
	}

	for _, p := range pp {
		if err := f.checkPosition(&p); err != nil {
			return err
		}
	}

	return nil
}

func (f *FollowUptrendStrategy) checkNewOrder() error {
	if !f.analyzer.IsBuySignal(f.facade.GetSellRateHistory()) {
		log.Printf("[check] no buy signal => skip new order")
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
	amount := balance.Amount * fundsRatio

	log.Printf("[trade] sending buy order ...")
	pos1, err := f.facade.SendMarketBuyOrder(amount, nil)
	if err != nil {
		return err
	}
	log.Printf("[trade] completed to send buy order [%v]", pos1.OpenerOrder)

	if _, err := f.waitContract(pos1.OpenerOrder.ID); err != nil {
		return err
	}

	log.Printf("[trade] sending sell order ...")
	//pos2, err := f.sendSellOrder(pos1)
	pos2, err := f.sendSellOrder(pos1)
	if err != nil {
		return err
	}
	log.Printf("[trade] completed to send sell order [%v]", *pos2.CloserOrder)

	return nil
}

func (f *FollowUptrendStrategy) checkPosition(pos *model.Position) error {
	if pos.OpenerOrder.Status == model.Open {
		log.Printf("[check] position[id:%d]: OpenerOrder is OPEN => wait for contract new order[%s rate:%s amount:%.5f] ...",
			pos.ID,
			pos.OpenerOrder.Type,
			toDisplayStr(pos.OpenerOrder.Rate, "--"),
			pos.OpenerOrder.Amount)
		return nil
	}
	if pos.CloserOrder == nil {
		log.Printf("[check] position[id:%d]: Closer Order is nothing => sending sell order ...", pos.ID)
		pos2, err := f.sendSellOrder(pos)
		if err != nil {
			return err
		}
		log.Printf("[trade] completed to send order [%s rate:%s amount:%.5f]",
			pos2.CloserOrder.Type,
			toDisplayStr(pos2.CloserOrder.Rate, "--"),
			pos2.CloserOrder.Amount,
		)
		return nil
	}

	if pos.CloserOrder.Status == model.Open {
		log.Printf("[check] position[id:%d]: Closer Order is OPEN => check for resend order[%s rate:%s amount:%.5f] ...",
			pos.ID,
			pos.CloserOrder.Type,
			toDisplayStr(pos.CloserOrder.Rate, "--"),
			pos.CloserOrder.Amount,
		)

		shouldLossCut := f.analyzer.ShouldLossCut(f.facade.GetSellRateHistory(), pos.CloserOrder)
		if shouldLossCut {
			log.Printf("[trade] sending cancel order ...")
			pos2, err := f.facade.CancelSettleOrder(pos)
			if err != nil {
				return err
			}
			log.Printf("[trade] completed to send cancel order[order_id:%d]", pos.CloserOrder.ID)

			log.Printf("[trade] sending loss cut sell order ...")
			pos3, err := f.sendLossCutSellOrder(pos2)
			if err != nil {
				return err
			}
			log.Printf("[trade] completed to send loss cut sell order[%s rate:%s amount:%.5f]",
				pos3.CloserOrder.Type,
				toDisplayStr(pos.CloserOrder.Rate, "--"),
				pos.CloserOrder.Amount)
		}
	}

	return nil
}

func (f *FollowUptrendStrategy) getRate() (buyRate, sellRate float32, err error) {
	b, err := f.facade.GetBuyRate()
	if err != nil {
		return 0, 0, nil
	}
	s, err := f.facade.GetSellRate()
	if err != nil {
		return 0, 0, nil
	}

	return b, s, nil
}

func (f *FollowUptrendStrategy) isUptrend() bool {
	// 一定期間の間の売レートの上昇回数が半分を超えてたら上昇トレンドと判断
	count := 0
	rates := f.facade.GetSellRateHistory()
	size := len(rates)
	for i := 1; i < size; i++ {
		if rates[i-1] < rates[i] {
			count++
		}
	}

	if count > ((size - 1) / 2) {
		log.Printf("[check] rise count: %d / %d => UP trend", count, size-1)
		return true
	}
	log.Printf("[check] rise count: %d / %d => not UP trend", count, size-1)
	return false
}

// func (f *FollowUptrendStrategy) sendBuyOrder() (*model.Position, error) {
// 	return f.facade.SendMarketBuyOrder(fundJpy, nil)
// }

func (f *FollowUptrendStrategy) waitContract(orderID uint64) ([]model.Contract, error) {
	log.Printf("[trade] waiting for contract ...")
	for {
		if err := f.facade.FetchAll(); err != nil {
			return nil, err
		}

		order, err := f.facade.GetOrder(orderID)
		if err != nil {
			return nil, err
		}
		if order != nil && order.Status == model.Closed {
			break
		}

		log.Printf("[trade] not contracted, waiting for contract ...")
		time.Sleep(contractCheckInterval)
	}
	contracts, err := f.facade.GetContracts(orderID)
	if err != nil {
		return nil, err
	}

	log.Printf("[trade] contracted!!! [%v]", contracts)
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

	return f.facade.SendSellOrder(amount, rate*(1.0+upRate), p)
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

	return f.facade.SendMarketSellOrder(amount, p)
}

func toDisplayStr(v *float32, def string) string {
	if v == nil {
		return def
	}
	return fmt.Sprintf("%.3f", *v)
}

// RateAnalyzer レート分析
type RateAnalyzer struct {
	ShortTermSize int
	LongTermSize  int
}

// IsBuySignal 買いシグナルかを判定
func (a *RateAnalyzer) IsBuySignal(rates []float32) bool {
	// レート情報が少ないときは判断不可
	if len(rates) < a.LongTermSize {
		log.Printf("[check] buy signal, rate count: count:%d < required:%d => not buy signal", len(rates), a.LongTermSize)
		return false
	}

	rr := []float64{}
	for _, r := range rates {
		rr = append(rr, float64(r))
	}

	sRates := talib.Ema(rr, a.ShortTermSize)
	sRate := sRates[len(sRates)-1]
	lRates := talib.Ema(rr, a.LongTermSize)
	lRate := lRates[len(lRates)-1]

	// 下降トレンド（短期の移動平均＜長期の移動平均）
	if sRate < lRate {
		log.Printf("[check] SMA short:%.3f < long:%.3f => not UP trend", sRate, lRate)
		return false
	}

	// 上昇続いてる？
	count := 0
	size := len(rates)
	begin := size - a.LongTermSize
	end := size - 1
	for i := begin + 1; i <= end; i++ {
		if rates[i-1] < rates[i] {
			count++
		}
	}
	if count < (a.LongTermSize / 2) {
		log.Printf("[check] rise count: %d / %d => not UP trend", count, a.LongTermSize)
		return false
	}
	log.Printf("[check] rise count: %d / %d => UP trend", count, a.LongTermSize)
	return true
}

// ShouldLossCut ロスカットすべきか判定
func (a *RateAnalyzer) ShouldLossCut(rates []float32, sellOrder *model.Order) bool {
	// レート情報が少ないときは判断不可
	if len(rates) < a.LongTermSize {
		log.Printf("[check] buy signal, rate count: count:%d < required:%d => not buy signal", len(rates), a.LongTermSize)
		return false
	}

	rr := []float64{}
	for _, r := range rates {
		rr = append(rr, float64(r))
	}

	sRates := talib.Ema(rr, a.ShortTermSize)
	sRate := sRates[len(sRates)-1]
	lRates := talib.Ema(rr, a.LongTermSize)
	lRate := lRates[len(lRates)-1]

	// 上昇トレンドになりそうなら待機
	if sRate >= lRate {
		log.Printf("[check] SMA short:%.3f >= long:%.3f => UP trend => skip loss cut", sRate, lRate)
		return false
	}

	if sellOrder.Rate == nil {
		return false
	}
	// 下限を下回ったらロスカット
	currentRate := rates[len(rates)-1]
	lowerLimit := *sellOrder.Rate * lossCutLowerLimitPer
	if lowerLimit <= currentRate {
		log.Printf(
			"[check] order[rate:%.3f] * %.3f = lowerLimit:%.3f <= %s[rate:%.3f] => skip loss cut\n",
			*sellOrder.Rate,
			lossCutLowerLimitPer,
			lowerLimit,
			sellOrder.Pair.String(),
			currentRate)
		return false
	}
	log.Printf(
		"[check] order[rate:%.3f] * %.3f = lowerLimit:%.3f > %s[rate:%.3f] => should loss cut\n",
		*sellOrder.Rate,
		lossCutLowerLimitPer,
		lowerLimit,
		sellOrder.Pair.String(),
		currentRate)
	return true
}
