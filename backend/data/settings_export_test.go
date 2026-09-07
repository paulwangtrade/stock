package data

import (
	"encoding/json"
	"strings"
	"testing"

	"go-stock/backend/security"
)

func TestExportConfigJSON_DefaultRedactsSecrets(t *testing.T) {
	cfg := &SettingConfig{
		Settings: &Settings{
			TushareToken: "ts-secret-token",
			SponsorCode:  "sponsor-xyz",
			DingRobot:    "https://oapi.dingtalk.com/robot/send?access_token=secret",
		},
		AiConfigs: []*AIConfig{
			{Name: "demo", ApiKey: "sk-live-should-not-export", SessionId: "sess-abc"},
		},
	}
	out := ExportConfigJSON(cfg, true)
	if strings.Contains(out, "sk-live-should-not-export") {
		t.Fatalf("api key leaked in export")
	}
	if strings.Contains(out, "ts-secret-token") {
		t.Fatalf("tushare token leaked")
	}
	if strings.Contains(out, "sponsor-xyz") {
		t.Fatalf("sponsor leaked")
	}
	if !strings.Contains(out, security.RedactedPlaceholder) {
		t.Fatalf("expected redacted placeholder: %s", out)
	}
	if cfg.AiConfigs[0].ApiKey != "sk-live-should-not-export" {
		t.Fatalf("live config mutated")
	}
	var parsed SettingConfig
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.AiConfigs[0].ApiKey != security.RedactedPlaceholder {
		t.Fatalf("got %q", parsed.AiConfigs[0].ApiKey)
	}
}
