package logger

import (
	log "github.com/sirupsen/logrus"
)

type Logger struct {
}

type LogLevel int

const (
	TraceLevel LogLevel = iota
	DebugLevel
	InfoLevel
)

func SetLogLevel(level LogLevel) {
	switch level {
	case TraceLevel:
		log.SetLevel(log.TraceLevel)
		break
	case DebugLevel:
		log.SetLevel(log.DebugLevel)
		break
	case InfoLevel:
		log.SetLevel(log.InfoLevel)
		break
	}
}

func Tracef(format string, args ...interface{}) {
	log.Tracef(format, args...)
}

func Debugf(format string, args ...interface{}) {
	log.Debugf(format, args...)
}

func Infof(format string, args ...interface{}) {
	log.Infof(format, args...)
}

// Errorf Print error message then EXIT
func Errorf(format string, args ...interface{}) {
	log.Errorf("ERROR! "+format, args...)
}
func Warningf(format string, args ...interface{}) {
	log.Warningf(format, args...)
}
