package slack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type TextMessage struct {
	Text string `json:"text"`
}

type Client struct {
	url string
}

func NewClient(url string) *Client {
	return &Client{
		url: url,
	}
}

func (c *Client) PostMessage(messageObj interface{}) error {
	values, err := json.Marshal(messageObj)
	if err != nil {
		return err
	}

	res, err := http.Post(c.url, "application/json", bytes.NewBuffer(values))
	if err != nil {
		return err
	}

	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("slack response %d error: %s", res.StatusCode, body)
	}

	return nil
}
