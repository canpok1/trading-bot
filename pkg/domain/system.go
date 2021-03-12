package domain

// Logger ロガー
type Logger interface {
	Debug(format string, v ...interface{})
	Info(format string, v ...interface{})
	Error(format string, v ...interface{})
}
