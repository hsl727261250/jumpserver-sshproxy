package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jumpserver/jumpserver-sshproxy/pkg/config"
)

const (
	levelDebug = iota
	levelInfo
	levelWarn
	levelError
)

var (
	mu       sync.RWMutex
	minLevel = levelInfo
	output   = log.New(os.Stdout, "", 0)
)

func Initial() {
	conf := config.GetConf()
	mu.Lock()
	defer mu.Unlock()
	minLevel = parseLevel(conf.LogLevel)

	writers := []io.Writer{os.Stdout}
	logFile, err := os.OpenFile(filepath.Join(conf.LogDirPath, "sshproxy.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		fmt.Printf("Open log file failed: %s\n", err)
	} else {
		writers = append(writers, logFile)
	}
	output = log.New(io.MultiWriter(writers...), "", 0)
}

func Debugf(format string, args ...interface{}) { write(levelDebug, "DEBUG", format, args...) }
func Infof(format string, args ...interface{})  { write(levelInfo, "INFO", format, args...) }
func Warnf(format string, args ...interface{})  { write(levelWarn, "WARN", format, args...) }
func Errorf(format string, args ...interface{}) { write(levelError, "ERROR", format, args...) }

func Fatalf(format string, args ...interface{}) {
	write(levelError, "ERROR", format, args...)
	os.Exit(1)
}

func write(level int, name, format string, args ...interface{}) {
	mu.RLock()
	defer mu.RUnlock()
	if level < minLevel || output == nil {
		return
	}
	output.Printf("%s [%s] %s", time.Now().Format("2006-01-02 15:04:05"), name, fmt.Sprintf(format, args...))
}

func parseLevel(value string) int {
	switch strings.ToUpper(value) {
	case "DEBUG":
		return levelDebug
	case "WARN", "WARNING":
		return levelWarn
	case "ERROR":
		return levelError
	default:
		return levelInfo
	}
}
