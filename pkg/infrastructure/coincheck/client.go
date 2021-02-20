package coincheck

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"time"
	"trading-bot/pkg/domain/model"
)

const (
	origin = "https://coincheck.com/"
)

type Client struct {
	APIAccessKey string
	APISecretKey string
}

func (c *Client) makeURL(endpoint string, queries map[string]string) (*url.URL, error) {
	u, err := url.Parse(origin)
	if err != nil {
		return nil, fmt.Errorf("failed parse origin url; origin: %s, error: %w", origin, err)
	}

	u.Path = path.Join(u.Path, endpoint)

	if queries == nil {
		return u, nil
	}

	q := u.Query()
	for k, v := range queries {
		q.Add(k, v)
	}
	u.RawQuery = q.Encode()

	return u, nil
}

func (c *Client) get(u *url.URL) ([]byte, error) {
	nonce := createNonce()
	signature := computeHmac256(nonce, u.String(), "", c.APISecretKey)

	req, err := c.createGetRequest(u.String(), nonce, signature)
	if err != nil {
		return nil, err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	return ioutil.ReadAll(res.Body)
}

//create nonce by milliseconds
func createNonce() string {
	nonce := time.Now().UnixNano() / (int64(time.Millisecond) / int64(time.Nanosecond))
	return strconv.FormatInt(nonce, 10)
}

//create signature
func computeHmac256(nonce, url, payload, secret string) string {
	message := nonce + url + payload
	key := []byte(secret)
	h := hmac.New(sha256.New, key)
	h.Write([]byte(message))
	return hex.EncodeToString(h.Sum(nil))
}

func (c *Client) createGetRequest(url, nonce, signature string) (req *http.Request, err error) {
	if req, err = http.NewRequest("GET", url, nil); err != nil {
		return
	}

	req.Header.Add("access-key", c.APIAccessKey)
	req.Header.Add("access-nonce", nonce)
	req.Header.Add("access-signature", signature)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("cache-control", "no-cache")
	return
}

func (c *Client) GetOrderRate(t model.OrderType, p model.CurrencyPair) (*model.OrderRate, error) {
	var orderType string
	switch t {
	case model.Sell:
		orderType = "sell"
	case model.Buy:
		orderType = "buy"
	}

	var pair string
	switch p {
	case model.BtcJpy:
		pair = "btc_jpy"
	}

	u, err := c.makeURL("/api/exchange/orders/rate", map[string]string{
		"order_type": orderType,
		"pair":       pair,
		"amount":     "1",
	})
	if err != nil {
		return nil, err
	}

	raw, err := c.get(u)
	if err != nil {
		return nil, fmt.Errorf("failed GetOrderRate, t: %v, p: %v; error: %w", t, p, err)
	}

	var unmarshaled struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
		Rate    string `json:"rate"`
		Amount  string `json:"amount"`
		Price   string `json:"price"`
	}
	if err := json.Unmarshal(raw, &unmarshaled); err != nil {
		return nil, fmt.Errorf("failed to parse response of GetOrderRate, t: %v, p: %v, response: %s; error: %w", t, p, raw, err)
	}
	if !unmarshaled.Success {
		return nil, fmt.Errorf("response of GetOrderRate is error, t: %v, p: %v, response: %s; %s", t, p, raw, unmarshaled.Error)
	}

	var rate float64
	if rate, err = strconv.ParseFloat(unmarshaled.Rate, 32); err != nil {
		return nil, fmt.Errorf("failed to parse response of GetOrderRate, t: %v, p: %v; error: %w", t, p, err)
	}

	return &model.OrderRate{
		Pair: p,
		Rate: float32(rate),
	}, nil
}

func (c *Client) GetAccountBalance() (*model.Balance, error) {
	u, err := c.makeURL("/api/accounts/balance", nil)
	if err != nil {
		return nil, err
	}

	raw, err := c.get(u)
	if err != nil {
		return nil, fmt.Errorf("failed GetAccountBalance; error: %w", err)
	}

	var unmarshaled struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
		Jpy     string `json:"jpy"`
		Btc     string `json:"btc"`
	}
	if err := json.Unmarshal(raw, &unmarshaled); err != nil {
		return nil, fmt.Errorf("failed to parse response of GetAccountBalance, response: %s; error: %w", raw, err)
	}
	if !unmarshaled.Success {
		return nil, fmt.Errorf("response of GetAccountBalance is error, response: %s; %s", raw, unmarshaled.Error)
	}

	var jpy, btc float64
	if jpy, err = strconv.ParseFloat(unmarshaled.Jpy, 32); err != nil {
		return nil, fmt.Errorf("failed to parse response of GetAccountBalance; error: %w", err)
	}
	if btc, err = strconv.ParseFloat(unmarshaled.Btc, 32); err != nil {
		return nil, fmt.Errorf("failed to parse response of GetAccountBalance; error: %w", err)
	}

	return &model.Balance{
		Jpy: float32(jpy),
		Btc: float32(btc),
	}, nil
}
