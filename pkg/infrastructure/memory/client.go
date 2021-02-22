package memory

import "trading-bot/pkg/domain/model"

// RateRepository レート保存
type RateRepository struct {
	maxSize int
	queue   []model.OrderRate
}

// NewRateRepository 生成
func NewRateRepository(maxSize int) *RateRepository {
	return &RateRepository{
		maxSize: maxSize,
		queue:   []model.OrderRate{},
	}
}

// AddRate レート追加
func (r *RateRepository) AddRate(o *model.OrderRate) error {
	r.queue = append(r.queue, *o)
	if len(r.queue) > r.maxSize {
		r.queue = r.queue[1:]
	}
	return nil
}

// GetCurrentRate 現在のレートを取得
func (r *RateRepository) GetCurrentRate(t *model.CurrencyType) *model.OrderRate {
	size := len(r.queue)
	if size == 0 {
		return nil
	}
	return &r.queue[size-1]
}

// GetRateHistory レートの履歴を取得
func (r *RateRepository) GetRateHistory(t *model.CurrencyType) []model.OrderRate {
	return r.queue
}
