package coincheck

import "time"

// NewOrder 注文（新規）
type NewOrder struct {
	Pair            string `json:"pair"`
	OrderType       string `json:"order_type"`
	Rate            string `json:"rate,omitempty"`
	Amount          string `json:"amount,omitempty"`
	MarketBuyAmount string `json:"market_buy_amount,omitempty"`
	StopLossRate    string `json:"stop_loss_rate,omitempty"`
}

// RegisteredOrder 注文（登録済み）
type RegisteredOrder struct {
	ID           uint64    `json:"id"`
	Rate         string    `json:"rate"`
	Amount       string    `json:"amount"`
	OrderType    string    `json:"order_type"`
	StopLossRate string    `json:"stop_loss_rate"`
	Pair         string    `json:"pair"`
	CreatedAt    time.Time `json:"created_at"`
}

// OpenOrder 注文（未決済）
type OpenOrder struct {
	ID            uint64    `json:"id"`
	OrderType     string    `json:"order_type"`
	Rate          string    `json:"rate"`
	Pair          string    `json:"pair"`
	PendingAmount string    `json:"pending_amount"`
	StopLossRate  string    `json:"stop_loss_rate"`
	CreatedAt     time.Time `json:"created_at"`
}

// OrderTransaction 取引履歴
type OrderTransaction struct {
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
