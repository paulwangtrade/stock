// Package brand guards Phase 13 brand detachment invariants for product-facing surfaces.
package brand

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ForbiddenTokens are legacy open-source identity markers that must not appear
// in Phase 1 product surfaces (wails metadata, frontend/src, embed roots).
var ForbiddenTokens = []string{
	"sparkmemory",
	"ArvinLovegood",
	"491605333",
	"506808970",
	"qm.qq.com",
	"alipay.jpg",
	"wxpay.jpg",
	"扫码_搜索联合传播样式",
}

// ScanRelativePaths are repo-relative files or directories checked by residual scans (Phase 1 + 1B).
var ScanRelativePaths = []string{
	"wails.json",
	"main.go",
	"app_update.go",
	"app.go",
	"frontend/src",
	"README.md",
	"CONTRIBUTING.md",
	"SECURITY.md",
	"docs",
	"ai-assistant-web",
}

// ResidualHit records one forbidden token occurrence.
type ResidualHit struct {
	Path  string
	Token string
	Line  int
}

// ScanFile reports forbidden tokens in a single file.
func ScanFile(path string, content string) []ResidualHit {
	var hits []ResidualHit
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		for _, token := range ForbiddenTokens {
			if strings.Contains(line, token) {
				hits = append(hits, ResidualHit{
					Path:  path,
					Token: token,
					Line:  i + 1,
				})
			}
		}
	}
	return hits
}

// ScanTree walks a directory (non-recursive into node_modules / wailsjs) and scans text files.
func ScanTree(root string, rel string) ([]ResidualHit, error) {
	abs := filepath.Join(root, filepath.FromSlash(rel))
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		b, err := os.ReadFile(abs)
		if err != nil {
			return nil, err
		}
		return ScanFile(rel, string(b)), nil
	}
	var hits []ResidualHit
	err = filepath.WalkDir(abs, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			name := d.Name()
			if name == "node_modules" || name == "wailsjs" {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".vue", ".js", ".ts", ".json", ".go", ".md":
		default:
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		hits = append(hits, ScanFile(filepath.ToSlash(relPath), string(b))...)
		return nil
	})
	return hits, err
}

// ScanProductSurfaces scans Phase 1 in-scope product paths under repoRoot.
func ScanProductSurfaces(repoRoot string) ([]ResidualHit, error) {
	var all []ResidualHit
	for _, rel := range ScanRelativePaths {
		hits, err := ScanTree(repoRoot, rel)
		if err != nil {
			return nil, fmt.Errorf("scan %s: %w", rel, err)
		}
		all = append(all, hits...)
	}
	return all, nil
}

// WailsIdentity is the expected official packaging metadata for Phase 1.
type WailsIdentity struct {
	AuthorName    string `json:"-"`
	AuthorEmail   string `json:"-"`
	CompanyName   string `json:"-"`
	Copyright     string `json:"-"`
	Comments      string `json:"-"`
	ProductName   string `json:"-"`
}

// ExpectedWailsIdentity returns the canonical go-stock official identity.
func ExpectedWailsIdentity() WailsIdentity {
	return WailsIdentity{
		AuthorName:  "go-stock",
		AuthorEmail: "support@go-stock.app",
		CompanyName: "go-stock",
		Copyright:   "Copyright (c) go-stock",
		Comments:    "go-stock 官方桌面版",
		ProductName: "go-stock",
	}
}

type wailsJSON struct {
	Author struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"author"`
	Info struct {
		CompanyName   string `json:"companyName"`
		ProductName   string `json:"productName"`
		Copyright     string `json:"copyright"`
		Comments      string `json:"comments"`
	} `json:"info"`
}

// ValidateWailsJSON checks wails.json matches official identity and has no legacy tokens.
func ValidateWailsJSON(repoRoot string) error {
	path := filepath.Join(repoRoot, "wails.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	content := string(b)
	for _, token := range ForbiddenTokens {
		if strings.Contains(content, token) {
			return fmt.Errorf("wails.json still contains forbidden token %q", token)
		}
	}
	var doc wailsJSON
	if err := json.Unmarshal(b, &doc); err != nil {
		return err
	}
	exp := ExpectedWailsIdentity()
	if doc.Author.Name != exp.AuthorName {
		return fmt.Errorf("author.name = %q, want %q", doc.Author.Name, exp.AuthorName)
	}
	if doc.Author.Email != exp.AuthorEmail {
		return fmt.Errorf("author.email = %q, want %q", doc.Author.Email, exp.AuthorEmail)
	}
	if doc.Info.CompanyName != exp.CompanyName {
		return fmt.Errorf("info.companyName = %q, want %q", doc.Info.CompanyName, exp.CompanyName)
	}
	if doc.Info.ProductName != exp.ProductName {
		return fmt.Errorf("info.productName = %q, want %q", doc.Info.ProductName, exp.ProductName)
	}
	if doc.Info.Copyright != exp.Copyright {
		return fmt.Errorf("info.copyright = %q, want %q", doc.Info.Copyright, exp.Copyright)
	}
	if !strings.Contains(doc.Info.Comments, exp.Comments) {
		return fmt.Errorf("info.comments should mention %q", exp.Comments)
	}
	return nil
}

// LegacyPaymentImageNames are removed from build/screenshot in Phase 1.
var LegacyPaymentImageNames = []string{
	"alipay.jpg",
	"wxpay.jpg",
}

// ValidatePaymentImagesRemoved ensures legacy payment / wechat QR assets are absent.
func ValidatePaymentImagesRemoved(repoRoot string) error {
	dir := filepath.Join(repoRoot, "build", "screenshot")
	for _, name := range LegacyPaymentImageNames {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return fmt.Errorf("legacy image still present: build/screenshot/%s", name)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		n := e.Name()
		if strings.Contains(n, "扫码") || strings.Contains(n, "传播") {
			return fmt.Errorf("legacy wechat QR image still present: build/screenshot/%s", n)
		}
	}
	return nil
}

// ValidateMainGoEmbed ensures payment QR blobs are not embedded in main.go.
func ValidateMainGoEmbed(repoRoot string) error {
	b, err := os.ReadFile(filepath.Join(repoRoot, "main.go"))
	if err != nil {
		return err
	}
	content := string(b)
	for _, marker := range []string{"alipay.jpg", "wxpay.jpg", "扫码_搜索联合传播样式", "var alipay", "var wxpay", "var wxgzh"} {
		if strings.Contains(content, marker) {
			return fmt.Errorf("main.go still references legacy embed %q", marker)
		}
	}
	return nil
}

// ValidateAppGoNoLegacyChannels ensures runtime entrypoints do not reference legacy operator endpoints.
func ValidateAppGoNoLegacyChannels(repoRoot string) error {
	b, err := os.ReadFile(filepath.Join(repoRoot, "app.go"))
	if err != nil {
		return err
	}
	content := string(b)
	for _, marker := range []string{"sparkmemory", "ArvinLovegood", "491605333", "506808970"} {
		if strings.Contains(content, marker) {
			return fmt.Errorf("app.go still contains legacy marker %q", marker)
		}
	}
	return nil
}
