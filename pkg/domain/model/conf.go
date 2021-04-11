package model

// Config ボット用設定
type Config struct {
	TargetCurrency         string   `required:"true" split_words:"true"`
	RateLogIntervalSeconds int      `required:"true" split_words:"true"`
	PositionCountMax       int      `required:"true" split_words:"true"`
	Exchange               Exchange `required:"true"`
	DB                     DB       `required:"true"`
}

func (c *Config) GetTargetPair(Settlement CurrencyType) *CurrencyPair {
	return &CurrencyPair{
		Key:        CurrencyType(c.TargetCurrency),
		Settlement: Settlement,
	}
}

// Exchange 取引所向け設定
type Exchange struct {
	AccessKey string `required:"true" split_words:"true"`
	SecretKey string `required:"true" split_words:"true"`
}

// DB DB用設定
type DB struct {
	Host     string `required:"true"`
	Port     int    `required:"true"`
	Name     string `required:"true"`
	UserName string `required:"true" split_words:"true"`
	Password string `required:"true"`
}

// SimulatorConfig シミュレーター用設定
type SimulatorConfig struct {
	StrategyName    string  `required:"true" split_words:"true"`
	Slippage        float64 `required:"true" split_words:"true"`
	RateHistoryFile string  `required:"true" split_words:"true"`
}
