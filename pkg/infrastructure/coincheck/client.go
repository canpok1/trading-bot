package coincheck

import (
	"log"
	ex "trading-bot/pkg/domain/exchange"
	"trading-bot/pkg/domain/model"
)

type Client struct {
	APIAccessKey string
	APISecretKey string
}

func (c *Client) GetOrderRate(t model.OrderType, p model.CurrencyPair) *ex.ResponseGetOrderRate {
	log.Println("start exchange.Client GetOrderRate")
	log.Println("end exchange.Client GetOrderRate")
	return nil
}
