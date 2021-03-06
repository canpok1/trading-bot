package model

// Config ボット用設定
type Config struct {
	RateLogIntervalSeconds int      `required:"true" split_words:"true"`
	Exchange               Exchange `required:"true"`
	DB                     DB       `required:"true"`
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
	StrategyName    string  `toml:"strategy_name"`
	RateHistorySize int     `toml:"rate_history_size"`
	Slippage        float32 `toml:"slippage"`
	RateHistoryFile string  `toml:"rate_history_file"`
}
