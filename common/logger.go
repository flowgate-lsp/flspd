package common

import (
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/natefinch/lumberjack.v2"
)

// InitLogger configures the global zerolog logger with console and file output.
func InitLogger() {
	consoleWriter := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.DateTime,
	}

	fileLogger := &lumberjack.Logger{
		Filename:   "flspd.log",
		MaxSize:    10, // megabytes
		MaxBackups: 3,
		MaxAge:     28,   // days
		Compress:   true, // disabled by default
	}

	multi := zerolog.MultiLevelWriter(consoleWriter, fileLogger)

	log.Logger = zerolog.New(multi).With().Timestamp().Logger()
}
