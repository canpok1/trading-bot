package coincheck

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"trading-bot/pkg/domain/model"
	"trading-bot/pkg/infrastructure/memory"

	"github.com/gorilla/websocket"
	gocache "github.com/pmylund/go-cache"
)

const (
	origin               = "https://coincheck.com/"
	originWS             = "wss://ws-api.coincheck.com/"
	cacheExpire          = 24 * 60 * 60 * time.Second
	cacheCleanupInterval = 60 * time.Second
)

// Client Coincheck用クライアント
type Client struct {
	Logger       *memory.Logger
	APIAccessKey string
	APISecretKey string
	tradeCaches  map[string]map[int]*gocache.Cache
}

// NewClient クライアントを生成
func NewClient(logger *memory.Logger, APIAccessKey, APISecretKey string) *Client {
	return &Client{
		Logger:       logger,
		APIAccessKey: APIAccessKey,
		APISecretKey: APISecretKey,
		tradeCaches:  map[string]map[int]*gocache.Cache{},
	}
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
func (c *Client) GetBalance(currency model.CurrencyType) (*model.Balance, error) {
	res, err := c.getAccountBalance()
	if err != nil {
		return nil, err
	}

	switch currency {
	case model.JPY:
		return &model.Balance{
			Currency: currency,
			Amount:   toFloat(res.Jpy, 0),
			Reserved: toFloat(res.JpyReserved, 0),
		}, nil
	case model.BTC:
		return &model.Balance{
			Currency: currency,
			Amount:   toFloat(res.Btc, 0),
			Reserved: toFloat(res.BtcReserved, 0),
		}, nil
	case model.ETC:
		return &model.Balance{
			Currency: currency,
			Amount:   toFloat(res.Etc, 0),
			Reserved: toFloat(res.EtcReserved, 0),
		}, nil
	case model.FCT:
		return &model.Balance{
			Currency: currency,
			Amount:   toFloat(res.Fct, 0),
			Reserved: toFloat(res.FctReserved, 0),
		}, nil
	case model.MONA:
		return &model.Balance{
			Currency: currency,
			Amount:   toFloat(res.Mona, 0),
			Reserved: toFloat(res.MonaReserved, 0),
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

// GetCancelStatus キャンセルステータス取得
func (c *Client) GetCancelStatus(id uint64) (bool, error) {
	return c.getCancelStatus(id)
}

// GetVolumes 取引量を取得
func (c *Client) GetVolumes(p *model.CurrencyPair, side model.OrderSide, d time.Duration) (float64, error) {
	cache := c.getCache(p, side)
	if cache == nil {
		return 0.0, nil
	}

	volumes := 0.0
	border := time.Now().Add(-d)
	for _, item := range cache.Items() {
		h, ok := item.Object.(*TradeHistory)
		if !ok {
			return 0.0, fmt.Errorf("type assertion error, volume cache item is not TradeHistory; %v", item.Object)
		}
		if h.Time.Before(border) {
			continue
		}
		volumes += h.Amount
	}
	return volumes, nil
}

// SubscribeTradeHistory 取引履歴を購読
func (c *Client) SubscribeTradeHistory(ctx context.Context, p *model.CurrencyPair, callback func(*TradeHistory) error) error {
	ws, _, err := websocket.DefaultDialer.DialContext(ctx, originWS, nil)
	if err != nil {
		return err
	}
	defer func() {
		ws.Close()
	}()

	ws.SetCloseHandler(func(code int, text string) error {
		return nil
	})

	param := map[string]string{
		"type":    "subscribe",
		"channel": p.String() + "-trades",
	}
	bytes, err := json.Marshal(param)
	if err != nil {
		return err
	}
	if err := ws.WriteMessage(websocket.TextMessage, bytes); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			ws.SetReadDeadline(time.Now().Add(30 * time.Second))
			_, b, err := ws.ReadMessage()
			if err != nil {
				return err
			}
			c.Logger.Debug("[receive] => trade:%v", string(b))

			h, err := NewTradeHistory(b)
			if err != nil {
				return err
			}

			if _, ok := c.tradeCaches[p.String()]; !ok {
				c.tradeCaches[p.String()] = map[int]*gocache.Cache{}
			}
			if _, ok := c.tradeCaches[p.String()][int(h.Side)]; !ok {
				c.tradeCaches[p.String()][int(h.Side)] = gocache.New(cacheExpire, cacheCleanupInterval)
			}

			key := fmt.Sprintf("%d", h.ID)
			if err := c.tradeCaches[p.String()][int(h.Side)].Add(key, h, cacheExpire); err != nil {
				return err
			}

			if err := callback(h); err != nil {
				return err
			}
		}
	}
}

func (c *Client) getCache(p *model.CurrencyPair, side model.OrderSide) *gocache.Cache {
	if caches, ok := c.tradeCaches[p.String()]; ok {
		if cache, ok := caches[int(side)]; ok {
			return cache
		}
	}
	return nil
}
