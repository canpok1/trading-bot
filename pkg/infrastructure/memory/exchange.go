package memory

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"time"
	"trading-bot/pkg/domain/model"
)

// Rate レート
type Rate struct {
	Datetime      string
	StoreRate     float64
	OrderBuyRate  float64
	OrderSellRate float64
}

// NewRate レートを生成
func NewRate(v []string) (*Rate, error) {
	if len(v) != 3 {
		return nil, fmt.Errorf("csv is not 3 columns, [%d columns]", len(v))
	}
	buyRate, err := strconv.ParseFloat(v[1], 32)
	if err != nil {
		return nil, err
	}
	sellRate, err := strconv.ParseFloat(v[2], 32)
	if err != nil {
		return nil, err
	}

	return &Rate{
		Datetime:      v[0],
		StoreRate:     sellRate,
		OrderBuyRate:  buyRate,
		OrderSellRate: sellRate,
	}, nil
}

// ExchangeMock 取引所モック
type ExchangeMock struct {
	rateReader *csv.Reader
	slippage   float64
	Rate       Rate
	orders     []model.Order
	contracts  []model.Contract
}

// NewExchangeMock 生成
func NewExchangeMock(r io.Reader, slippage float64) (*ExchangeMock, error) {
	reader := csv.NewReader(r)

	// ヘッダを読み飛ばす
	_, err := reader.Read()
	if err != nil {
		return nil, err
	}

	record, err := reader.Read()
	if err != nil {
		return nil, err
	}
	rate, err := NewRate(record)
	if err != nil {
		return nil, err
	}

	return &ExchangeMock{
		rateReader: reader,
		slippage:   slippage,
		Rate:       *rate,
		orders:     []model.Order{},
		contracts:  []model.Contract{},
	}, nil
}

// GetStoreRate 販売所のレートを取得
func (e *ExchangeMock) GetStoreRate(p *model.CurrencyPair) (*model.StoreRate, error) {
	return &model.StoreRate{
		Pair: *p,
		Rate: e.Rate.StoreRate,
	}, nil
}

// GetOrderRate 取引所のレートを取得
func (e *ExchangeMock) GetOrderRate(p *model.CurrencyPair, side model.OrderSide) (*model.OrderRate, error) {
	if side == model.BuySide {
		return &model.OrderRate{
			Pair: *p,
			Side: side,
			Rate: e.Rate.OrderBuyRate,
		}, nil
	}
	return &model.OrderRate{
		Pair: *p,
		Side: side,
		Rate: e.Rate.OrderSellRate,
	}, nil
}

// GetBalance 残高を取得
func (e *ExchangeMock) GetBalance(currency model.CurrencyType) (*model.Balance, error) {
	return &model.Balance{
		Currency: currency,
		Amount:   100000,
	}, nil
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
	var amount float64 = 0.0
	if o.Type == model.MarketBuy {
		amount = *o.MarketBuyAmount
	} else {
		amount = *o.Amount
	}

	orderedAt, err := time.Parse(time.RFC3339, e.Rate.Datetime)
	if err != nil {
		return nil, err
	}

	order := model.Order{
		ID:           uint64(len(e.orders) + 1),
		Type:         o.Type,
		Pair:         o.Pair,
		Amount:       amount,
		Rate:         o.Rate,
		StopLossRate: o.StopLossRate,
		Status:       model.Open,
		OrderedAt:    orderedAt,
	}
	e.orders = append(e.orders, order)

	if order.Type == model.MarketBuy || order.Type == model.MarketSell {
		e.closeOrder(order.ID)
	}

	return &order, nil
}

// DeleteOrder 注文を削除
func (e *ExchangeMock) DeleteOrder(id uint64) error {
	e.orders[id-1].Status = model.Canceled
	return nil
}

// NextStep 次のステップに進める
func (e *ExchangeMock) NextStep() bool {
	record, err := e.rateReader.Read()
	if err != nil {
		return false
	}
	rate, err := NewRate(record)
	if err != nil {
		return false
	}
	e.Rate = *rate

	for _, o := range e.orders {
		e.closeOrder(o.ID)
	}

	return true
}

func (e *ExchangeMock) closeOrder(orderID uint64) {
	o := &e.orders[orderID-1]
	if o.Status != model.Open {
		return
	}

	var contract *model.Contract
	switch o.Type {
	case model.Buy:
		if o.Rate != nil && (*o.Rate) >= e.Rate.OrderBuyRate {
			o.Status = model.Closed
		} else if o.StopLossRate != nil && (*o.StopLossRate) <= e.Rate.OrderBuyRate {
			o.Status = model.Closed
		}
		if o.Status == model.Closed {
			contract = &model.Contract{
				ID:               uint64(len(e.contracts) + 1),
				OrderID:          o.ID,
				Rate:             e.Rate.OrderBuyRate,
				IncreaseCurrency: o.Pair.Key,
				IncreaseAmount:   o.Amount / e.Rate.OrderBuyRate,
				DecreaseCurrency: o.Pair.Settlement,
				DecreaseAmount:   -o.Amount,
				FeeCurrency:      "",
				Fee:              0,
				Liquidity:        model.Taker,
				Side:             model.BuySide,
			}
		}
	case model.MarketBuy:
		o.Status = model.Closed
		rate := e.Rate.OrderBuyRate * (1.00 + e.slippage)
		contract = &model.Contract{
			ID:               uint64(len(e.contracts) + 1),
			OrderID:          o.ID,
			Rate:             rate,
			IncreaseCurrency: o.Pair.Key,
			IncreaseAmount:   o.Amount / rate,
			DecreaseCurrency: o.Pair.Settlement,
			DecreaseAmount:   -o.Amount,
			FeeCurrency:      "",
			Fee:              0,
			Liquidity:        model.Taker,
			Side:             model.BuySide,
		}
	case model.Sell:
		if o.Rate != nil && (*o.Rate) <= e.Rate.OrderSellRate {
			o.Status = model.Closed
		} else if o.StopLossRate != nil && (*o.StopLossRate) >= e.Rate.OrderSellRate {
			o.Status = model.Closed
		}
		if o.Status == model.Closed {
			contract = &model.Contract{
				ID:               uint64(len(e.contracts) + 1),
				OrderID:          o.ID,
				Rate:             e.Rate.OrderSellRate,
				IncreaseCurrency: o.Pair.Settlement,
				IncreaseAmount:   o.Amount * e.Rate.OrderBuyRate,
				DecreaseCurrency: o.Pair.Key,
				DecreaseAmount:   -o.Amount,
				FeeCurrency:      "",
				Fee:              0,
				Liquidity:        model.Taker,
				Side:             model.SellSide,
			}
		}
	case model.MarketSell:
		o.Status = model.Closed
		rate := e.Rate.OrderSellRate * (1.00 - e.slippage)
		contract = &model.Contract{
			ID:               uint64(len(e.contracts) + 1),
			OrderID:          o.ID,
			Rate:             rate,
			IncreaseCurrency: o.Pair.Settlement,
			IncreaseAmount:   o.Amount * rate,
			DecreaseCurrency: o.Pair.Key,
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

// GetVolumes 取引量を取得
func (e *ExchangeMock) GetVolumes(*model.CurrencyPair, model.OrderSide, time.Duration) (float64, error) {
	return 0, nil
}
