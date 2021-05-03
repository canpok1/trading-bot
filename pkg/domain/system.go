package domain

import "fmt"

// Logger ロガー
type Logger interface {
	Debug(format string, v ...interface{})
	Info(format string, v ...interface{})
	Error(format string, v ...interface{})
}

func Red(format string, a ...interface{}) string {
	return fmt.Sprintf("\x1b[31m"+format+"\x1b[0m", a...)
}

func Green(format string, a ...interface{}) string {
	return fmt.Sprintf("\x1b[32m"+format+"\x1b[0m", a...)
}

func Yellow(format string, a ...interface{}) string {
	return fmt.Sprintf("\x1b[33m"+format+"\x1b[0m", a...)
}

func Blue(format string, a ...interface{}) string {
	return fmt.Sprintf("\x1b[34m"+format+"\x1b[0m", a...)
}

func Magenta(format string, a ...interface{}) string {
	return fmt.Sprintf("\x1b[35m"+format+"\x1b[0m", a...)
}

func Cyan(format string, a ...interface{}) string {
	return fmt.Sprintf("\x1b[36m"+format+"\x1b[0m", a...)
}

func White(format string, a ...interface{}) string {
	return fmt.Sprintf("\x1b[37m"+format+"\x1b[0m", a...)
}

func Round(v float64) float64 {
	return float64(int(v*10000)) / 10000
}
