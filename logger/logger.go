package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/adrian-griffin/cargoport/job"
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

// horrific map merging function to merge package presets + additional fields for logging, saving on code repetition
func MergeFields(presetFields map[string]interface{}, addOnFields map[string]interface{}) map[string]interface{} {
	// make new field map equal to the length of both field maps combined
	merged := make(map[string]interface{}, len(presetFields)+len(addOnFields))
	// loop over range of presetFields &
	for key, value := range presetFields {
		merged[key] = value // create new key;value in merged
	}
	for key, value := range addOnFields {
		merged[key] = value // create new key;value in merged
	}
	return merged
}

// core, minimum log fields for all structured logging
func CoreLogFields(context *job.JobContext, pkg string) map[string]interface{} {
	return map[string]interface{}{
		"target":  context.Target,
		"job_id":  context.JobID,
		"package": pkg,
	}
}

// log to both stdout & persistent output with dynamic map for fields
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

func InitLogging(cargoportBase, defaultLogLevelString, logFormat string, logTextColour bool) (logFilePath string) {

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

	if logFormat == "json" {
		Logx.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: time.RFC3339,
		})
	} else {
		Logx.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: time.RFC3339,
			ForceColors:     logTextColour,
			PadLevelText:    true,
		})
	}

	Logx.SetLevel(logLevelStringSwitch(defaultLogLevelString))

	return logFilePath
}
