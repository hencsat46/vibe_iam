package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Logger struct {
	z *zap.Logger
}

func New() (*Logger, error) {
	cfg := zap.NewProductionConfig()
	cfg.EncoderConfig.TimeKey = "ts"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	z, err := cfg.Build(zap.WithCaller(false))
	if err != nil {
		return nil, err
	}
	return &Logger{z: z}, nil
}

func (l *Logger) WithContext(layer, scope string) *Logger {
	return &Logger{z: l.z.With(zap.String("layer", layer), zap.String("scope", scope))}
}

func (l *Logger) WithMethod(method string) *Logger {
	return &Logger{z: l.z.With(zap.String("method", method))}
}

func (l *Logger) Info(msg string, fields ...zap.Field) {
	l.z.Info(msg, fields...)
}

func (l *Logger) Warn(msg string, fields ...zap.Field) {
	l.z.Warn(msg, fields...)
}

func (l *Logger) Error(msg string, fields ...zap.Field) {
	l.z.Error(msg, fields...)
}

func (l *Logger) Debug(msg string, fields ...zap.Field) {
	l.z.Debug(msg, fields...)
}

func (l *Logger) Sync() {
	_ = l.z.Sync()
}
