package coincheck

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
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
	u, err := c.makeURL("/api/exchange/orders/rate", map[string]string{
		"order_type": string(t),
		"pair":       p.String(),
		"amount":     "1",
	})
	if err != nil {
		return nil, err
	}

	var res struct {
		Rate   string `json:"rate"`
		Amount string `json:"amount"`
		Price  string `json:"price"`
	}
	if err := c.request(http.MethodGet, u, "", &res); err != nil {
		return nil, err
	}

	var rate float64
	if rate, err = strconv.ParseFloat(res.Rate, 32); err != nil {
		return nil, fmt.Errorf("failed to parse response of GetOrderRate, t: %v, p: %v; error: %w", t, p, err)
	}

	return &model.OrderRate{
		Pair: p,
		Rate: float32(rate),
	}, nil
}

// GetAccountBalance 残高取得
func (c *Client) GetAccountBalance() (*model.Balance, error) {
	u, err := c.makeURL("/api/accounts/balance", nil)
	if err != nil {
		return nil, err
	}

	var res struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
		Jpy     string `json:"jpy"`
		Btc     string `json:"btc"`
	}
	if err := c.request(http.MethodGet, u, "", &res); err != nil {
		return nil, err
	}

	return &model.Balance{
		Jpy: toFloat32(res.Jpy, 0),
		Btc: toFloat32(res.Btc, 0),
	}, nil
}

// GetOrderTransactions 注文履歴取得
func (c *Client) GetOrderTransactions() ([]model.OrderTransaction, error) {
	u, err := c.makeURL("/api/exchange/orders/transactions", nil)
	if err != nil {
		return nil, err
	}

	type transaction struct {
		ID          uint64            `json:"id"`
		OrderID     uint64            `json:"order_id"`
		CreatedAt   time.Time         `json:"created_at"`
		Funds       map[string]string `json:"funds"`
		PairStr     string            `json:"pair"`
		Rate        string            `json:"rate"`
		FeeCurrency string            `json:"fee_currency"`
		Fee         string            `json:"fee"`
		Liquidity   string            `json:"liquidity"`
		Side        string            `json:"side"`
	}

	var res struct {
		Transactions []transaction `json:"transactions"`
	}

	if err := c.request(http.MethodGet, u, "", &res); err != nil {
		return nil, err
	}

	tt := []model.OrderTransaction{}
	for _, t := range res.Transactions {
		if len(t.Funds) != 2 {
			return nil, fmt.Errorf("transaction has not 2 funds, funds: %v", t.Funds)
		}

		currencies := []model.CurrencyType{}
		funds := []float32{}
		for k, v := range t.Funds {
			currencies = append(currencies, model.CurrencyType(k))
			funds = append(funds, toFloat32(v, 0))
		}

		tt = append(tt, model.OrderTransaction{
			OrderID:   t.OrderID,
			CreatedAt: t.CreatedAt,
			Canceled:  false,
			Contract: &model.Contract{
				ID:          t.ID,
				Rate:        toFloat32(t.Rate, 0),
				Currency1:   currencies[0],
				Fund1:       funds[0],
				Currency2:   currencies[1],
				Fund2:       funds[1],
				FeeCurrency: model.CurrencyType(t.FeeCurrency),
				Fee:         toFloat32(t.Fee, 0),
				Liquidity:   model.LiquidityType(t.Liquidity),
				Side:        model.OrderType(t.Side),
			},
		})
	}
	return tt, nil
}

// PostOrder 注文登録
func (c *Client) PostOrder(o *model.NewOrder) (*model.Order, error) {
	u, err := c.makeURL("/api/exchange/orders", nil)
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(struct {
		Pair         string `json:"pair"`
		OrderType    string `json:"order_type"`
		Rate         string `json:"rate,omitempty"`
		Amount       string `json:"amount,omitempty"`
		StopLossRate string `json:"stop_loss_rate"`
	}{
		Pair:         o.Pair.String(),
		OrderType:    string(o.Type),
		Rate:         toRequestString(o.Rate),
		Amount:       toRequestString(o.Amount),
		StopLossRate: toRequestString(o.StopLossRate),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create request param, order: %v", o)
	}

	var res struct {
		ID           uint64    `json:"id"`
		Rate         string    `json:"rate"`
		Amount       string    `json:"amount"`
		OrderType    string    `json:"order_type"`
		StopLossRate string    `json:"stop_loss_rate"`
		Pair         string    `json:"pair"`
		CreatedAt    time.Time `json:"created_at"`
	}

	if err := c.request(http.MethodPost, u, string(body), &res); err != nil {
		return nil, err
	}

	return &model.Order{
		ID:           res.ID,
		Type:         model.OrderType(res.OrderType),
		Pair:         model.CurrencyPair{},
		Amount:       toFloat32(res.Amount, 0),
		Rate:         toFloat32Nullable(res.Rate, nil),
		StopLossRate: toFloat32Nullable(res.StopLossRate, nil),
		CreatedAt:    res.CreatedAt,
	}, nil
}

// DeleteOrder 注文削除
func (c *Client) DeleteOrder(id uint64) error {
	u, err := c.makeURL(fmt.Sprintf("/api/exchange/orders/%d", id), nil)
	if err != nil {
		return err
	}
	var res struct {
		ID uint64 `json:"id"`
	}
	return c.request(http.MethodDelete, u, "", &res)
}
