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
	"strings"
	"time"
	"trading-bot/pkg/domain/model"
)

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

func (c *Client) request(method string, u *url.URL, reqBody string) ([]byte, error) {
	nonce := createNonce()
	signature := computeHmac256(nonce, u.String(), reqBody, c.APISecretKey)

	req, err := c.createRequest(method, u.String(), nonce, signature, reqBody)
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

func (c *Client) requestWithValidation(method string, u *url.URL, reqBody string, resJSON interface{}) error {
	body, err := c.request(method, u, reqBody)
	if err != nil {
		return err
	}

	var result struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response body, url: %s, body: %s; error: %w", u.String(), body, err)
	}
	if !result.Success {
		return fmt.Errorf("response is error, url: %s, reqBody: %s, resBody: %s, message: %s;", u.String(), reqBody, body, result.Error)
	}

	return json.Unmarshal(body, resJSON)
}

func createNonce() string {
	nonce := time.Now().UnixNano() / (int64(time.Millisecond) / int64(time.Nanosecond))
	return strconv.FormatInt(nonce, 10)
}

func computeHmac256(nonce, url, payload, secret string) string {
	message := nonce + url + payload
	key := []byte(secret)
	h := hmac.New(sha256.New, key)
	h.Write([]byte(message))
	return hex.EncodeToString(h.Sum(nil))
}

func (c *Client) createRequest(method string, url, nonce, signature, body string) (req *http.Request, err error) {
	if req, err = http.NewRequest(method, url, strings.NewReader(body)); err != nil {
		return
	}

	req.Header.Add("access-key", c.APIAccessKey)
	req.Header.Add("access-nonce", nonce)
	req.Header.Add("access-signature", signature)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("cache-control", "no-cache")
	return
}

func toFloat(s string, def float64) float64 {
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		return v
	}
	return def
}

func toFloatNullable(s string, def *float64) *float64 {
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		return &v
	}
	return def
}

func toRequestString(v *float64) string {
	if v == nil {
		return ""
	}

	integerStr := fmt.Sprintf("%d", int(*v))
	if len(integerStr) >= 5 {
		return integerStr
	}

	// 小数含めて全体で5桁（ドット含め6文字）にする
	s := fmt.Sprintf("%.5f", *v)
	if len(s) < 6 {
		return s
	}
	return s[:6]
}

func toCurrencyPair(s string) model.CurrencyPair {
	splited := strings.Split(s, "_")
	return model.CurrencyPair{
		Key:        model.CurrencyType(splited[0]),
		Settlement: model.CurrencyType(splited[1]),
	}
}
