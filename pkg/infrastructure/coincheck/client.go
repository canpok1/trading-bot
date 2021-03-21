package coincheck

import (
	"fmt"
	"trading-bot/pkg/domain/model"
)

const (
	origin = "https://coincheck.com/"
)

// Client Coincheck用クライアント
type Client struct {
	APIAccessKey string
	APISecretKey string
}

// GetStoreRate 販売所のレート取得
func (c *Client) GetStoreRate(p *model.CurrencyPair) (*model.StoreRate, error) {
	r, err := c.getRate(p)
	if err != nil {
		return nil, err
	}
	return &model.StoreRate{
		Pair: *p,
		Rate: r,
	}, nil
}

// GetOrderRate 注文レート取得
func (c *Client) GetOrderRate(p *model.CurrencyPair, s model.OrderSide) (*model.OrderRate, error) {
	return c.getOrderRate(s, p)
}

// GetBalance 残高取得
func (c *Client) GetBalance(currency *model.CurrencyType) (*model.Balance, error) {
	res, err := c.getAccountBalance()
	if err != nil {
		return nil, err
	}

	switch *currency {
	case model.JPY:
		return &model.Balance{
			Currency: *currency,
			Amount:   toFloat(res.Jpy, 0),
		}, nil
	case model.BTC:
		return &model.Balance{
			Currency: *currency,
			Amount:   toFloat(res.Btc, 0),
		}, nil
	case model.ETC:
		return &model.Balance{
			Currency: *currency,
			Amount:   toFloat(res.Etc, 0),
		}, nil
	case model.FCT:
		return &model.Balance{
			Currency: *currency,
			Amount:   toFloat(res.Fct, 0),
		}, nil
	case model.MONA:
		return &model.Balance{
			Currency: *currency,
			Amount:   toFloat(res.Mona, 0),
		}, nil
	default:
		return nil, fmt.Errorf("failed to get balance, unknown ")
	}
}

// GetOpenOrders 未決済の注文取得
func (c *Client) GetOpenOrders(pair *model.CurrencyPair) ([]model.Order, error) {
	orders := []model.Order{}

	oo, err := c.getOpenOrders()
	if err != nil {
		return nil, err
	}
	for _, o := range oo {
		if pair != nil && o.Pair != pair.String() {
			continue
		}
		orders = append(orders, model.Order{
			ID:           o.ID,
			Type:         model.OrderType(o.OrderType),
			Pair:         toCurrencyPair(o.Pair),
			Amount:       toFloat(o.PendingAmount, 0),
			Rate:         toFloatNullable(o.Rate, nil),
			StopLossRate: toFloatNullable(o.StopLossRate, nil),
			Status:       model.Open,
			OrderedAt:    o.CreatedAt,
		})
	}
	return orders, nil
}

// GetContracts 約定情報取得
func (c *Client) GetContracts() ([]model.Contract, error) {
	tt, err := c.getOrderTransactions()
	if err != nil {
		return nil, err
	}

	cc := []model.Contract{}
	for _, t := range tt {
		if len(t.Funds) != 2 {
			return nil, fmt.Errorf("transaction has not 2 funds, funds: %v", t.Funds)
		}

		var increaseCurrency, decreaseCurrency model.CurrencyType
		var increaseAmount, decreaseAmount float64
		for k, v := range t.Funds {
			value := toFloat(v, 0)
			if value > 0.0 {
				increaseCurrency = model.CurrencyType(k)
				increaseAmount = value
			} else {
				decreaseCurrency = model.CurrencyType(k)
				decreaseAmount = value
			}
		}

		var liquidity model.LiquidityType
		if t.Liquidity == "M" {
			liquidity = model.Maker
		} else {
			liquidity = model.Taker
		}

		var side model.OrderSide
		if t.Side == "buy" {
			side = model.BuySide
		} else {
			side = model.SellSide
		}

		cc = append(cc, model.Contract{
			ID:               t.ID,
			OrderID:          t.OrderID,
			Rate:             toFloat(t.Rate, 0),
			IncreaseCurrency: increaseCurrency,
			IncreaseAmount:   increaseAmount,
			DecreaseCurrency: decreaseCurrency,
			DecreaseAmount:   decreaseAmount,
			FeeCurrency:      model.CurrencyType(t.FeeCurrency),
			Fee:              toFloat(t.Fee, 0),
			Liquidity:        liquidity,
			Side:             side,
		})
	}
	return cc, nil
}

// PostOrder 注文登録
func (c *Client) PostOrder(o *model.NewOrder) (*model.Order, error) {
	res, err := c.postOrder(o)
	if err != nil {
		return nil, err
	}
	return &model.Order{
		ID:           res.ID,
		Type:         model.OrderType(res.OrderType),
		Pair:         o.Pair,
		Amount:       toFloat(res.Amount, 0),
		Rate:         toFloatNullable(res.Rate, nil),
		StopLossRate: toFloatNullable(res.StopLossRate, nil),
		Status:       model.Open,
		OrderedAt:    res.CreatedAt,
	}, nil
}

// DeleteOrder 注文削除
func (c *Client) DeleteOrder(id uint64) error {
	return c.deleteOrder(id)
}
