package strategy

import (
	"context"
	"log"
	"trading-bot/pkg/usecase/trade"
)

// WatchOnlyStrategy 定期取得のみ
type WatchOnlyStrategy struct {
	facade *trade.Facade
}

// NewWatchOnlyStrategy 戦略を生成
func NewWatchOnlyStrategy(f *trade.Facade) *WatchOnlyStrategy {
	return &WatchOnlyStrategy{
		facade: f,
	}
}

// Trade 取引処理
func (s *WatchOnlyStrategy) Trade(ctx context.Context) error {
	if err := s.facade.FetchAll(); err != nil {
		return err
	}

	log.Printf("buy rate: %v\n", s.facade.GetBuyRateHistory())
	log.Printf("sell rate: %v\n", s.facade.GetSellRateHistory())

	oo, err := s.facade.GetOpenOrders()
	if err != nil {
		return err
	}
	log.Printf("open order count: %d\n", len(oo))
	for _, o := range oo {
		log.Printf("open order id: %d\n", o.ID)
	}

	for _, o := range oo {
		cc, err := s.facade.GetContracts(o.ID)
		if err != nil {
			return err
		}
		log.Printf("contract count: %d\n", len(cc))
		for _, c := range cc {
			log.Printf("contract: %#v\n", c)
		}
	}

	return nil
}
