package logger

import (
	"bytes"
	"fmt"
	"github.com/sirupsen/logrus"
	"go.etcd.io/etcd/raft/v3"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
)

var Logger *logrus.Logger

func init() {
	Logger = NewDefaultLogrus()
	raft.SetLogger(NewRaftLogger())
	lvl, ok := os.LookupEnv("LOG_LEVEL")
	// LOG_LEVEL not set, let's default to debug
	if !ok {
		lvl = "debug"
	}
	// parse string, this is built-in feature of logrus
	ll, err := logrus.ParseLevel(lvl)
	if err != nil {
		ll = logrus.DebugLevel
	}
	// set global log level
	Logger.SetLevel(ll)
}

func NewRaftLogger() *logrus.Logger {
	logger := NewDefaultLogrus()
	logger.SetFormatter(&logrus.TextFormatter{
		ForceColors:            true,
		FullTimestamp:          false,
		TimestampFormat:        "",
		DisableSorting:         false,
		SortingFunc:            nil,
		DisableLevelTruncation: false,
		PadLevelText:           false,

		QuoteEmptyFields: true,
		CallerPrettyfier: raftCallerPrettyfier,
	})
	return logger
}

func callerPrettyfier(_ *runtime.Frame) (string, string) {
	f := getCaller()
	fullFilename := filepath.Base(f.File)
	fullFunction := f.Function
	function := fullFunction[strings.LastIndex(fullFunction, ".")+1:]
	filename := fullFilename[strings.LastIndex(fullFilename, "/")+1:]
	return fmt.Sprintf("%10.10s │", function), fmt.Sprintf("%15.15s:%.3d", filename, f.Line)
}

func raftCallerPrettyfier(_ *runtime.Frame) (string, string) {
	f := getCaller()
	var function string
	if f != nil {
		fullFilename := filepath.Base(f.File)
		filename := fullFilename[strings.LastIndex(fullFilename, "/")+1:]
		function = fmt.Sprintf("%10.10s │", filename)
	} else {
		function = fmt.Sprintf("%10.10s │", "")
	}
	file := fmt.Sprintf("%15.15s:---", "RAFT")
	return function, file
}

func newLoggerWithPG(pgID uint64) *logrus.Logger {
	logger := NewDefaultLogrus()
	logger.SetFormatter(&logrus.TextFormatter{
		ForceColors:            true,
		FullTimestamp:          false,
		TimestampFormat:        "",
		DisableSorting:         false,
		SortingFunc:            nil,
		DisableLevelTruncation: false,
		PadLevelText:           false,

		QuoteEmptyFields: true,
		CallerPrettyfier: callerPrettyfier,
	})
	logger.WithField("pgID", pgID)
	return logger
}

func NewDefaultLogrus() *logrus.Logger {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	logger.SetOutput(os.Stdout)
	logger.SetReportCaller(true)
	// (3) 日志格式
	logger.SetFormatter(&logrus.TextFormatter{
		ForceColors:            true,
		FullTimestamp:          false,
		TimestampFormat:        "",
		DisableSorting:         false,
		SortingFunc:            nil,
		DisableLevelTruncation: false,
		PadLevelText:           false,

		QuoteEmptyFields: true,
		CallerPrettyfier: callerPrettyfier,
	})
	return logger
}

type LogFormatter struct{}

func (m *LogFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	var b *bytes.Buffer
	if entry.Buffer != nil {
		b = entry.Buffer
	} else {
		b = &bytes.Buffer{}
	}

	timestamp := entry.Time.Format("2006-01-02 15:04:05")
	var newLog string

	//HasCaller()为true才会有调用信息
	if entry.HasCaller() {
		fName := filepath.Base(entry.Caller.File)
		newLog = fmt.Sprintf("[%s] [%s] [%s:%d %s] %s\n",
			timestamp, entry.Level, fName, entry.Caller.Line, entry.Caller.Function, entry.Message)
	} else {
		newLog = fmt.Sprintf("[%s] [%s] %s\n", timestamp, entry.Level, entry.Message)
	}

	b.WriteString(newLog)
	return b.Bytes(), nil
}

func Tracef(format string, args ...interface{}) {
	Logger.Tracef(format, args...)
}

func Debugf(format string, args ...interface{}) {
	Logger.Debugf(format, args...)
}

func Infof(format string, args ...interface{}) {
	Logger.Infof(format, args...)
}

func Warningf(format string, args ...interface{}) {
	Logger.Warningf(format, args...)
}

func Errorf(format string, args ...interface{}) {
	Logger.Errorf("ERROR! "+format, args...)
	debug.PrintStack()
}

// Fatalf Print fatal error message then EXIT
func Fatalf(format string, args ...interface{}) {
	debug.PrintStack()
	Logger.Fatalf(format, args...)
}
