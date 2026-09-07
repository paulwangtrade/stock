package logger

import (
	"go-stock/backend/security"

	"go.uber.org/zap/zapcore"
)

// redactingWriteSyncer scrubs API keys / tokens / abs paths before disk/stdout write.
type redactingWriteSyncer struct {
	inner zapcore.WriteSyncer
}

func (w redactingWriteSyncer) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return w.inner.Write(p)
	}
	clean := security.SanitizeLogLine(string(p))
	n, err := w.inner.Write([]byte(clean))
	if err != nil {
		return n, err
	}
	// Zap expects Write to report the original length; report len(p) on success
	// so callers do not treat truncation as short write.
	return len(p), nil
}

func (w redactingWriteSyncer) Sync() error {
	return w.inner.Sync()
}

func wrapRedact(ws zapcore.WriteSyncer) zapcore.WriteSyncer {
	return redactingWriteSyncer{inner: ws}
}
