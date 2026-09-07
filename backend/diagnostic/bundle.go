package diagnostic

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

const bundleReadme = `go-stock DiagnosticBundle (local only)

This package was generated on your machine. It does not upload automatically.
Do not share publicly if it may contain paths or tokens in log tails.

Contents:
  bundle.json           — DiagnosticBundle v2 summary
  diagnostic.json       — full diag-2 report
  config_redacted.json  — redacted settings (if available)
  logs/error_tail.txt   — sanitized error log tail (if available)
  README.txt            — this file

Forbidden by design: holdings, fills, cash, account IDs, API keys, passwords.
`

// WriteBundleZip writes DiagnosticBundle v2 as a local zip archive.
func WriteBundleZip(w io.Writer, surface string) error {
	report := CollectReport(surface)
	bundle := BuildBundleV2(report)

	bundleRaw, err := ToBundleJSON(bundle)
	if err != nil {
		return err
	}
	reportRaw, err := ToReportJSON(report)
	if err != nil {
		return err
	}

	zw := zip.NewWriter(w)
	defer zw.Close()

	if err := writeZipEntry(zw, "bundle.json", bundleRaw); err != nil {
		return err
	}
	if err := writeZipEntry(zw, "diagnostic.json", reportRaw); err != nil {
		return err
	}
	if err := writeZipEntry(zw, "README.txt", []byte(bundleReadme)); err != nil {
		return err
	}
	if configRedactedProvider != nil {
		cfg := strings.TrimSpace(configRedactedProvider())
		if cfg != "" && !looksForbiddenConfig(cfg) {
			if err := writeZipEntry(zw, "config_redacted.json", []byte(cfg)); err != nil {
				return err
			}
		}
	}
	if len(report.LogSummary.ErrorTail) > 0 {
		body := strings.Join(report.LogSummary.ErrorTail, "\n") + "\n"
		if err := writeZipEntry(zw, "logs/error_tail.txt", []byte(body)); err != nil {
			return err
		}
	}
	return zw.Close()
}

func writeZipEntry(zw *zip.Writer, name string, body []byte) error {
	h := &zip.FileHeader{
		Name:     name,
		Method:   zip.Deflate,
		Modified: time.Now().UTC(),
	}
	w, err := zw.CreateHeader(h)
	if err != nil {
		return fmt.Errorf("zip create %s: %w", name, err)
	}
	_, err = io.Copy(w, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("zip write %s: %w", name, err)
	}
	return nil
}

func looksForbiddenConfig(s string) bool {
	l := strings.ToLower(s)
	for _, k := range []string{"api_key", "apikey", "sk-", "gstk-", "password", "sponsorcode"} {
		if strings.Contains(l, k) && !strings.Contains(l, "[redacted]") {
			return true
		}
	}
	return false
}

// WriteBundleZipToFile writes DiagnosticBundle v2 to path (tests/helpers).
func WriteBundleZipToFile(path, surface string) error {
	var buf bytes.Buffer
	if err := WriteBundleZip(&buf, surface); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}
