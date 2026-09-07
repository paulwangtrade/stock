package security

import (
	"strings"
	"testing"
)

func TestRedactSecrets_APIKeyAndToken(t *testing.T) {
	in := `failed call api_key=sk-live-secret123 token=abcDEF123456 Bearer eyJhbGciOiJIUzI1NiJ9`
	out := RedactSecrets(in)
	if strings.Contains(out, "sk-live-secret123") {
		t.Fatalf("api key leaked: %q", out)
	}
	if strings.Contains(out, "abcDEF123456") {
		t.Fatalf("token leaked: %q", out)
	}
	if strings.Contains(out, "eyJhbGciOiJIUzI1NiJ9") {
		t.Fatalf("bearer leaked: %q", out)
	}
	if !strings.Contains(out, RedactedPlaceholder) {
		t.Fatalf("expected placeholder: %q", out)
	}
}

func TestRedactPaths_WindowsAndUnix(t *testing.T) {
	in := `open D:\stock\data\stock.db failed; also /home/user/go-stock/data/stock.db`
	out := RedactPaths(in)
	if strings.Contains(out, `D:\stock`) || strings.Contains(out, "/home/user") {
		t.Fatalf("abs path leaked: %q", out)
	}
	if !strings.Contains(out, "path#") {
		t.Fatalf("expected path hash: %q", out)
	}
}

func TestSanitizeLogLine(t *testing.T) {
	in := `apiKey=sk-abcdefghijklmnop path=C:\Users\alice\AppData\stock.db`
	out := SanitizeLogLine(in)
	if strings.Contains(out, "sk-abcdefghijklmnop") || strings.Contains(out, `C:\Users`) {
		t.Fatalf("leak: %q", out)
	}
}

func TestIsRedactedPlaceholder(t *testing.T) {
	if !IsRedactedPlaceholder("[REDACTED]") {
		t.Fatal("expected true")
	}
	if IsRedactedPlaceholder("real-key") {
		t.Fatal("expected false")
	}
}

func TestPathHint(t *testing.T) {
	h := PathHint(`D:\stock\data\stock.db`)
	if strings.Contains(h, `D:\`) {
		t.Fatalf("hint still absolute: %q", h)
	}
	if !strings.Contains(h, "stock.db") {
		t.Fatalf("want basename: %q", h)
	}
}
