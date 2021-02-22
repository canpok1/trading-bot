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

func (c *Client) AddOrder(o *model.Order) error {
	return nil
}

func (c *Client) GetOpenOrders() ([]model.Order, error) {
	return nil, nil
}

func (c *Client) UpdateOrderStatus(s model.OrderStatus) error {
	return nil
}

func (c *Client) AddContract(co *model.Contract) error {
	return nil
}

// UpsertOrders 注文情報の新規登録・更新
func (c *Client) UpsertOrders(orders []model.Order) error {
	if len(orders) == 0 {
		return nil
	}

	records := []Order{}
	for _, order := range orders {
		records = append(records, *NewOrder(&order))
	}

	return c.db.Clauses(clause.OnConflict{UpdateAll: true}).Create(&records).Error
}

// UpdateContracts 約定注文情報の更新
func (c *Client) UpdateContracts(contracts []model.Contract) error {
	if len(contracts) == 0 {
		return nil
	}

	ids := []uint64{}
	for _, contract := range contracts {
		ids = append(ids, contract.OrderID)
	}

	return c.db.Model(Order{}).Where("id IN ?", ids).Updates(Order{Status: 1}).Error
}
