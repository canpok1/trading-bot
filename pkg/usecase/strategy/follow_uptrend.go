package strategy

import (
	"context"
	ex "trading-bot/pkg/domain/exchange"
	repo "trading-bot/pkg/domain/repository"
)

// FollowUptrendStrategy 上昇トレンド追従戦略
type FollowUptrendStrategy struct {
	ExClient   ex.Client
	RepoClient repo.Client
}

// Tick 情報更新
func (s *FollowUptrendStrategy) Tick(ctx context.Context) error {
	contracts, err := s.ExClient.GetContracts()
	if err != nil {
		return err
	}
	if err := s.RepoClient.UpdateContracts(contracts); err != nil {
		return err
	}

	orders, err := s.ExClient.GetOpenOrders()
	if err != nil {
		return err
	}
	if err := s.RepoClient.UpsertOrders(orders); err != nil {
		return err
	}

	return nil
}

// Trade 取引実施
func (s *FollowUptrendStrategy) Trade(ctx context.Context) error {
	// 各種情報を更新

	// if ポジションを持ってない {
	//   if 上昇トレンド { 買い指値注文 }
	// } else {
	//   if 買い指値注文が未決済 {
	//     if 現在レートとの差が一定以上離れてる { 買い注文キャンセル }
	//   } else if 買い指値注文が約定、売り指値注文なし {
	//     売り指値注文
	//   } else if 買い指値注文が約定、売り指値注文が未決済 {
	//     if 現在レートとの差が一定以上離れてる { 売り注文キャンセル }
	//   } else if 買い指値注文が約定、売り指値注文が約定 {
	//     利益を計算し、ポジションをクローズ
	//   }
	// }

	return nil
}
