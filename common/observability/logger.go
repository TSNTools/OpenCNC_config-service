package observability

import "log"

// Logger is the lightweight logging contract shared across config-service packages.
// Client implements this interface, allowing one dependency for logs and events.
type Logger interface {
	Printf(format string, args ...any)
	Println(args ...any)
	Errorf(format string, args ...any)
	Warnf(format string, args ...any)
	FatalF(format string, args ...any)
	Fatalf(format string, args ...any)
}

type DiscardLogger struct{}

func (DiscardLogger) Printf(string, ...any) {}

func (DiscardLogger) Println(...any) {}

func (DiscardLogger) Errorf(string, ...any) {}

func (DiscardLogger) Warnf(string, ...any) {}

func (DiscardLogger) FatalF(string, ...any) {}

func (DiscardLogger) Fatalf(string, ...any) {}

type StdLoggerWrapper struct {
	*log.Logger
}

func (w *StdLoggerWrapper) Errorf(format string, args ...any) {
	w.Printf("[ERROR] "+format, args...)
}

func (w *StdLoggerWrapper) Warnf(format string, args ...any) {
	w.Printf("[WARN] "+format, args...)
}

func (w *StdLoggerWrapper) FatalF(format string, args ...any) {
	w.Logger.Fatalf(format, args...)
}

func (w *StdLoggerWrapper) Fatalf(format string, args ...any) {
	w.Logger.Fatalf(format, args...)
}

func WrapLogger(l *log.Logger) Logger {
	if l == nil {
		return DiscardLogger{}
	}
	return &StdLoggerWrapper{Logger: l}
}

func NormalizeLogger(logger Logger) Logger {
	if logger == nil {
		return DiscardLogger{}
	}
	return logger
}
