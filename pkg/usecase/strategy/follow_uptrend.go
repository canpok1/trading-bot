package strategy

import (
	"context"
	"log"
	"time"
	ex "trading-bot/pkg/domain/exchange"
	"trading-bot/pkg/domain/model"
	repo "trading-bot/pkg/domain/repository"
)

const (
	// 買い注文1回に使う金額(JPY)
	fundJpy float32 = 500

	// 売り注文時のレート上乗せ分(%)
	// 売り注文レート = 買い注文の約定レート × (1 + upRate)
	upRate float32 = 0.01

	// キャンセルの基準値(%)
	// 現レートと指値との差分が基準値以上ならキャンセルする
	// 差分 = (売指レート - 現レート) / 現レート
	cancelBorderPer float32 = 0.05

	// 約定チェック間隔
	contractCheckInterval = 5 * time.Second
)

// FollowUptrendStrategy 上昇トレンド追従戦略
type FollowUptrendStrategy struct {
	ExClient     ex.Client
	OrderRepo    repo.OrderRepository
	RateRepo     repo.RateRepository
	CurrencyPair *model.CurrencyPair
}

// NewFollowUptrendStrategy 戦略を生成
func NewFollowUptrendStrategy(exClient ex.Client, orderRepo repo.OrderRepository, rateRepo repo.RateRepository, p *model.CurrencyPair) *FollowUptrendStrategy {
	return &FollowUptrendStrategy{
		ExClient:     exClient,
		OrderRepo:    orderRepo,
		RateRepo:     rateRepo,
		CurrencyPair: p,
	}
}

// Trade 取引実施
func (f *FollowUptrendStrategy) Trade(ctx context.Context) error {

	log.Printf("get open order ...")
	orders, err := f.ExClient.GetOpenOrders()
	if err != nil {
		return err
	}
	log.Printf("completed to get open order [count: %d]", len(orders))

	if len(orders) == 0 {
		if !f.isUpTrend() {
			log.Printf("current trend is not UP => skip")
			return nil
		}
		log.Printf("current trend is UP => start trade")

		log.Printf("sending buy order ...")
		buyOrder, err := f.sendBuyOrder()
		if err != nil {
			return err
		}
		log.Printf("completed to send buy order [%v]", *buyOrder)

		log.Printf("waiting for contract ...")
		contract, err := f.waitContract(buyOrder.ID)
		if err != nil {
			return err
		}
		log.Printf("contracted!!! [%v]", *contract)

		log.Printf("sending sell order ...")
		sellOrder, err := f.sendSellOrder(contract)
		if err != nil {
			return err
		}
		log.Printf("completed to send sell order [%v]", *sellOrder)

		log.Printf("end trade")
		return nil
	}

	for _, order := range orders {
		if f.shouldCancel(&order) {
			log.Printf("order[id:%d] => cancel", order.ID)
			if err := f.cancel(&order); err != nil {
				return err
			}
		} else {
			log.Printf("order[id:%d] => wait for contract", order.ID)
		}
	}

	return nil
}

func (f *FollowUptrendStrategy) isUpTrend() bool {
	// 一定期間の間の上昇回数が半分を超えてたら上昇トレンドと判断
	count := 0
	rates := f.RateRepo.GetRateHistory(&f.CurrencyPair.Key)
	size := len(rates)
	for i := 1; i < size; i++ {
		if rates[i-1].Rate < rates[i].Rate {
			count++
		}
	}

	log.Printf("check upper count: %d / %d", count, size)

	return count > (size / 2)
}

func (f *FollowUptrendStrategy) sendBuyOrder() (*model.Order, error) {
	amount := fundJpy
	return f.ExClient.PostOrder(&model.NewOrder{
		Type:            model.MarketBuy,
		Pair:            *f.CurrencyPair,
		MarketBuyAmount: &amount,
	})
}

func (f *FollowUptrendStrategy) waitContract(orderID uint64) (*model.Contract, error) {
	for {
		contracts, err := f.ExClient.GetContracts()
		if err != nil {
			return nil, err
		}
		for _, c := range contracts {
			if c.OrderID == orderID {
				return &c, nil
			}
		}

		time.Sleep(contractCheckInterval)
	}
}

func (f *FollowUptrendStrategy) sendSellOrder(c *model.Contract) (*model.Order, error) {
	rate := c.IncreaseAmount * (1.0 + upRate)
	return f.ExClient.PostOrder(&model.NewOrder{
		Type:   model.Sell,
		Pair:   *f.CurrencyPair,
		Amount: &c.IncreaseAmount,
		Rate:   &rate,
	})
}

func (f *FollowUptrendStrategy) shouldCancel(sellOrder *model.Order) bool {
	// 指値までの差が現在レートから離れすぎてたらキャンセル
	current := f.RateRepo.GetCurrentRate(&f.CurrencyPair.Key)
	diff := (*sellOrder.Rate - current.Rate) / current.Rate
	return diff > cancelBorderPer
}

func (f *FollowUptrendStrategy) cancel(o *model.Order) error {
	return f.ExClient.DeleteOrder(o.ID)
}
