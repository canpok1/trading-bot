package memory_test

import (
	"strings"
	"testing"
	"trading-bot/pkg/domain/model"
	"trading-bot/pkg/infrastructure/memory"
)

func TestExchangeMock_NotContract_5step(t *testing.T) {
	rates := []string{
		"日付, 販売所買い価格, 販売所売り価格",
		"2021-02-23T19:27:01Z,200.0,199.0",
		"2021-02-23T19:27:02Z,200.0,199.0",
		"2021-02-23T19:27:02Z,200.0,199.0",
		"2021-02-23T19:27:02Z,200.0,199.0",
		"2021-02-23T19:27:02Z,200.0,199.0",
	}
	r := strings.NewReader(strings.Join(rates, "\n"))
	mock, err := memory.NewExchangeMock(r, 0)
	if err != nil {
		t.Fatal(err.Error())
	}

	var amount, rate float64 = 1.0, 199.0
	order, err := mock.PostOrder(&model.NewOrder{
		Type:            model.Buy,
		Pair:            model.BtcJpy,
		Amount:          &amount,
		Rate:            &rate,
		MarketBuyAmount: nil,
		StopLossRate:    nil,
	})
	if err != nil {
		t.Fatal(err.Error())
	}

	mock.NextStep()
	mock.NextStep()
	mock.NextStep()
	mock.NextStep()

	openOrders, err := mock.GetOpenOrders(&order.Pair)
	if err != nil {
		t.Errorf("error occured in GetOpenOrders\nerror: %v", err)
	}
	if len(openOrders) != 1 {
		t.Errorf("OpenOrders count is wrong\nwant: 1\ngot: %d\ngot detail: %#v", len(openOrders), openOrders)
	}

	contracts, err := mock.GetContracts()
	if err != nil {
		t.Errorf("error occured in GetContracts\nerror: %v", err)
	}
	if len(contracts) != 0 {
		t.Errorf("Contracts count is wrong\nwant: 0\ngot: %d\ngot detail: %#v", len(contracts), contracts)
	}
	contains := false
	for _, c := range contracts {
		if c.OrderID == order.ID {
			contains = true
		}
	}
	if contains {
		t.Errorf("Contract contains is wrong\nwant: false\ngot: %v\ngot detai: %#v", contains, contracts)
	}
}

func TestExchangeMock_CloseBuyOrder_1step(t *testing.T) {
	rates := []string{
		"日付, 販売所買い価格, 販売所売り価格",
		"2021-02-23T19:27:01Z,202.0,201.0",
		"2021-02-23T19:27:02Z,201.0,200.0",
		"2021-02-23T19:27:02Z,200.0,199.0",
	}
	r := strings.NewReader(strings.Join(rates, "\n"))
	mock, err := memory.NewExchangeMock(r, 0)
	if err != nil {
		t.Fatal(err.Error())
	}

	var amount, rate float64 = 1.0, 201.0
	order, err := mock.PostOrder(&model.NewOrder{
		Type:            model.Buy,
		Pair:            model.BtcJpy,
		Amount:          &amount,
		Rate:            &rate,
		MarketBuyAmount: nil,
		StopLossRate:    nil,
	})
	if err != nil {
		t.Fatal(err.Error())
	}

	mock.NextStep()

	openOrders, err := mock.GetOpenOrders(&order.Pair)
	if err != nil {
		t.Errorf("error occured in GetOpenOrders\nerror: %v", err)
	}
	if len(openOrders) > 0 {
		t.Errorf("OpenOrders is not empty\ngot: %#v", openOrders)
	}

	contracts, err := mock.GetContracts()
	if err != nil {
		t.Errorf("error occured in GetContracts\nerror: %v", err)
	}
	if len(contracts) != 1 {
		t.Errorf("Contracts count is wrong\nwant: 1\ngot: %d\ngot detail: %#v", len(contracts), contracts)
	}
	contains := false
	for _, c := range contracts {
		if c.OrderID == order.ID {
			contains = true
		}
	}
	if !contains {
		t.Errorf("Contract is not contains order\ncontracts: %#v", contracts)
	}
}

func TestExchangeMock_CloseBuyOrder_2tep(t *testing.T) {
	rates := []string{
		"日付, 販売所買い価格, 販売所売り価格",
		"2021-02-23T19:27:01Z,202.0,201.0",
		"2021-02-23T19:27:02Z,201.0,200.0",
		"2021-02-23T19:27:02Z,200.0,199.0",
	}
	r := strings.NewReader(strings.Join(rates, "\n"))
	mock, err := memory.NewExchangeMock(r, 0)
	if err != nil {
		t.Fatal(err.Error())
	}

	var amount, rate float64 = 1.0, 201.0
	order, err := mock.PostOrder(&model.NewOrder{
		Type:            model.Buy,
		Pair:            model.BtcJpy,
		Amount:          &amount,
		Rate:            &rate,
		MarketBuyAmount: nil,
		StopLossRate:    nil,
	})
	if err != nil {
		t.Fatal(err.Error())
	}

	mock.NextStep()
	mock.NextStep()

	openOrders, err := mock.GetOpenOrders(&order.Pair)
	if err != nil {
		t.Errorf("error occured in GetOpenOrders\nerror: %v", err)
	}
	if len(openOrders) > 0 {
		t.Errorf("OpenOrders is not empty\ngot: %#v", openOrders)
	}

	contracts, err := mock.GetContracts()
	if err != nil {
		t.Errorf("error occured in GetContracts\nerror: %v", err)
	}
	if len(contracts) != 1 {
		t.Errorf("Contracts count is wrong\nwant: 1\ngot: %d\ngot detail: %#v", len(contracts), contracts)
	}
	contains := false
	for _, c := range contracts {
		if c.OrderID == order.ID {
			contains = true
		}
	}
	if !contains {
		t.Errorf("Contract is not contains order\ncontracts: %#v", contracts)
	}
}

func TestExchangeMock_CloseSellOrder_1step(t *testing.T) {
	rates := []string{
		"日付, 販売所買い価格, 販売所売り価格",
		"2021-02-23T19:27:01Z,200.0,199.0",
		"2021-02-23T19:27:02Z,201.0,200.0",
		"2021-02-23T19:27:02Z,202.0,201.0",
	}
	r := strings.NewReader(strings.Join(rates, "\n"))
	mock, err := memory.NewExchangeMock(r, 0)
	if err != nil {
		t.Fatal(err.Error())
	}

	var amount, rate float64 = 1.0, 200.0
	order, err := mock.PostOrder(&model.NewOrder{
		Type:            model.Sell,
		Pair:            model.BtcJpy,
		Amount:          &amount,
		Rate:            &rate,
		MarketBuyAmount: nil,
		StopLossRate:    nil,
	})
	if err != nil {
		t.Fatal(err.Error())
	}

	mock.NextStep()

	openOrders, err := mock.GetOpenOrders(&order.Pair)
	if err != nil {
		t.Errorf("error occured in GetOpenOrders\nerror: %v", err)
	}
	if len(openOrders) > 0 {
		t.Errorf("OpenOrders is not empty\ngot: %#v", openOrders)
	}

	contracts, err := mock.GetContracts()
	if err != nil {
		t.Errorf("error occured in GetContracts\nerror: %v", err)
	}
	if len(contracts) != 1 {
		t.Errorf("Contracts count is wrong\nwant: 1\ngot: %d\ngot detail: %#v", len(contracts), contracts)
	}
	contains := false
	for _, c := range contracts {
		if c.OrderID == order.ID {
			contains = true
		}
	}
	if !contains {
		t.Errorf("Contract is not contains order\ncontracts: %#v", contracts)
	}
}

func TestExchangeMock_CloseSellOrder_2step(t *testing.T) {
	rates := []string{
		"日付, 販売所買い価格, 販売所売り価格",
		"2021-02-23T19:27:01Z,200.0,199.0",
		"2021-02-23T19:27:02Z,201.0,200.0",
		"2021-02-23T19:27:02Z,202.0,201.0",
	}
	r := strings.NewReader(strings.Join(rates, "\n"))
	mock, err := memory.NewExchangeMock(r, 0)
	if err != nil {
		t.Fatal(err.Error())
	}

	var amount, rate float64 = 1.0, 200.0
	order, err := mock.PostOrder(&model.NewOrder{
		Type:            model.Sell,
		Pair:            model.BtcJpy,
		Amount:          &amount,
		Rate:            &rate,
		MarketBuyAmount: nil,
		StopLossRate:    nil,
	})
	if err != nil {
		t.Fatal(err.Error())
	}

	mock.NextStep()
	mock.NextStep()

	openOrders, err := mock.GetOpenOrders(&order.Pair)
	if err != nil {
		t.Errorf("error occured in GetOpenOrders\nerror: %v", err)
	}
	if len(openOrders) > 0 {
		t.Errorf("OpenOrders is not empty\ngot: %#v", openOrders)
	}

	contracts, err := mock.GetContracts()
	if err != nil {
		t.Errorf("error occured in GetContracts\nerror: %v", err)
	}
	if len(contracts) != 1 {
		t.Errorf("Contracts count is wrong\nwant: 1\ngot: %d\ngot detail: %#v", len(contracts), contracts)
	}
	contains := false
	for _, c := range contracts {
		if c.OrderID == order.ID {
			contains = true
		}
	}
	if !contains {
		t.Errorf("Contract is not contains order\ncontracts: %#v", contracts)
	}
}
