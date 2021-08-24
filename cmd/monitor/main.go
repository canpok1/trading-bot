package main

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"text/template"
	"time"
	"trading-bot/pkg/domain/model"
	"trading-bot/pkg/infrastructure/memory"
	"trading-bot/pkg/infrastructure/mysql"

	"github.com/gorilla/mux"
	"github.com/kelseyhightower/envconfig"
)

//TemperatureDataElem 気温データの一つのデータセット
type TemperatureDataElem struct {
	Label string
	Data  []float64
}

type MonitorConfig struct {
	DB model.DB `required:"true" split_words:"true"`
}

func main() {
	logger := memory.Logger{Level: memory.Debug}
	logger.Info("===== START PROGRAM ====================")
	defer logger.Info("===== END PROGRAM ======================")

	var config MonitorConfig
	if err := envconfig.Process("", &config); err != nil {
		logger.Error(err.Error())
		return
	}

	mysqlCli := mysql.NewClient(config.DB.UserName, config.DB.Password, config.DB.Host, config.DB.Port, config.DB.Name)
	wd, err := os.Getwd()
	if err != nil {
		logger.Error(err.Error())
		return
	}

	r := mux.NewRouter()
	r.HandleFunc("/", rootHandler).Methods(http.MethodGet)
	r.HandleFunc("/dashboard/{pair}", dashboardHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/{pair}", apiHandler(mysqlCli)).Methods(http.MethodGet).Queries("minute", "{minute:[0-9]+}")
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir(wd+"/web/static/"))))

	http.Handle("/", r)
	if err := (http.ListenAndServe(":8080", nil)); err != nil {
		logger.Error("error occured: %v", err)
	}
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	t, err := template.ParseFiles("web/index.html")
	if err != nil {
		panic(err.Error())
	}
	if err := t.Execute(w, nil); err != nil {
		panic(err.Error())
	}
}

func dashboardHandler(w http.ResponseWriter, r *http.Request) {
	t, err := template.ParseFiles("web/dashboard.html")
	if err != nil {
		panic(err.Error())
	}

	p := struct {
		Pair string
	}{}

	vars := mux.Vars(r)
	pairStr := vars["pair"]
	pair, err := model.ParseToCurrencyPair(pairStr)
	if err != nil {
		p.Pair = err.Error()
	} else {
		p.Pair = pair.String()
	}

	if err := t.Execute(w, p); err != nil {
		panic(err.Error())
	}
}

func apiHandler(mysqlCli *mysql.Client) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err, ok := recover().(error); ok {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(struct {
					Error string `json:"error"`
				}{
					Error: err.Error(),
				})
			}
		}()
		w.Header().Set("Content-Type", "application/json")

		pair, err := model.ParseToCurrencyPair(mux.Vars(r)["pair"])
		if err != nil {
			panic(err)
		}
		minute, err := strconv.Atoi(r.URL.Query().Get("minute"))
		if err != nil {
			panic(err)
		}
		duration := time.Duration(minute) * time.Minute

		res := Response{
			Markets: []Market{},
			Events:  []Event{},
		}

		markets, err := mysqlCli.GetMarkets(pair, &duration)
		if err != nil {
			panic(err)
		}
		for _, m := range markets {
			res.Markets = append(res.Markets, Market{
				Datetime:   m.RecordedAt.Format(time.RFC3339),
				SellRate:   m.ExRateSell,
				BuyRate:    m.ExRateBuy,
				SellVolume: m.ExVolumeSell,
				BuyVolume:  m.ExVolumeBuy,
			})
		}

		events, err := mysqlCli.GetEvents(pair, &duration)
		if err != nil {
			panic(err)
		}
		for _, e := range events {
			res.Events = append(res.Events, Event{
				Datetime: e.RecordedAt.Format(time.RFC3339),
				Type:     e.EventType,
				Memo:     e.Memo,
			})
		}

		botStatus, err := mysqlCli.GetBotStatusAll("default", pair)
		if err != nil {
			panic(err)
		}
		res.Statuses = botStatus

		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(res); err != nil {
			panic(err)
		}
	}
}

type Market struct {
	Datetime   string  `json:"datetime"`
	SellRate   float64 `json:"sell_rate"`
	BuyRate    float64 `json:"buy_rate"`
	SellVolume float64 `json:"sell_volume"`
	BuyVolume  float64 `json:"buy_volume"`
}
type Event struct {
	Datetime string `json:"datetime"`
	Type     int    `json:"type"`
	Memo     string `json:"memo"`
}
type Response struct {
	Markets  []Market           `json:"markets"`
	Events   []Event            `json:"events"`
	Statuses map[string]float64 `json:"statuses"`
}
