package logger

import (
	"log/slog"
	"os"
	"runtime"
	"strconv"
)

var logger *slog.Logger

func Init() {
	handlerOptions := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}

	handler := slog.NewTextHandler(os.Stdout, handlerOptions)

	logger = slog.New(handler)

	slog.SetDefault(logger)

	slog.Info("Logger initialized")
}

func getCallerInfo() slog.Attr {
	_, file, line, ok := runtime.Caller(2) // Skip getCallerInfo and the wrapper function
	if !ok {
		return slog.String("source", "unknown")
	}
	return slog.String("source", file+":"+strconv.Itoa(line))
}

func Debug(msg string, args ...any) {
	logger.Debug(msg, append(args, getCallerInfo())...)
}

func Info(msg string, args ...any) {
	logger.Info(msg, append(args, getCallerInfo())...)
}

func Warn(msg string, args ...any) {
	logger.Warn(msg, append(args, getCallerInfo())...)
}

func Error(msg string, args ...any) {
	logger.Error(msg, append(args, getCallerInfo())...)
}

func Fatalf(msg string, args ...any) {
	logger.Error(msg, append(args, getCallerInfo())...)
	os.Exit(1)
}
