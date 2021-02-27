package model

// Config ボット用設定
type Config struct {
	StrategyName         string   `toml:"strategy_name"`
	TradeIntervalSeconds int      `toml:"trade_interval_seconds"`
	RateHistorySize      int      `toml:"rate_history_size"`
	TargetCurrency       string   `toml:"target_currency"`
	WarmupTimeSeconds    int      `toml:"warmup_time_seconds"`
	Exchange             Exchange `toml:"exchange"`
	DB                   DB       `toml:"db"`
}

// Exchange 取引所向け設定
type Exchange struct {
	AccessKey string `toml:"access_key"`
	SecretKey string `toml:"secret_key"`
}

// DB DB用設定
type DB struct {
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	Name     string `toml:"name"`
	UserName string `toml:"user_name"`
	Password string `toml:"password"`
}

// SimulatorConfig シミュレーター用設定
type SimulatorConfig struct {
	StrategyName    string `toml:"strategy_name"`
	TargetCurrency  string `toml:"target_currency"`
	RateHistorySize int    `toml:"rate_history_size"`
	DB              DB     `toml:"db"`
}
