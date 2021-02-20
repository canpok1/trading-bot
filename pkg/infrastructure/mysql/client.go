package mysql

import (
	"log"
	"trading-bot/pkg/domain/model"
)

type Client struct {
	UserName string
	Password string
	DBName   string
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
