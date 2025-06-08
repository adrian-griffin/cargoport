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

func InitLogging(cargoportBase string) (logFilePath string) {

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
		FullTimestamp:   true,
		PadLevelText:    true,
		TimestampFormat: time.RFC3339,
	})

	Logx.SetLevel(logrus.InfoLevel)

	return logFilePath
}
