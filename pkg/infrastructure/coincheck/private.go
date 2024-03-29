package coincheck

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
	"trading-bot/pkg/domain/model"
)

func (c *Client) getTrades(p *model.CurrencyPair, limit int) ([]model.Trade, error) {
	u, err := c.makeURL("/api/trades", map[string]string{
		"pair":  p.String(),
		"limit": fmt.Sprintf("%d", limit),
	})
	if err != nil {
		return nil, err
	}

	type Pagination struct {
		Limit int `json:"limit"`
	}
	type Trade struct {
		ID        uint64    `json:"id"`
		Amount    string    `json:"amount"`
		Rate      string    `json:"rate"`
		Pair      string    `json:"pair"`
		OrderType string    `json:"order_type"`
		CreatedAt time.Time `json:"created_at"`
	}

	var res struct {
		Pagination Pagination `json:"pagination"`
		Data       []Trade    `json:"data"`
	}
	if err := c.requestWithValidation(http.MethodGet, u, "", &res); err != nil {
		return nil, err
	}

	trades := []model.Trade{}
	for _, t := range res.Data {
		amount, err := strconv.ParseFloat(t.Amount, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse response amount field of GetTrades, t: %v, p: %v; error: %w", t, p, err)
		}
		pair, err := model.ParseToCurrencyPair(t.Pair)
		if err != nil {
			return nil, fmt.Errorf("failed to parse response pair field of GetTrades, t: %v, p: %v; error: %w", t, p, err)
		}
		var rate float64
		if rate, err = strconv.ParseFloat(t.Rate, 32); err != nil {
			return nil, fmt.Errorf("failed to parse response of GetTrades, t: %v, p: %v; error: %w", t, p, err)
		}
		var side model.OrderSide
		if t.OrderType == "buy" {
			side = model.BuySide
		} else {
			side = model.SellSide
		}

		trades = append(trades, model.Trade{
			ID:        t.ID,
			Pair:      *pair,
			Rate:      rate,
			Amount:    amount,
			Side:      side,
			CreatedAt: t.CreatedAt,
		})
	}

	return trades, nil
}

// getOrderRate レート取得
func (c *Client) getOrderRate(s model.OrderSide, p *model.CurrencyPair) (*model.OrderRate, error) {
	t := "sell"
	if s == model.BuySide {
		t = "buy"
	}

	u, err := c.makeURL("/api/exchange/orders/rate", map[string]string{
		"order_type": t,
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
	if err := c.requestWithValidation(http.MethodGet, u, "", &res); err != nil {
		return nil, err
	}

	var rate float64
	if rate, err = strconv.ParseFloat(res.Rate, 32); err != nil {
		return nil, fmt.Errorf("failed to parse response of GetOrderRate, t: %v, p: %v; error: %w", t, p, err)
	}

	return &model.OrderRate{
		Pair: *p,
		Side: s,
		Rate: rate,
	}, nil
}

// getRate レート取得
func (c *Client) getRate(p *model.CurrencyPair) (float64, error) {
	u, err := c.makeURL(fmt.Sprintf("/api/rate/%s", p.String()), nil)
	if err != nil {
		return 0, err
	}

	var res struct {
		Rate string `json:"rate"`
	}
	if body, err := c.request(http.MethodGet, u, ""); err != nil {
		return 0, err
	} else if err := json.Unmarshal(body, &res); err != nil {
		return 0, err
	}

	var rate float64
	if rate, err = strconv.ParseFloat(res.Rate, 32); err != nil {
		return 0, fmt.Errorf("failed to parse response of GetRate, p: %v; error: %w", p, err)
	}

	return float64(rate), nil
}

// getAccountBalance 残高取得
func (c *Client) getAccountBalance() (res *Balance, err error) {
	u, err := c.makeURL("/api/accounts/balance", nil)
	if err != nil {
		return nil, err
	}
	err = c.requestWithValidation(http.MethodGet, u, "", &res)
	return
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

	if err := c.requestWithValidation(http.MethodGet, u, "", &res); err != nil {
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

	if err := c.requestWithValidation(http.MethodGet, u, "", &res); err != nil {
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
		Pair:            o.Pair.String(),
		OrderType:       string(o.Type),
		Rate:            toRequestString(o.Rate),
		Amount:          toRequestString(o.Amount),
		MarketBuyAmount: toRequestString(o.MarketBuyAmount),
		StopLossRate:    toRequestString(o.StopLossRate),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create request param, order: %v", o)
	}

	var res RegisteredOrder
	if err := c.requestWithValidation(http.MethodPost, u, string(body), &res); err != nil {
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
	return c.requestWithValidation(http.MethodDelete, u, "", &res)
}

// getCancelStatus キャンセルステータス取得
func (c *Client) getCancelStatus(id uint64) (bool, error) {
	u, err := c.makeURL("/api/exchange/orders/cancel_status", map[string]string{
		"id": fmt.Sprintf("%d", id),
	})
	if err != nil {
		return false, err
	}

	var res struct {
		ID        uint64    `json:"id"`
		Cancel    bool      `json:"cancel"`
		CreatedAt time.Time `json:"created_at"`
	}
	if err := c.requestWithValidation(http.MethodGet, u, "", &res); err != nil {
		return false, err
	}

	return res.Cancel, err
}
