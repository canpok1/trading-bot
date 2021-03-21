package trade

import (
	"time"
	"trading-bot/pkg/domain/exchange"
	"trading-bot/pkg/domain/model"
	"trading-bot/pkg/domain/repository"
)

// Facade トレード操作をまとめたもの
type Facade struct {
	exClient     exchange.Client
	rateRepo     repository.RateRepository
	orderRepo    repository.OrderRepository
	contractRepo repository.ContractRepository
	positionRepo repository.PositionRepository
	rateDuration *time.Duration
}

// NewFacade 生成
func NewFacade(
	exCli exchange.Client,
	rateRepo repository.RateRepository,
	orderRepo repository.OrderRepository,
	contractRepo repository.ContractRepository,
	positionRepo repository.PositionRepository,
	rateDuration *time.Duration,
) *Facade {
	return &Facade{
		exClient:     exCli,
		rateRepo:     rateRepo,
		orderRepo:    orderRepo,
		contractRepo: contractRepo,
		positionRepo: positionRepo,
		rateDuration: rateDuration,
	}
}

// getOrderRate レートを取得
//func (f *Facade) getOrderRate(pair *model.CurrencyPair, side model.OrderSide) (float64, error) {
//	if rate := f.rateRepo.GetCurrentRate(&pair.Key, side); rate != nil {
//		return *rate, nil
//	}
//
//	rate, err := f.exClient.GetOrderRate(pair, side)
//	if err != nil {
//		return 0, err
//	}
//
//	return rate.Rate, nil
//}

// GetRate レートを取得
func (f *Facade) GetRate(p *model.CurrencyPair) (float64, error) {
	return f.rateRepo.GetRate(p)
}

// GetRates レートを取得
func (f *Facade) GetRates(p *model.CurrencyPair) ([]float64, error) {
	return f.rateRepo.GetRates(p, f.rateDuration)
}

// // GetBuyRate 買レートを取得
// func (f *Facade) GetBuyRate(pair *model.CurrencyPair) (float64, error) {
// 	return f.getOrderRate(pair, model.BuySide)
// }
//
// // GetSellRate 売レートを取得
// func (f *Facade) GetSellRate(pair *model.CurrencyPair) (float64, error) {
// 	return f.getOrderRate(pair, model.SellSide)
// }

// // GetBuyRateHistory 買レートの遷移を取得
// func (f *Facade) GetBuyRateHistory(pair *model.CurrencyPair) []float64 {
// 	return f.rateRepo.GetRateHistory(&pair.Key, model.BuySide)
// }
//
// // GetSellRateHistory 売レートの遷移を取得
// func (f *Facade) GetSellRateHistory(pair *model.CurrencyPair) []float64 {
// 	return f.rateRepo.GetRateHistory(&pair.Key, model.SellSide)
// }
//
// // GetSellRateHistory64 売レートの遷移を取得
// func (f *Facade) GetSellRateHistory64(pair *model.CurrencyPair) []float64 {
// 	rates := f.rateRepo.GetRateHistory(&pair.Key, model.SellSide)
//
// 	rr := []float64{}
// 	for _, r := range rates {
// 		rr = append(rr, float64(r))
// 	}
//
// 	return rr
// }

// GetOpenPositions オープン状態のポジションを取得
func (f *Facade) GetOpenPositions() ([]model.Position, error) {
	return f.positionRepo.GetOpenPositions()
}

// GetContracts 約定情報を取得
func (f *Facade) GetContracts(orderID uint64) ([]model.Contract, error) {
	return f.contractRepo.GetContracts(orderID)
}

// GetOrder 注文情報を取得
func (f *Facade) GetOrder(orderID uint64) (*model.Order, error) {
	return f.orderRepo.GetOrder(orderID)
}

// SendMarketBuyOrder 成行買い注文
func (f *Facade) SendMarketBuyOrder(pair *model.CurrencyPair, amount float64, p *model.Position) (*model.Position, error) {
	return f.postOrder(&model.NewOrder{
		Type:            model.MarketBuy,
		Pair:            *pair,
		MarketBuyAmount: &amount,
	}, p)
}

// SendMarketSellOrder 成行売り注文
func (f *Facade) SendMarketSellOrder(pair *model.CurrencyPair, amount float64, p *model.Position) (*model.Position, error) {
	return f.postOrder(&model.NewOrder{
		Type:            model.MarketSell,
		Pair:            *pair,
		Amount:          &amount,
		Rate:            nil,
		MarketBuyAmount: nil,
		StopLossRate:    nil,
	}, p)
}

// SendSellOrder 売り注文
func (f *Facade) SendSellOrder(pair *model.CurrencyPair, amount float64, rate float64, p *model.Position) (*model.Position, error) {
	return f.postOrder(&model.NewOrder{
		Type:   model.Sell,
		Pair:   *pair,
		Amount: &amount,
		Rate:   &rate,
	}, p)
}

// postOrder 注文
func (f *Facade) postOrder(o *model.NewOrder, p *model.Position) (*model.Position, error) {
	order, err := f.exClient.PostOrder(o)
	if err != nil {
		return nil, err
	}

	if p == nil {
		return f.positionRepo.AddNewOrder(order)
	}
	return f.positionRepo.AddSettleOrder(p.ID, order)
}

// CancelSettleOrder 注文キャンセル
func (f *Facade) CancelSettleOrder(p *model.Position) (*model.Position, error) {
	if err := f.exClient.DeleteOrder(p.CloserOrder.ID); err != nil {
		return nil, err
	}

	return f.positionRepo.CancelSettleOrder(p.ID)
}

// // GetRateHistorySizeMax レート履歴の最大容量を取得
// func (f *Facade) GetRateHistorySizeMax() int {
// 	return f.rateRepo.GetHistorySizeMax()
// }

// GetJpyBalance 日本円の残高を取得
func (f *Facade) GetJpyBalance() (*model.Balance, error) {
	c := model.JPY
	return f.exClient.GetBalance(&c)
}
