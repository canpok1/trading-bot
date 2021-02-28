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

// GetOrderRate 注文レート取得
func (c *Client) GetOrderRate(p *model.CurrencyPair, s model.OrderSide) (*model.OrderRate, error) {
	return c.getOrderRate(s, p)
}

// GetAccountBalance 残高取得
func (c *Client) GetAccountBalance() (*model.Balance, error) {
	return c.getAccountBalance()
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
			Amount:       toFloat32(o.PendingAmount, 0),
			Rate:         toFloat32Nullable(o.Rate, nil),
			StopLossRate: toFloat32Nullable(o.StopLossRate, nil),
			Status:       model.Open,
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
		var increaseAmount, decreaseAmount float32
		for k, v := range t.Funds {
			value := toFloat32(v, 0)
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
			Rate:             toFloat32(t.Rate, 0),
			IncreaseCurrency: increaseCurrency,
			IncreaseAmount:   increaseAmount,
			DecreaseCurrency: decreaseCurrency,
			DecreaseAmount:   decreaseAmount,
			FeeCurrency:      model.CurrencyType(t.FeeCurrency),
			Fee:              toFloat32(t.Fee, 0),
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
		Amount:       toFloat32(res.Amount, 0),
		Rate:         toFloat32Nullable(res.Rate, nil),
		StopLossRate: toFloat32Nullable(res.StopLossRate, nil),
		Status:       model.Open,
	}, nil
}

// DeleteOrder 注文削除
func (c *Client) DeleteOrder(id uint64) error {
	return c.deleteOrder(id)
}
