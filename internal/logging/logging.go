package logging

import (
	"log/slog"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"oracle-fisher/internal/config"
)

const (
	serviceName = "oracle-fisher"
	appName     = "oracle-fisher"
)

// Init configura o logger padrão em NDJSON no stdout.
func Init() {
	env := config.Get("APP_ENV", config.Get("ENV", "production"))
	if env != "development" {
		env = "production"
	}

	level := slog.LevelInfo
	switch strings.ToLower(config.Get("LOG_LEVEL", "info")) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if len(groups) > 0 {
				return a
			}
			switch a.Key {
			case slog.MessageKey:
				a.Key = "message"
			case slog.LevelKey:
				a.Key = "level"
				if lvl, ok := a.Value.Any().(slog.Level); ok {
					name := "info"
					switch {
					case lvl >= slog.LevelError:
						name = "error"
					case lvl >= slog.LevelWarn:
						name = "warn"
					case lvl >= slog.LevelInfo:
						name = "info"
					default:
						name = "debug"
					}
					a.Value = slog.StringValue(name)
				}
			case slog.TimeKey:
				a.Key = "timestamp"
				if t, ok := a.Value.Any().(time.Time); ok {
					a.Value = slog.StringValue(t.UTC().Format("2006-01-02T15:04:05.000Z"))
				}
			}
			return a
		},
	}).WithAttrs([]slog.Attr{
		slog.String("service", serviceName),
		slog.String("app", appName),
		slog.String("env", env),
	})
	slog.SetDefault(slog.New(handler))
}

// Error registra falha com error e stack_trace quando err não é nil.
func Error(msg string, err error, args ...any) {
	if err != nil {
		args = append(args, "error", err.Error(), "stack_trace", strings.TrimSpace(string(debug.Stack())))
	}
	slog.Error(msg, args...)
}
