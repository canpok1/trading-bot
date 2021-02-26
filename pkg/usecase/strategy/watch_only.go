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

	pp, err := s.facade.GetOpenPositions()
	if err != nil {
		return err
	}
	log.Printf("open position count: %d\n", len(pp))
	for _, p := range pp {
		log.Printf("open position id: %d\n", p.ID)
		{
			cc, err := s.facade.GetContracts(p.OpenerOrder.ID)
			if err != nil {
				return err
			}
			log.Printf("open order id: %d, contract count: %d\n", p.OpenerOrder.ID, len(cc))
			for _, c := range cc {
				log.Printf("contract: %#v\n", c)
			}
		}
		{
			cc, err := s.facade.GetContracts(p.CloserOrder.ID)
			if err != nil {
				return err
			}
			log.Printf("close order id: %d, contract count: %d\n", p.CloserOrder.ID, len(cc))
			for _, c := range cc {
				log.Printf("contract: %#v\n", c)
			}
		}
	}

	return nil
}
