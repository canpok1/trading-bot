package memory

import (
	"log"
	"trading-bot/pkg/domain"
)

const (
	Debug = iota
	Info
	Error
)

type Logger struct {
	Level int
}

func (l *Logger) Debug(format string, v ...interface{}) {
	if l.Level > Debug {
		return
	}
	log.Printf("[DEBUG] "+format, v...)
}

func (l *Logger) Info(format string, v ...interface{}) {
	if l.Level > Info {
		return
	}
	log.Printf("[INFO] "+format, v...)
}

func (l *Logger) Error(format string, v ...interface{}) {
	if l.Level > Error {
		return
	}
	level := domain.Red("[ERROR]")

	log.Printf(level+format, v...)
}
