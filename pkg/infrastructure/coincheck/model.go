package coincheck

import (
	"strconv"
	"strings"
	"time"
	"trading-bot/pkg/domain/model"
)

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

// Balance 残高
type Balance struct {
	Jpy          string `json:"jpy"`
	Btc          string `json:"btc"`
	Etc          string `json:"etc"`
	Fct          string `json:"fct"`
	Mona         string `json:"mona"`
	JpyReserved  string `json:"jpy_reserved"`
	BtcReserved  string `json:"btc_reserved"`
	EtcReserved  string `json:"etc_reserved"`
	FctReserved  string `json:"fct_reserved"`
	MonaReserved string `json:"mona_reserved"`
}

// 取引履歴
type TradeHistory struct {
	ID     uint64
	Pair   string
	Rate   float64
	Amount float64
	Side   model.OrderSide
	Time   time.Time
}

// NewTradeHistory Webソケットのメッセージから取引履歴を生成
func NewTradeHistory(b []byte) (h *TradeHistory, err error) {
	message := string(b)
	message = strings.ReplaceAll(message, "[", "")
	message = strings.ReplaceAll(message, "]", "")
	message = strings.ReplaceAll(message, "\"", "")

	values := strings.Split(message, ",")

	h = &TradeHistory{}

	h.ID, err = strconv.ParseUint(values[0], 10, 64)
	if err != nil {
		return
	}
	h.Pair = values[1]
	h.Rate, err = strconv.ParseFloat(values[2], 64)
	if err != nil {
		return
	}
	h.Amount, err = strconv.ParseFloat(values[3], 64)
	if err != nil {
		return
	}
	if values[4] == "buy" {
		h.Side = model.BuySide
	} else {
		h.Side = model.SellSide
	}

	h.Time = time.Now()
	return
}
