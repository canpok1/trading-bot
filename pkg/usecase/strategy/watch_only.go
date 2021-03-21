package strategy

// import (
// 	"context"
// 	"time"
// 	"trading-bot/pkg/domain"
// 	"trading-bot/pkg/domain/model"
// 	"trading-bot/pkg/usecase/trade"
//
// 	"github.com/BurntSushi/toml"
// )
//
// // WatchOnlyStrategy 定期取得のみ
// type WatchOnlyStrategy struct {
// 	configPath     string
// 	facade         *trade.Facade
// 	interval       time.Duration
// 	targetCurrency *model.CurrencyPair
// 	logger         domain.Logger
// }
//
// // NewWatchOnlyStrategy 戦略を生成
// func NewWatchOnlyStrategy(f *trade.Facade, l domain.Logger, configPath string) (*WatchOnlyStrategy, error) {
// 	s := &WatchOnlyStrategy{
// 		configPath: configPath,
// 		facade:     f,
// 		logger:     l,
// 	}
//
// 	if err := s.loadConfig(); err != nil {
// 		return nil, err
// 	}
//
// 	return s, nil
// }
//
// // Trade 取引処理
// func (s *WatchOnlyStrategy) Trade(ctx context.Context) error {
// 	if err := s.loadConfig(); err != nil {
// 		return err
// 	}
//
// 	if err := s.facade.FetchAll(s.targetCurrency); err != nil {
// 		return err
// 	}
//
// 	s.logger.Debug("buy rate: %v\n", s.facade.GetBuyRateHistory(s.targetCurrency))
// 	s.logger.Debug("sell rate: %v\n", s.facade.GetSellRateHistory(s.targetCurrency))
//
// 	pp, err := s.facade.GetOpenPositions()
// 	if err != nil {
// 		return err
// 	}
// 	s.logger.Debug("open position count: %d\n", len(pp))
// 	for _, p := range pp {
// 		s.logger.Debug("open position id: %d\n", p.ID)
// 		{
// 			cc, err := s.facade.GetContracts(p.OpenerOrder.ID)
// 			if err != nil {
// 				return err
// 			}
// 			s.logger.Debug("open order id: %d, contract count: %d\n", p.OpenerOrder.ID, len(cc))
// 			for _, c := range cc {
// 				s.logger.Debug("contract: %#v\n", c)
// 			}
// 		}
// 		{
// 			if p.CloserOrder != nil {
// 				cc, err := s.facade.GetContracts(p.CloserOrder.ID)
// 				if err != nil {
// 					return err
// 				}
// 				s.logger.Debug("close order id: %d, contract count: %d\n", p.CloserOrder.ID, len(cc))
// 				for _, c := range cc {
// 					s.logger.Debug("contract: %#v\n", c)
// 				}
// 			}
// 		}
// 	}
//
// 	return nil
// }
//
// // Wait 待機
// func (s *WatchOnlyStrategy) Wait(ctx context.Context) error {
// 	s.logger.Debug("waiting ... (%v)\n", s.interval)
// 	ctx, cancel := context.WithTimeout(ctx, s.interval)
// 	defer cancel()
// 	select {
// 	case <-ctx.Done():
// 		if ctx.Err() != context.Canceled && ctx.Err() != context.DeadlineExceeded {
// 			return ctx.Err()
// 		}
// 		return nil
// 	}
// }
//
// // GetCurrency 対象通貨を取得
// func (s *WatchOnlyStrategy) GetCurrency() model.CurrencyType {
// 	return s.targetCurrency.Key
// }
//
// func (s *WatchOnlyStrategy) loadConfig() error {
// 	var conf watchOnlyConfig
// 	if _, err := toml.DecodeFile(s.configPath, &conf); err != nil {
// 		return err
// 	}
//
// 	s.interval = time.Duration(conf.IntervalSeconds) * time.Second
// 	s.targetCurrency = &model.CurrencyPair{
// 		Key:        model.CurrencyType(conf.TargetCurrency),
// 		Settlement: model.JPY,
// 	}
//
// 	return nil
// }
//
// type watchOnlyConfig struct {
// 	TargetCurrency  string `toml:"target_currency"`
// 	IntervalSeconds int    `toml:"interval_seconds"`
// }
//
