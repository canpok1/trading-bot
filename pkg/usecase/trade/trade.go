package trade

import (
	"trading-bot/pkg/domain/exchange"
	"trading-bot/pkg/domain/model"
	"trading-bot/pkg/domain/repository"
)

// Facade トレード操作をまとめたもの
type Facade struct {
	pair         *model.CurrencyPair
	exClient     exchange.Client
	rateRepo     repository.RateRepository
	orderRepo    repository.OrderRepository
	contractRepo repository.ContractRepository
	positionRepo repository.PositionRepository
}

// NewFacade 生成
func NewFacade(
	p *model.CurrencyPair,
	exCli exchange.Client,
	rateRepo repository.RateRepository,
	orderRepo repository.OrderRepository,
	contractRepo repository.ContractRepository,
	positionRepo repository.PositionRepository,
) *Facade {
	return &Facade{
		pair:         p,
		exClient:     exCli,
		rateRepo:     rateRepo,
		orderRepo:    orderRepo,
		contractRepo: contractRepo,
		positionRepo: positionRepo,
	}
}

// FetchAll 情報更新
func (f *Facade) FetchAll() error {
	if err := f.FetchRate(); err != nil {
		return err
	}

	if err := f.FetchContracts(); err != nil {
		return err
	}

	if err := f.FetchOrders(); err != nil {
		return err
	}

	return nil
}

// FetchRate レートを更新
func (f *Facade) FetchRate() error {
	buyRate, err := f.exClient.GetOrderRate(f.pair, model.BuySide)
	if err != nil {
		return err
	}
	if err := f.rateRepo.AddOrderRate(buyRate); err != nil {
		return err
	}

	sellRate, err := f.exClient.GetOrderRate(f.pair, model.SellSide)
	if err != nil {
		return err
	}
	if err := f.rateRepo.AddOrderRate(sellRate); err != nil {
		return err
	}

	return nil
}

//FetchContracts 約定情報を更新
func (f *Facade) FetchContracts() error {
	oo, err := f.orderRepo.GetOpenOrders()
	if err != nil {
		return err
	}
	if len(oo) == 0 {
		return nil
	}

	cc, err := f.exClient.GetContracts()
	if err != nil {
		return err
	}

	targets := []model.Contract{}
	for _, c := range cc {
		for _, o := range oo {
			if c.OrderID == o.ID {
				targets = append(targets, c)
				break
			}
		}
	}

	if err := f.contractRepo.UpsertContracts(targets); err != nil {
		return err
	}

	return nil
}

// FetchOrders 注文情報を更新
func (f *Facade) FetchOrders() error {
	openOrders, err := f.exClient.GetOpenOrders(f.pair)
	if err != nil {
		return err
	}

	registeredOrders, err := f.orderRepo.GetOpenOrders()
	if err != nil {
		return err
	}

	for _, o := range registeredOrders {
		opened := false
		for _, openOrder := range openOrders {
			if openOrder.ID == o.ID {
				opened = true
				break
			}
		}

		if opened {
			continue
		}
		if err := f.orderRepo.UpdateStatus(o.ID, model.Closed); err != nil {
			return err
		}
	}

	return nil
}

// GetCurrencyPair 通貨ペアを取得
func (f *Facade) GetCurrencyPair() *model.CurrencyPair {
	return f.pair
}

// getOrderRate レートを取得
func (f *Facade) getOrderRate(side model.OrderSide) (float32, error) {
	if rate := f.rateRepo.GetCurrentRate(&f.pair.Key, side); rate != nil {
		return *rate, nil
	}

	rate, err := f.exClient.GetOrderRate(f.pair, side)
	if err != nil {
		return 0, err
	}

	return rate.Rate, nil
}

// GetBuyRate 買レートを取得
func (f *Facade) GetBuyRate() (float32, error) {
	return f.getOrderRate(model.BuySide)
}

// GetSellRate 売レートを取得
func (f *Facade) GetSellRate() (float32, error) {
	return f.getOrderRate(model.SellSide)
}

// GetBuyRateHistory 買レートの遷移を取得
func (f *Facade) GetBuyRateHistory() []float32 {
	return f.rateRepo.GetRateHistory(&f.pair.Key, model.BuySide)
}

// GetSellRateHistory 売レートの遷移を取得
func (f *Facade) GetSellRateHistory() []float32 {
	return f.rateRepo.GetRateHistory(&f.pair.Key, model.SellSide)
}

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
func (f *Facade) SendMarketBuyOrder(amount float32, p *model.Position) (*model.Position, error) {
	return f.postOrder(&model.NewOrder{
		Type:            model.MarketBuy,
		Pair:            *f.GetCurrencyPair(),
		MarketBuyAmount: &amount,
	}, p)
}

// SendSellOrder 売り注文
func (f *Facade) SendSellOrder(amount float32, rate float32, p *model.Position) (*model.Position, error) {
	return f.postOrder(&model.NewOrder{
		Type:   model.Sell,
		Pair:   *f.GetCurrencyPair(),
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
func (f *Facade) CancelSettleOrder(orderID uint64) (*model.Position, error) {
	if err := f.exClient.DeleteOrder(orderID); err != nil {
		return nil, err
	}

	return f.positionRepo.CancelSettleOrder(orderID)
}

// GetRateHistorySizeMax レート履歴の最大容量を取得
func (f *Facade) GetRateHistorySizeMax() int {
	return f.rateRepo.GetHistorySizeMax()
}
