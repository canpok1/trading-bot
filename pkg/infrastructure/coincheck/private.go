package coincheck

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"trading-bot/pkg/domain/model"
)

// getOrderRate レート取得
func (c *Client) getOrderRate(t model.OrderType, p model.CurrencyPair) (*model.OrderRate, error) {
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

// getAccountBalance 残高取得
func (c *Client) getAccountBalance() (*model.Balance, error) {
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

// getOpenOrders 未決済の注文一覧
func (c *Client) getOpenOrders() ([]OpenOrder, error) {
	u, err := c.makeURL("/api/exchange/orders/opens", nil)
	if err != nil {
		return nil, err
	}

	var res struct {
		Orders []OpenOrder `json:"orders"`
	}

	if err := c.request(http.MethodGet, u, "", &res); err != nil {
		return nil, err
	}
	return res.Orders, nil

}

// getOrderTransactions 取引履歴
func (c *Client) getOrderTransactions() ([]OrderTransaction, error) {
	u, err := c.makeURL("/api/exchange/orders/transactions", nil)
	if err != nil {
		return nil, err
	}

	var res struct {
		Transactions []OrderTransaction `json:"transactions"`
	}

	if err := c.request(http.MethodGet, u, "", &res); err != nil {
		return nil, err
	}
	return res.Transactions, nil
}

// postOrder 新規注文
func (c *Client) postOrder(o *model.NewOrder) (*RegisteredOrder, error) {
	u, err := c.makeURL("/api/exchange/orders", nil)
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(NewOrder{
		Pair:         o.Pair.String(),
		OrderType:    string(o.Type),
		Rate:         toRequestString(o.Rate),
		Amount:       toRequestString(o.Amount),
		StopLossRate: toRequestString(o.StopLossRate),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create request param, order: %v", o)
	}

	var res RegisteredOrder
	if err := c.request(http.MethodPost, u, string(body), &res); err != nil {
		return nil, err
	}
	return &res, nil
}

// deleteOrder 注文キャンセル
func (c *Client) deleteOrder(id uint64) error {
	u, err := c.makeURL(fmt.Sprintf("/api/exchange/orders/%d", id), nil)
	if err != nil {
		return err
	}
	var res struct {
		ID uint64 `json:"id"`
	}
	return c.request(http.MethodDelete, u, "", &res)
}
