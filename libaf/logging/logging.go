// Package logging provides configurable zap logger creation for Antfly/Termite services.
package logging

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -config cfg.yaml openapi.yaml

import (
	"log"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewLogger creates a zap logger based on the Config settings.
// If config is nil or has empty values, defaults to terminal style with info level.
func NewLogger(c *Config) *zap.Logger {
	var err error
	var logger *zap.Logger

	// Determine logger type based on log style config
	loggingStyle := StyleTerminal     // default
	logLevel := zapcore.InfoLevel     // default

	if c != nil {
		if c.Style != "" {
			loggingStyle = c.Style
		}
		if c.Level != "" {
			lvl, parseErr := zapcore.ParseLevel(string(c.Level))
			if parseErr == nil {
				logLevel = lvl
			}
		}
	}

	switch loggingStyle {
	case StyleNoop:
		logger = zap.NewNop()
	case StyleJson:
		cfg := zap.NewProductionConfig()
		cfg.Level = zap.NewAtomicLevelAt(logLevel)
		logger, err = cfg.Build(
			zap.AddCaller(),
			zap.AddStacktrace(zap.ErrorLevel),
		)
	case StyleTerminal:
		cfg := zap.NewDevelopmentConfig()
		cfg.Level = zap.NewAtomicLevelAt(logLevel)
		logger, err = cfg.Build(
			zap.AddCaller(),
			zap.AddStacktrace(zap.ErrorLevel),
		)
	default:
		log.Fatalf(
			"invalid logging style '%s': must be one of: terminal, json, noop",
			loggingStyle,
		)
	}

	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}
	return logger
}
