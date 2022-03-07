package logger

import (
	"bytes"
	"fmt"
	"github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"runtime/debug"
)

var Logger *logrus.Logger

func init() {
	Logger = NewDefaultLogrus()
	//打印文件和行号
	callerHook := GetCallerHook{
		Field:  "caller",
		levels: nil,
	}
	Logger.AddHook(&callerHook)
	//logrus.SetReportCaller(true)
	//logrus.SetFormatter(&LogFormatter{})
}

func NewDefaultLogrus() *logrus.Logger {
	logger := logrus.New()
	logrus.SetLevel(logrus.DebugLevel)
	logger.SetOutput(os.Stdout)
	//logger.SetOutput(os.Stderr)
	//弃用官方的Caller,由于封装了logrus，打印的级别不是预期效果
	//logger.SetReportCaller(true)
	// (3) 日志格式
	logger.SetFormatter(&logrus.TextFormatter{
		//// CallerPrettyfier can be set by the user to modify the content
		//// of the function and file keys in the data when ReportCaller is
		//// activated. If any of the returned value is the empty string the
		//// corresponding key will be removed from fields.
		//CallerPrettyfier func(*runtime.Frame) (function string, file string)
		//
		//ForceColors: true, //show colors
		//DisableColors:true,//remove colors
		TimestampFormat: "2006-01-02 15:04:05",
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

type LogLevel int

const (
	TraceLevel LogLevel = iota
	DebugLevel
	InfoLevel
)

func SetLogLevel(level LogLevel) {
	switch level {
	case TraceLevel:
		logrus.SetLevel(logrus.TraceLevel)
	case DebugLevel:
		logrus.SetLevel(logrus.DebugLevel)
	case InfoLevel:
		logrus.SetLevel(logrus.InfoLevel)
	}
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
	Logger.Fatalf(format, args...)
}
