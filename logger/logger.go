package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
)

// global logging
var Logx *logrus.Logger

// typecasts logrus levels based on basic string ID
func logLevelStringSwitch(logLevelString string) logrus.Level {
	switch logLevelString {
	case "debug":
		return logrus.DebugLevel
	case "info":
		return logrus.InfoLevel
	case "warn":
		return logrus.WarnLevel
	case "error":
		return logrus.ErrorLevel
	case "fatal":
		return logrus.FatalLevel
	default:
		return logrus.InfoLevel
	}
}

func LogxWithFields(levelString string, msg string, fields map[string]interface{}) {
	entry := Logx.WithFields(fields)

	level := logLevelStringSwitch(levelString)

	switch level {
	case logrus.DebugLevel:
		entry.Debug(msg)
	case logrus.InfoLevel:
		entry.Info(msg)
	case logrus.WarnLevel:
		entry.Warn(msg)
	case logrus.ErrorLevel:
		entry.Error(msg)
	case logrus.FatalLevel:
		entry.Fatal(msg)
	default:
		entry.Info(msg)
	}
}

func InitLogging(cargoportBase string, defaultLogLevelString string) (logFilePath string) {

	logFilePath = filepath.Join(cargoportBase, "cargoport-main.log")
	logFile, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("ERROR: Failed to initialize logging: %v", err)
		os.Exit(1)
	}

	// init logrus
	Logx = logrus.New()

	// multi-writer to output to .log and stdout
	multiWriter := io.MultiWriter(logFile, os.Stdout)

	Logx.SetOutput((multiWriter))

	Logx.SetFormatter(&logrus.TextFormatter{
		//Logx.SetFormatter(&logrus.JSONFormatter{
		FullTimestamp:   true,
		PadLevelText:    true,
		TimestampFormat: time.RFC3339,
		ForceColors:     true,
	})

	Logx.SetLevel(logLevelStringSwitch(defaultLogLevelString))

	return logFilePath
}
