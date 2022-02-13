package logger

import log "github.com/sirupsen/logrus"

type Logger struct {
}

func Tracef(format string, args ...interface{}) {
	log.Tracef(format, args...)
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
