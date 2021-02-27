package memory

import "trading-bot/pkg/domain/model"

// Rate レート
type Rate struct {
	StoreRate     float32
	OrderBuyRate  float32
	OrderSellRate float32
}

// ExchangeMock 取引所モック
type ExchangeMock struct {
	pair        model.CurrencyPair
	rates       []Rate
	currentStep int
	orders      []model.Order
	contracts   []model.Contract
}

// NewExchangeMock 生成
func NewExchangeMock() *ExchangeMock {
	rates := []Rate{
		{100, 100, 100},
		{101, 101, 101},
		{102, 102, 102},
		{103, 103, 103},
		{104, 104, 104},
		{105, 105, 105},
		{106, 106, 106},
		{107, 107, 107},
		{108, 108, 108},
		{109, 109, 109},
		{110, 110, 110},
		{111, 111, 111},
		{112, 112, 112},
	}
	return &ExchangeMock{
		pair:        model.MonaJpy,
		rates:       rates,
		currentStep: 0,
		orders:      []model.Order{},
		contracts:   []model.Contract{},
	}
}

// GetStoreRate 販売所のレートを取得
func (e *ExchangeMock) GetStoreRate(p *model.CurrencyPair) (*model.StoreRate, error) {
	return &model.StoreRate{
		Pair: *p,
		Rate: e.rates[e.currentStep].StoreRate,
	}, nil
}

// GetOrderRate 取引所のレートを取得
func (e *ExchangeMock) GetOrderRate(p *model.CurrencyPair, side model.OrderSide) (*model.OrderRate, error) {
	rate := e.rates[e.currentStep]
	if side == model.BuySide {
		return &model.OrderRate{
			Pair: *p,
			Side: side,
			Rate: rate.OrderBuyRate,
		}, nil
	}
	return &model.OrderRate{
		Pair: *p,
		Side: side,
		Rate: rate.OrderSellRate,
	}, nil
}

// GetAccountBalance 残高を取得
func (e *ExchangeMock) GetAccountBalance() (*model.Balance, error) {
	return nil, nil
}

// GetOpenOrders 未決済の注文を取得
func (e *ExchangeMock) GetOpenOrders(*model.CurrencyPair) ([]model.Order, error) {
	oo := []model.Order{}
	for _, o := range e.orders {
		if o.Status == model.Open {
			oo = append(oo, o)
		}
	}
	return oo, nil
}

// GetContracts 約定情報を取得
func (e *ExchangeMock) GetContracts() ([]model.Contract, error) {
	return e.contracts, nil
}

// PostOrder 注文を送信
func (e *ExchangeMock) PostOrder(o *model.NewOrder) (*model.Order, error) {
	var amount float32 = 0.0
	if o.Type == model.MarketBuy {
		amount = *o.MarketBuyAmount
	} else {
		amount = *o.Amount
	}

	order := model.Order{
		ID:           uint64(len(e.orders) + 1),
		Type:         o.Type,
		Pair:         o.Pair,
		Amount:       amount,
		Rate:         o.Rate,
		StopLossRate: o.StopLossRate,
		Status:       model.Open,
	}
	e.orders = append(e.orders, order)

	if o.Type == model.MarketBuy || o.Type == model.MarketSell {
		e.closeOrder(&e.orders[len(e.orders)-1])
	}

	return &order, nil
}

// DeleteOrder 注文を削除
func (e *ExchangeMock) DeleteOrder(id uint64) error {
	e.orders[id].Status = model.Canceled
	return nil
}

// NextStep 次のステップに進める
func (e *ExchangeMock) NextStep() {
	if e.HasNextStep() {
		e.currentStep++
	}

	for i, _ := range e.orders {
		e.closeOrder(&e.orders[i])
	}
}

func (e *ExchangeMock) closeOrder(o *model.Order) {
	if o.Status != model.Open {
		return
	}

	rate := e.rates[e.currentStep]

	var contract *model.Contract
	switch o.Type {
	case model.Buy:
		if o.Rate != nil && (*o.Rate) >= rate.OrderBuyRate {
			o.Status = model.Closed
		} else if o.StopLossRate != nil && (*o.StopLossRate) <= rate.OrderBuyRate {
			o.Status = model.Closed
		}
		if o.Status == model.Closed {
			contract = &model.Contract{
				ID:               uint64(len(e.contracts) + 1),
				OrderID:          o.ID,
				Rate:             rate.OrderBuyRate,
				IncreaseCurrency: e.pair.Key,
				IncreaseAmount:   o.Amount / rate.OrderBuyRate,
				DecreaseCurrency: e.pair.Settlement,
				DecreaseAmount:   -o.Amount,
				FeeCurrency:      "",
				Fee:              0,
				Liquidity:        model.Taker,
				Side:             model.BuySide,
			}
		}
	case model.MarketBuy:
		o.Status = model.Closed
		contract = &model.Contract{
			ID:               uint64(len(e.contracts) + 1),
			OrderID:          o.ID,
			Rate:             rate.OrderBuyRate,
			IncreaseCurrency: e.pair.Key,
			IncreaseAmount:   o.Amount / rate.OrderBuyRate,
			DecreaseCurrency: e.pair.Settlement,
			DecreaseAmount:   -o.Amount,
			FeeCurrency:      "",
			Fee:              0,
			Liquidity:        model.Taker,
			Side:             model.BuySide,
		}
	case model.Sell:
		if o.Rate != nil && (*o.Rate) <= rate.OrderSellRate {
			o.Status = model.Closed
		} else if o.StopLossRate != nil && (*o.StopLossRate) >= rate.OrderSellRate {
			o.Status = model.Closed
		}
		contract = &model.Contract{
			ID:               uint64(len(e.contracts) + 1),
			OrderID:          o.ID,
			Rate:             rate.OrderSellRate,
			IncreaseCurrency: e.pair.Settlement,
			IncreaseAmount:   o.Amount * rate.OrderBuyRate,
			DecreaseCurrency: e.pair.Key,
			DecreaseAmount:   -o.Amount,
			FeeCurrency:      "",
			Fee:              0,
			Liquidity:        model.Taker,
			Side:             model.SellSide,
		}
	case model.MarketSell:
		o.Status = model.Closed
		contract = &model.Contract{
			ID:               uint64(len(e.contracts) + 1),
			OrderID:          o.ID,
			Rate:             rate.OrderSellRate,
			IncreaseCurrency: e.pair.Settlement,
			IncreaseAmount:   o.Amount * rate.OrderBuyRate,
			DecreaseCurrency: e.pair.Key,
			DecreaseAmount:   -o.Amount,
			FeeCurrency:      "",
			Fee:              0,
			Liquidity:        model.Taker,
			Side:             model.SellSide,
		}
	}

	if contract != nil {
		e.contracts = append(e.contracts, *contract)
	}
}

// HasNextStep 次のステップがあるか？
func (e *ExchangeMock) HasNextStep() bool {
	return e.currentStep < len(e.rates)-1
}
