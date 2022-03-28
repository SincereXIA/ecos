package logger

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"runtime"
	"strings"
)

const (
	Time_Stamp_Format      = "01-02 15:04:05"
	maximumCallerDepth int = 25
	minimumCallerDepth int = 4
)

//GetCallerHook let logrus know real log position
//This method may lead to system crash in the case of concurrency.
//Please use it with caution or Control caller level;set config LogLevelReportCaller
//Get Caller hook
type GetCallerHook struct {
	Field  string
	KipPkg string
	levels []logrus.Level
}

// Levels implement levels
func (hook *GetCallerHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// Fire implement fire
func (hook *GetCallerHook) Fire(entry *logrus.Entry) error {
	entry.Caller = hook.getCaller()
	fileVal := fmt.Sprintf("%s:%d", entry.Caller.File, entry.Caller.Line)
	entry.Data[hook.Field] = fileVal
	return nil
}
func (hook *GetCallerHook) getCaller() *runtime.Frame {
	// Restrict the lookback frames to avoid runaway lookups
	pcs := make([]uintptr, maximumCallerDepth)
	depth := runtime.Callers(minimumCallerDepth, pcs)
	frames := runtime.CallersFrames(pcs[:depth])
	for f, again := frames.Next(); again; f, again = frames.Next() {
		pkg := getPackageName(f.Function)
		// If the caller isn't part of this package, we're done
		if !strings.Contains(hook.GetKipPkg(), pkg) {
			return &f
		}
	}
	// if we got here, we failed to find the caller's context
	return nil
}

// getPackageName reduces a fully qualified function name to the package name
// There really ought to be to be a better way...
func getPackageName(f string) string {
	for {
		lastPeriod := strings.LastIndex(f, ".")
		lastSlash := strings.LastIndex(f, "/")
		if lastPeriod > lastSlash {
			f = f[:lastPeriod]
		} else {
			f = f[lastSlash+1:]
			break
		}
	}

	return f
}
func (hook GetCallerHook) SetKipPkg(args ...string) {
	hook.KipPkg = strings.Join(args, ",")
}
func (hook GetCallerHook) GetKipPkg() string {
	if hook.KipPkg == "" {
		return "logrus,logs,logger"
	}
	return hook.KipPkg
}
