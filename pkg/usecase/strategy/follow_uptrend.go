package strategy

import (
	"context"
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
	upRate float32 = 0.005

	// キャンセルの基準値(%)
	// 現レートと指値との差分が基準値以上ならキャンセルする
	// 差分 = (売指レート - 現レート) / 現レート
	cancelBorderPer float32 = 0.05

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
	log.Printf("open poss [count: %d]", len(pp))

	if len(pp) < positionCountMax {
		if err := f.checkNewOrder(); err != nil {
			return err
		}
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
		log.Printf("current trend is not UP => skip")
		return nil
	}
	log.Printf("current trend is UP => start trade")

	log.Printf("sending buy order ...")
	pos1, err := f.sendBuyOrder()
	if err != nil {
		return err
	}
	log.Printf("completed to send buy order [%v]", pos1.OpenerOrder)

	if _, err := f.waitContract(pos1.OpenerOrder.ID); err != nil {
		return err
	}

	log.Printf("sending sell order ...")
	pos2, err := f.sendSellOrder(pos1)
	if err != nil {
		return err
	}
	log.Printf("completed to send sell order [%v]", *pos2.CloserOrder)

	return nil
}

func (f *FollowUptrendStrategy) checkPosition(pos *model.Position) error {
	if pos.OpenerOrder.Status == model.Open {
		log.Printf("%v => wait for contract new order...", pos)
		return nil
	}
	if pos.CloserOrder == nil {
		log.Printf("%v => sending sell order ...", pos)
		pos2, err := f.sendSellOrder(pos)
		if err != nil {
			return err
		}
		log.Printf("completed to send sell order [%v]", *pos2.CloserOrder)
		return nil
	}

	if pos.CloserOrder.Status == model.Open {
		shouldReorder, err := f.shouldReorder(pos.CloserOrder)
		if err != nil {
			return err
		}

		if shouldReorder {
			log.Printf("%v => sending re order ...", pos)

			log.Printf("sending cancel order ...")
			pos2, err := f.facade.CancelSettleOrder(pos.CloserOrder.ID)
			if err != nil {
				return err
			}
			log.Printf("completed to send cancel order [order_id:%d]", pos2.CloserOrder.ID)

			log.Printf("sending sell order ...")
			pos3, err := f.sendSellOrder(pos)
			if err != nil {
				return err
			}
			log.Printf("completed to send sell order [%v]", *pos3)
		} else {
			log.Printf("%v => wait for contract ...", pos)
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

	log.Printf("check rise count: %d / %d", count, size-1)

	return count > ((size - 1) / 2)
}

func (f *FollowUptrendStrategy) sendBuyOrder() (*model.Position, error) {
	return f.facade.SendMarketBuyOrder(fundJpy, nil)
}

func (f *FollowUptrendStrategy) waitContract(orderID uint64) ([]model.Contract, error) {
	log.Printf("waiting for contract ...")
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

		log.Printf("not contracted, waiting for contract ...")
		time.Sleep(contractCheckInterval)
	}
	contracts, err := f.facade.GetContracts(orderID)
	if err != nil {
		return nil, err
	}

	log.Printf("contracted!!! [%v]", contracts)
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

func (f *FollowUptrendStrategy) shouldReorder(sellOrder *model.Order) (bool, error) {
	// 指値までの差が現在レートから離れすぎてたら再注文
	currentRate, err := f.facade.GetSellRate()
	if err != nil {
		return false, err
	}

	pair := f.facade.GetCurrencyPair().String()
	diff := (*sellOrder.Rate - currentRate) / currentRate
	if diff > cancelBorderPer {
		log.Printf("%s rate: [current: %.3f, order: %.3f], diff: %.3f > border: %.3f => should reorder\n", pair, currentRate, *sellOrder.Rate, diff, cancelBorderPer)
		return true, nil
	}
	log.Printf("%s rate: [current: %.3f, order: %.3f], diff: %.3f =< border: %.3f => should not reorder\n", pair, currentRate, *sellOrder.Rate, diff, cancelBorderPer)
	return false, nil
}
