package mysql

import (
	"fmt"
	"log"
	"time"
	"trading-bot/pkg/domain/model"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

// Client MySQL用クライアント
type Client struct {
	db *gorm.DB
}

// NewClient MySQL用クライアントの生成
func NewClient(userName, password, dbHost string, dbPort int, dbName string) *Client {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8&%s", userName, password, dbHost, dbPort, dbName, "parseTime=True")
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("Got error when connect database, the error is '%v'", err)
	}

	return &Client{
		db: db,
	}
}

// GetOrder 注文を取得
func (c *Client) GetOrder(orderID uint64) (*model.Order, error) {
	var record Order
	if err := c.db.First(&record, orderID).Error; err != nil {
		return nil, err
	}

	order, err := record.ToDomainModel()
	if err != nil {
		return nil, err
	}

	return order, nil
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

func (c *Client) getPosition(id uint64) (*model.Position, error) {
	var p Position
	if err := c.db.First(&p, id).Error; err != nil {
		return nil, err
	}
	position := model.Position{ID: id}

	var oRecord Order
	if err := c.db.First(&oRecord, p.OpenerOrderID).Error; err != nil {
		return nil, err
	}
	oOrder, err := oRecord.ToDomainModel()
	if err != nil {
		return nil, err
	}
	position.OpenerOrder = oOrder

	if p.CloserOrderID != nil {
		var cRecord Order
		if err := c.db.First(&cRecord, p.CloserOrderID).Error; err != nil {
			return nil, err
		}
		cOrder, err := cRecord.ToDomainModel()
		if err != nil {
			return nil, err
		}
		position.CloserOrder = cOrder
	}

	return &position, nil
}

// UpdateCloserOrderID クローズ注文を更新
func (c *Client) UpdateCloserOrderID(id, closerOrderID uint64) (*model.Position, error) {
	if err := c.db.Model(Position{}).Where("id = ?", id).Update("closer_order_id", closerOrderID).Error; err != nil {
		return nil, err
	}
	return c.getPosition(id)
}

// UpdateStatus 注文ステータス更新
func (c *Client) UpdateStatus(orderID uint64, s model.OrderStatus) error {
	return c.db.Model(Order{}).Where("id = ?", orderID).Update("status", int(s)).Error
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

// AddNewOrder 注文情報を追加
func (c *Client) AddNewOrder(o *model.Order) (*model.Position, error) {
	oRecord := NewOrder(o, model.Open)
	err := c.db.Create(oRecord).Error
	if err != nil {
		return nil, err
	}
	newOrder, err := oRecord.ToDomainModel()
	if err != nil {
		return nil, err
	}

	pRecord := Position{
		OpenerOrderID: oRecord.ID,
		CloserOrderID: nil,
	}
	err = c.db.Create(&pRecord).Error
	if err != nil {
		return nil, err
	}

	return &model.Position{
		ID:          pRecord.ID,
		OpenerOrder: newOrder,
		CloserOrder: nil,
	}, nil
}

// AddSettleOrder 注文情報を追加
func (c *Client) AddSettleOrder(positionID uint64, o *model.Order) (*model.Position, error) {
	oRecord := NewOrder(o, model.Open)
	err := c.db.Create(oRecord).Error
	if err != nil {
		return nil, err
	}
	settleOrder, err := oRecord.ToDomainModel()
	if err != nil {
		return nil, err
	}

	err = c.db.Model(&Position{}).Where("id = ?", positionID).Update("closer_order_id", oRecord.ID).Error
	if err != nil {
		return nil, err
	}

	var pRecord Position
	if err := c.db.First(&pRecord, positionID).Error; err != nil {
		return nil, err
	}

	var nRecord Order
	if err := c.db.First(&nRecord, pRecord.OpenerOrderID).Error; err != nil {
		return nil, err
	}
	newOrder, err := nRecord.ToDomainModel()
	if err != nil {
		return nil, err
	}

	return &model.Position{
		ID:          pRecord.ID,
		OpenerOrder: newOrder,
		CloserOrder: settleOrder,
	}, nil
}

// CancelSettleOrder 決済注文をキャンセル
func (c *Client) CancelSettleOrder(positionID uint64) (*model.Position, error) {
	var pos Position
	if err := c.db.First(&pos, positionID).Error; err != nil {
		return nil, err
	}

	if err := c.db.Model(&Order{}).Where("id = ?", pos.OpenerOrderID).Update("status", 2).Error; err != nil {
		return nil, err
	}

	if err := c.db.Model(&Position{}).Where("id = ?", pos.ID).Update("closer_order_id", nil).Error; err != nil {
		return nil, err
	}

	var o Order
	if err := c.db.First(&o, pos.OpenerOrderID).Error; err != nil {
		return nil, err
	}
	order, err := o.ToDomainModel()
	if err != nil {
		return nil, err
	}

	return &model.Position{
		ID:          pos.ID,
		OpenerOrder: order,
		CloserOrder: nil,
	}, nil
}

// GetOpenPositions ポジションを取得
func (c *Client) GetOpenPositions() ([]model.Position, error) {
	var records []struct {
		ID uint64
	}
	err := c.db.Table("positions").
		Select("positions.id").
		Joins("LEFT JOIN orders ON positions.closer_order_id = orders.id").
		Where("positions.closer_order_id IS NULL OR orders.status = 0").
		Scan(&records).Error
	if err != nil {
		return nil, err
	}

	pp := []model.Position{}
	for _, r := range records {
		p, err := c.getPosition(r.ID)
		if err != nil {
			return nil, err
		}
		pp = append(pp, *p)
	}

	return pp, nil
}

// TruncateAll 全テーブルから全レコードを削除
func (c *Client) TruncateAll() error {
	qq := []string{
		"SET FOREIGN_KEY_CHECKS = 0;",
		"TRUNCATE TABLE profits;",
		"TRUNCATE TABLE positions;",
		"TRUNCATE TABLE contracts;",
		"TRUNCATE TABLE orders;",
		"TRUNCATE TABLE rates;",
		"INSERT INTO profits (amount) VALUES (0);",
		"SET FOREIGN_KEY_CHECKS = 1;",
	}

	for _, q := range qq {
		if err := c.db.Exec(q).Error; err != nil {
			return err
		}
	}
	return nil
}

// GetProfit 利益を取得
func (c *Client) GetProfit() (float64, error) {
	var profit Profit
	if err := c.db.Order("id desc, aggregated_at desc").First(&profit).Error; err != nil {
		return 0, err
	}
	return profit.Amount, nil
}

func (c *Client) AddRates(p *model.CurrencyPair, rate float64, recordedAt time.Time) error {
	r := &Rate{
		Currency:   p.String(),
		Rate:       rate,
		RecordedAt: recordedAt,
	}
	return c.db.Create(&r).Error
}

// GetRate レート取得
func (c *Client) GetRate(p *model.CurrencyPair) (float64, error) {
	var r Rate
	if err := c.db.Where("currency = ?", p.String()).Order("recorded_at DESC").First(&r).Error; err != nil {
		return 0, err
	}
	return r.Rate, nil
}

// GetRates レート取得
func (c *Client) GetRates(p *model.CurrencyPair, d *time.Duration) (rates []float64, err error) {
	var rr []Market
	if d == nil {
		err = c.db.
			Where("pair = ?", p.String()).
			Order("recorded_at").Find(&rr).
			Error
	} else {
		begin := time.Now().Add(-1 * *d)
		err = c.db.
			Where("recorded_at > ? AND pair = ?", begin, p.String()).
			Order("recorded_at").Find(&rr).
			Error
	}
	if err != nil {
		return nil, err
	}

	rates = []float64{}
	for _, r := range rr {
		rates = append(rates, r.ExRateSell)
	}

	return rates, nil
}

// AddMarket 市場情報を追加
func (c *Client) AddMarket(info *Market) error {
	return c.db.Create(&info).Error
}

// GetMarkets
func (c *Client) GetMarkets(p *model.CurrencyPair, d *time.Duration) (markets []Market, err error) {
	if d == nil {
		err = c.db.
			Where("pair = ?", p.String()).
			Order("recorded_at").Find(&markets).
			Error
	} else {
		begin := time.Now().Add(-1 * *d)
		err = c.db.
			Where("recorded_at > ? AND pair = ?", begin, p.String()).
			Order("recorded_at").Find(&markets).
			Error
	}
	return
}

// AddEvent イベント情報を追加
func (c *Client) AddEvent(e *Event) error {
	return c.db.Create(&e).Error
}

// GetEvents
func (c *Client) GetEvents(p *model.CurrencyPair, d *time.Duration) (events []Event, err error) {
	if d == nil {
		err = c.db.
			Where("pair = ?", p.String()).
			Order("recorded_at").Find(&events).
			Error
	} else {
		begin := time.Now().Add(-1 * *d)
		err = c.db.
			Where("recorded_at > ? AND pair = ?", begin, p.String()).
			Order("recorded_at").Find(&events).
			Error
	}
	return
}

// GetAccountInfo
func (c *Client) GetAccountInfo(t AccocuntInfoType) (v float64, err error) {
	r := AccountInfo{}
	err = c.db.Where("type = ?", string(t)).Find(&r).Error
	if err != nil {
		return 0, err
	}
	return r.Value, nil
}

// UpsertAccountInfo
func (c *Client) UpsertAccountInfo(t AccocuntInfoType, v float64) error {
	r := AccountInfo{
		Type:  string(t),
		Value: v,
	}
	return c.db.Clauses(clause.OnConflict{UpdateAll: true}).Create(&r).Error
}

// GetBotStatusAll
func (c *Client) GetBotStatusAll() (map[string]float64, error) {
	m := map[string]float64{}

	rr := []BotStatus{}
	err := c.db.Find(&rr).Error
	if err != nil {
		return m, err
	}

	for _, r := range rr {
		m[r.Type] = r.Value
	}

	return m, nil
}

// UpsertBotInfo
func (c *Client) UpsertBotStatuses(statuses []BotStatus) error {
	if len(statuses) == 0 {
		return nil
	}
	return c.db.Clauses(clause.OnConflict{UpdateAll: true}).Create(&statuses).Error
}
