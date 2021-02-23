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
	// 売り注文レート = 買い注文の約定レート × (1 + upRate)
	upRate float32 = 0.005

	// キャンセルの基準値(%)
	// 現レートと指値との差分が基準値以上ならキャンセルする
	// 差分 = (売指レート - 現レート) / 現レート
	cancelBorderPer float32 = 0.05

	// 約定チェック間隔
	contractCheckInterval = 5 * time.Second
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

	orders, err := f.facade.GetOpenOrders()
	if err != nil {
		return err
	}
	log.Printf("open order [count: %d]", len(orders))

	if len(orders) == 0 {
		if !f.isUptrend() {
			log.Printf("current trend is not UP => skip")
			return nil
		}
		log.Printf("current trend is UP => start trade")

		log.Printf("sending buy order ...")
		buyOrder, err := f.sendBuyOrder()
		if err != nil {
			return err
		}
		log.Printf("completed to send buy order [%#v]", *buyOrder)

		log.Printf("waiting for contract ...")
		contracts, err := f.waitContract(buyOrder.ID)
		if err != nil {
			return err
		}
		log.Printf("contracted!!! [%#v]", contracts)

		for _, contract := range contracts {
			log.Printf("sending sell order ...")
			sellOrder, err := f.sendSellOrder(&contract)
			if err != nil {
				return err
			}
			log.Printf("completed to send sell order [%#v]", *sellOrder)
		}

		log.Printf("end trade")
		return nil
	}

	for _, order := range orders {
		shouldReorder, err := f.shouldReorder(&order)
		if err != nil {
			return err
		}
		if shouldReorder {
			log.Printf("open order[id:%d %s %s rate:%.3f amount:%.3f] => should reorder", order.ID, order.Type, order.Pair.String(), *order.Rate, order.Amount)

			log.Printf("sending cancel order ...")
			err := f.facade.CancelOrder(order.ID)
			if err != nil {
				return err
			}
			log.Printf("completed to send cancel order [order_id:%d]", order.ID)

			log.Printf("sending sell order ...")
			newOrder, err := f.resendSellOrder(&order)
			if err != nil {
				return err
			}
			log.Printf("completed to send sell order [%#v]", *newOrder)
		} else {
			log.Printf("open order[id:%d %s %s rate:%.3f amount:%.3f] => wait for contract", order.ID, order.Type, order.Pair.String(), *order.Rate, order.Amount)
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

	log.Printf("check rise count: %d / %d", count, size)

	return count > (size / 2)
}

func (f *FollowUptrendStrategy) sendBuyOrder() (*model.Order, error) {
	amount := fundJpy
	return f.facade.PostOrder(&model.NewOrder{
		Type:            model.MarketBuy,
		Pair:            *f.facade.GetCurrencyPair(),
		MarketBuyAmount: &amount,
	})
}

func (f *FollowUptrendStrategy) waitContract(orderID uint64) ([]model.Contract, error) {
	for {
		if err := f.facade.FetchAll(); err != nil {
			return nil, err
		}

		contracts, err := f.facade.GetContracts(orderID)
		if err != nil {
			return nil, err
		}
		if len(contracts) > 0 {
			return contracts, nil
		}

		time.Sleep(contractCheckInterval)
	}
}

func (f *FollowUptrendStrategy) sendSellOrder(c *model.Contract) (*model.Order, error) {
	rate := c.Rate * (1.0 + upRate)
	return f.facade.PostOrder(&model.NewOrder{
		Type:   model.Sell,
		Pair:   *f.facade.GetCurrencyPair(),
		Amount: &c.IncreaseAmount,
		Rate:   &rate,
	})
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

func (f *FollowUptrendStrategy) resendSellOrder(sellOrder *model.Order) (*model.Order, error) {
	// 現レートを基準に売り注文を出し直す
	currentRate, err := f.facade.GetSellRate()
	if err != nil {
		return nil, err
	}
	rate := currentRate * (1.0 + upRate)
	return f.facade.PostOrder(&model.NewOrder{
		Type:   model.Sell,
		Pair:   *f.facade.GetCurrencyPair(),
		Amount: &sellOrder.Amount,
		Rate:   &rate,
	})
}
