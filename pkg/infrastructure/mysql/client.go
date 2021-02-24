package mysql

import (
	"fmt"
	"log"
	"trading-bot/pkg/domain/model"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Client MySQL用クライアント
type Client struct {
	db *gorm.DB
}

// NewClient MySQL用クライアントの生成
func NewClient(userName, password, dbHost string, dbPort int, dbName string) *Client {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8&parseTime=True&loc=Local", userName, password, dbHost, dbPort, dbName)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Got error when connect database, the error is '%v'", err)
	}

	return &Client{
		db: db,
	}
}

// AddOrder 注文情報を追加
func (c *Client) AddOrder(o *model.Order) error {
	return c.db.Create(NewOrder(o, model.Open)).Error
}

// GetOpenOrders 未決済の注文を取得
func (c *Client) GetOpenOrders() ([]model.Order, error) {
	records := []Order{}
	if err := c.db.Find(&records, "status = ?", model.Open).Error; err != nil {
		return nil, err
	}

	orders := []model.Order{}
	for _, r := range records {
		order, err := r.ToDomainModel()
		if err != nil {
			return nil, err
		}
		orders = append(orders, *order)
	}

	return orders, nil
}

// UpdateOrderStatus 注文ステータス更新
func (c *Client) UpdateOrderStatus(orderID uint64, s model.OrderStatus) error {
	return c.db.Model(Order{}).Where("id = ?", orderID).Update("status", int(s)).Error
}

// UpsertContracts 約定情報追加
func (c *Client) UpsertContracts(cons []model.Contract) error {
	if len(cons) == 0 {
		return nil
	}
	records := []Contract{}
	for _, con := range cons {
		records = append(records, *NewContract(&con))
	}
	return c.db.Clauses(clause.OnConflict{UpdateAll: true}).Create(&records).Error
}

// GetContracts 約定情報取得
func (c *Client) GetContracts(orderID uint64) ([]model.Contract, error) {
	records := []Contract{}
	if err := c.db.Where(&Contract{OrderID: orderID}).Find(&records).Error; err != nil {
		return nil, err
	}

	contracts := []model.Contract{}
	for _, r := range records {
		contracts = append(contracts, model.Contract{
			ID:               r.ID,
			OrderID:          r.OrderID,
			Rate:             r.Rate,
			IncreaseCurrency: model.CurrencyType(r.IncreaseCurrency),
			IncreaseAmount:   r.IncreaseAmount,
			DecreaseCurrency: model.CurrencyType(r.DecreaseCurrency),
			DecreaseAmount:   r.DecreaseAmount,
			FeeCurrency:      model.CurrencyType(r.FeeCurrency),
			Fee:              r.FeeAmount,
			Liquidity:        model.LiquidityType(r.Liquidity),
			Side:             model.OrderSide(r.Side),
		})
	}

	return contracts, nil
}
