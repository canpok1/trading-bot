package memory

import "trading-bot/pkg/domain/model"

// RateRepository レート保存
type RateRepository struct {
	maxSize   int
	buyQueue  []model.OrderRate
	sellQueue []model.OrderRate
}

// NewRateRepository 生成
func NewRateRepository(maxSize int) *RateRepository {
	return &RateRepository{
		maxSize:   maxSize,
		buyQueue:  []model.OrderRate{},
		sellQueue: []model.OrderRate{},
	}
}

// AddOrderRate レート追加
func (r *RateRepository) AddOrderRate(o *model.OrderRate) error {
	if o.Side == model.SellSide {
		r.sellQueue = append(r.sellQueue, *o)
		if len(r.sellQueue) > r.maxSize {
			r.sellQueue = r.sellQueue[1:]
		}
	} else {
		r.buyQueue = append(r.buyQueue, *o)
		if len(r.buyQueue) > r.maxSize {
			r.buyQueue = r.buyQueue[1:]
		}
	}
	return nil
}

// GetCurrentRate 現在のレートを取得
func (r *RateRepository) GetCurrentRate(t *model.CurrencyType, s model.OrderSide) *float32 {
	if s == model.SellSide {
		size := len(r.sellQueue)
		if size == 0 {
			return nil
		}
		return &r.sellQueue[size-1].Rate
	}

	size := len(r.buyQueue)
	if size == 0 {
		return nil
	}
	return &r.buyQueue[size-1].Rate
}

// GetRateHistory レートの履歴を取得
func (r *RateRepository) GetRateHistory(t *model.CurrencyType, s model.OrderSide) []float32 {
	h := []float32{}

	if s == model.SellSide {
		for _, r := range r.sellQueue {
			h = append(h, r.Rate)
		}
	} else {
		for _, r := range r.buyQueue {
			h = append(h, r.Rate)
		}
	}

	return h
}

// GetHistorySizeMax 最大容量取得
func (r *RateRepository) GetHistorySizeMax() int {
	return r.maxSize
}
