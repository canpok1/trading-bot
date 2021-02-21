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
func (c *Client) GetOrderRate(t model.OrderType, p model.CurrencyPair) (*model.OrderRate, error) {
	return c.getOrderRate(t, p)
}

// GetAccountBalance 残高取得
func (c *Client) GetAccountBalance() (*model.Balance, error) {
	return c.getAccountBalance()
}

// GetOpenOrders 未決済の注文取得
func (c *Client) GetOpenOrders() ([]model.Order, error) {
	orders := []model.Order{}

	oo, err := c.getOpenOrders()
	if err != nil {
		return nil, err
	}
	for _, o := range oo {
		orders = append(orders, model.Order{
			ID:           o.ID,
			Type:         model.OrderType(o.OrderType),
			Pair:         toCurrencyPair(o.Pair),
			Amount:       toFloat32(o.PendingAmount, 0),
			Rate:         toFloat32Nullable(o.Rate, nil),
			StopLossRate: toFloat32Nullable(o.StopLossRate, nil),
			OpenAt:       o.CreatedAt,
			Canceled:     false,
			Contract:     nil,
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

		currencies := []model.CurrencyType{}
		funds := []float32{}
		for k, v := range t.Funds {
			currencies = append(currencies, model.CurrencyType(k))
			funds = append(funds, toFloat32(v, 0))
		}

		cc = append(cc, model.Contract{
			OrderID:     t.OrderID,
			Rate:        toFloat32(t.Rate, 0),
			Currency1:   currencies[0],
			Fund1:       funds[0],
			Currency2:   currencies[1],
			Fund2:       funds[1],
			FeeCurrency: model.CurrencyType(t.FeeCurrency),
			Fee:         toFloat32(t.Fee, 0),
			Liquidity:   model.LiquidityType(t.Liquidity),
			Side:        model.OrderType(t.Side),
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
		Pair:         model.CurrencyPair{},
		Amount:       toFloat32(res.Amount, 0),
		Rate:         toFloat32Nullable(res.Rate, nil),
		StopLossRate: toFloat32Nullable(res.StopLossRate, nil),
		OpenAt:       res.CreatedAt,
	}, nil
}

// DeleteOrder 注文削除
func (c *Client) DeleteOrder(id uint64) error {
	return c.deleteOrder(id)
}
