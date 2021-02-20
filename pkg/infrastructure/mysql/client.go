package mysql

import "log"

type Client struct {
	UserName string
	Password string
	DBName   string
}

func (c *Client) Update() error {
	log.Println("start mysql.Client Update")
	log.Println("end mysql.Client Update")
	return nil
}
