package memory

import (
	"fmt"
	"time"
	"trading-bot/pkg/domain/model"
)

type DummyRDS struct {
	orders      map[uint64]*model.Order
	positions   map[uint64]*model.Position
	contracts   map[uint64]*model.Contract
	profit      float64
	rates       []model.StoreRate
	rateMaxSize *int
}

func NewDummyRDS(rateMaxSize *int) *DummyRDS {
	return &DummyRDS{
		orders:      map[uint64]*model.Order{},
		positions:   map[uint64]*model.Position{},
		contracts:   map[uint64]*model.Contract{},
		profit:      0,
		rates:       []model.StoreRate{},
		rateMaxSize: rateMaxSize,
	}
}

func (d *DummyRDS) GetOrder(id uint64) (*model.Order, error) {
	return d.orders[id], nil
}

func (d *DummyRDS) GetOpenOrders() ([]model.Order, error) {
	orders := []model.Order{}
	for _, v := range d.orders {
		orders = append(orders, *v)
	}
	return orders, nil
}

func (d *DummyRDS) UpdateCloseOrderID(id, closerOrderID uint64) (*model.Position, error) {
	p := d.positions[id]
	p.CloserOrder = d.orders[closerOrderID]
	return p, nil
}

func (d *DummyRDS) UpdateStatus(orderID uint64, status model.OrderStatus) error {
	d.orders[orderID].Status = status
	return nil
}

func (d *DummyRDS) GetContracts(orderID uint64) ([]model.Contract, error) {
	cc := []model.Contract{}
	for _, contract := range d.contracts {
		if contract.OrderID == orderID {
			cc = append(cc, *contract)
		}
	}
	return cc, nil
}

func (d *DummyRDS) UpsertContracts(contracts []model.Contract) error {
	for _, contract := range contracts {
		if registered, ok := d.contracts[contract.ID]; ok {
			registered.OrderID = contract.OrderID
			registered.Rate = contract.Rate
			registered.IncreaseCurrency = contract.IncreaseCurrency
			registered.IncreaseAmount = contract.IncreaseAmount
			registered.DecreaseCurrency = contract.DecreaseCurrency
			registered.DecreaseAmount = contract.DecreaseAmount
			registered.FeeCurrency = contract.FeeCurrency
			registered.Fee = contract.Fee
			registered.Liquidity = contract.Liquidity
			registered.Side = contract.Side
		} else {
			d.contracts[contract.ID] = &contract
			if contract.IncreaseCurrency == model.JPY {
				d.profit += float64(contract.IncreaseAmount)
			}
			if contract.DecreaseCurrency == model.JPY {
				d.profit += float64(contract.DecreaseAmount)
			}
		}
	}

	return nil
}

func (d *DummyRDS) AddNewOrder(o *model.Order) (*model.Position, error) {
	o.ID = uint64(len(d.orders) + 1)
	d.orders[o.ID] = o

	p := model.Position{
		ID:          uint64(len(d.positions) + 1),
		OpenerOrder: o,
		CloserOrder: nil,
	}
	d.positions[p.ID] = &p

	return &p, nil
}

func (d *DummyRDS) AddSettleOrder(positionID uint64, o *model.Order) (*model.Position, error) {
	o.ID = uint64(len(d.orders) + 1)
	d.orders[o.ID] = o

	p := d.positions[positionID]
	p.CloserOrder = o
	return p, nil
}

func (d *DummyRDS) CancelSettleOrder(positionID uint64) (*model.Position, error) {
	p := d.positions[positionID]
	p.CloserOrder.Status = model.Canceled
	p.CloserOrder = nil
	return p, nil
}

func (d *DummyRDS) GetOpenPositions() ([]model.Position, error) {
	pp := []model.Position{}
	for _, p := range d.positions {
		if p.CloserOrder == nil || p.CloserOrder.Status == model.Open {
			pp = append(pp, *p)
		}
	}
	return pp, nil
}

func (d *DummyRDS) TruncateAll() error {
	d.orders = map[uint64]*model.Order{}
	d.positions = map[uint64]*model.Position{}
	d.contracts = map[uint64]*model.Contract{}
	d.profit = 0
	return nil
}

func (d *DummyRDS) GetProfit() (float64, error) {
	return d.profit, nil
}

// AddRates レート追加
func (d *DummyRDS) AddRates(p *model.CurrencyPair, rate float64, t time.Time) error {
	d.rates = append(d.rates, model.StoreRate{
		Pair: *p,
		Rate: rate,
	})
	if d.rateMaxSize != nil && len(d.rates) > *d.rateMaxSize {
		d.rates = d.rates[1:]
	}
	return nil
}

// GetRate 現在のレートを取得
func (d *DummyRDS) GetRate(p *model.CurrencyPair) (float64, error) {
	size := len(d.rates)
	if size == 0 {
		return 0, fmt.Errorf("rate is nothing")
	}
	return d.rates[size-1].Rate, nil
}

// GetRates レートの履歴を取得
func (d *DummyRDS) GetRates(p *model.CurrencyPair, duration *time.Duration) ([]float64, error) {
	h := []float64{}

	for _, r := range d.rates {
		h = append(h, r.Rate)
	}

	return h, nil
}
