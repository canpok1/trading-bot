package strategy

import (
	"context"
	"fmt"
	"log"
	"time"
	"trading-bot/pkg/domain/model"
	"trading-bot/pkg/usecase/trade"
)

const (
	// 買い注文1回に使う金額(JPY)
	fundJpy float32 = 500

	// 売り注文時のレート上乗せ分(%)
	// 売り注文レート = 買い注文のレート × (1 + upRate)
	upRate float32 = 0.01

	// キャンセルの基準値(%)
	// 現レートと指値との差分が基準値以上ならキャンセルする
	// 差分 = (売指レート - 現レート) / 現レート
	cancelBorderPer float32 = 0.10

	// 約定チェック間隔
	contractCheckInterval = 2 * time.Second

	// 同時に持てるポジションの最大数
	positionCountMax = 1
)

// FollowUptrendStrategy 上昇トレンド追従戦略
type FollowUptrendStrategy struct {
	facade *trade.Facade
}

// NewFollowUptrendStrategy 戦略を生成
func NewFollowUptrendStrategy(facade *trade.Facade) *FollowUptrendStrategy {
	return &FollowUptrendStrategy{
		facade: facade,
	}
}

// Trade 取引実施
func (f *FollowUptrendStrategy) Trade(ctx context.Context) error {
	if err := f.facade.FetchAll(); err != nil {
		return err
	}
	if f.isWarmingUp() {
		log.Println("warming up ...")
		return nil
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
	if !f.isUptrend() {
		log.Printf("[check] current trend is not UP => skip new order")
		return nil
	}

	log.Printf("[trade] sending buy order ...")
	pos1, err := f.facade.SendMarketBuyOrder(fundJpy, nil)
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

		shouldLossCut, err := f.shouldLossCut(pos.CloserOrder)
		if err != nil {
			return err
		}

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

func (f *FollowUptrendStrategy) isWarmingUp() bool {
	return len(f.facade.GetSellRateHistory()) < f.facade.GetRateHistorySizeMax()
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

func (f *FollowUptrendStrategy) shouldLossCut(sellOrder *model.Order) (bool, error) {
	// 指値までの差が現在レートから離れすぎてたら損切り
	currentRate, err := f.facade.GetSellRate()
	if err != nil {
		return false, err
	}

	pair := f.facade.GetCurrencyPair().String()
	diff := (*sellOrder.Rate - currentRate) / currentRate
	if diff > cancelBorderPer {
		log.Printf("[check] order[rate:%.3f] %s[%.3f] diff: %.3f > border: %.3f => should loss cut\n", *sellOrder.Rate, pair, currentRate, diff, cancelBorderPer)
		return true, nil
	}
	log.Printf("[check] order[rate:%.3f] %s[%.3f] diff: %.3f > border: %.3f => skip loss cut\n", *sellOrder.Rate, pair, currentRate, diff, cancelBorderPer)
	return false, nil
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
