package logger

import (
	"bytes"
	"strings"
	"testing"

	"go.uber.org/zap/zapcore"
)

type bufSyncer struct {
	buf bytes.Buffer
}

func (b *bufSyncer) Write(p []byte) (int, error) { return b.buf.Write(p) }
func (b *bufSyncer) Sync() error                 { return nil }

func TestRedactingWriteSyncer_MasksSecrets(t *testing.T) {
	inner := &bufSyncer{}
	w := wrapRedact(zapcore.AddSync(inner))
	msg := []byte("call failed api_key=sk-secret-value-here token=abc123456789")
	n, err := w.Write(msg)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(msg) {
		t.Fatalf("want n=%d got %d", len(msg), n)
	}
	got := inner.buf.String()
	if strings.Contains(got, "sk-secret-value-here") || strings.Contains(got, "abc123456789") {
		t.Fatalf("secret leaked: %q", got)
	}
}
