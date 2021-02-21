package mysql

import (
	"fmt"
	"log"
	"trading-bot/pkg/domain/model"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type Client struct {
	db *gorm.DB
}

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

func (c *Client) GetOpenPositions() ([]model.Position, error) {
	log.Println("*** Unimplemented mysql.Client#GetOpenPositions ***")
	return nil, nil
}

func (c *Client) GetOrders() error {
	log.Println("*** Unimplemented mysql.Client#GetOrders ***")
	return nil
}

func (c *Client) UpsertOrders() error {
	log.Println("*** Unimplemented mysql.Client#UpsertOrders ***")
	return nil
}

func (c *Client) SaveProfit(jpy float32) error {
	log.Println("*** Unimplemented mysql.Client#SaveProfit ***")
	return nil
}
