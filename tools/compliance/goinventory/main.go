// Command goinventory scans Go module dependencies and writes a license inventory JSON.
// It does not import application packages (tradeplan, execution, broker, etc.).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type moduleInfo struct {
	Path    string `json:"path"`
	Version string `json:"version"`
	Dir     string `json:"dir"`
	Main    bool   `json:"main,omitempty"`
}

type inventoryEntry struct {
	Module     string   `json:"module"`
	Version    string   `json:"version"`
	Direct     bool     `json:"direct"`
	License    string   `json:"license"`
	LicenseFiles []string `json:"license_files,omitempty"`
	ModuleDir  string   `json:"module_dir,omitempty"`
}

type inventoryReport struct {
	GeneratedAt string           `json:"generated_at"`
	GoVersion   string           `json:"go_version"`
	ModuleRoot  string           `json:"module_root"`
	ModuleCount int              `json:"module_count"`
	Entries     []inventoryEntry `json:"entries"`
	Summary     map[string]int   `json:"summary_by_license"`
	Violations  []string         `json:"violations,omitempty"`
}

func main() {
	out := flag.String("out", "", "output JSON path (default stdout)")
	repo := flag.String("repo", ".", "repository root containing go.mod")
	policyForbidden := flag.String("forbidden", "GPL,AGPL,LGPL", "comma-separated forbidden license substrings")
	flag.Parse()

	report, err := buildReport(filepath.Clean(*repo), strings.Split(*policyForbidden, ","))
	if err != nil {
		fmt.Fprintf(os.Stderr, "goinventory: %v\n", err)
		os.Exit(1)
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "goinventory: marshal: %v\n", err)
		os.Exit(1)
	}

	if *out == "" {
		os.Stdout.Write(data)
		return
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "goinventory: mkdir: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*out, data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "goinventory: write: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "wrote %s (%d modules)\n", *out, report.ModuleCount)
}

func buildReport(repoRoot string, forbidden []string) (*inventoryReport, error) {
	goVer, err := exec.Command("go", "version").Output()
	if err != nil {
		return nil, fmt.Errorf("go version: %w", err)
	}

	modules, direct, err := listModules(repoRoot)
	if err != nil {
		return nil, err
	}

	cacheRoot := moduleCacheRoot()
	entries := make([]inventoryEntry, 0, len(modules))
	summary := map[string]int{}
	var violations []string

	for _, m := range modules {
		if m.Path == "" {
			continue
		}
		isMain := m.Main || m.Path == "go-stock"
		dir := m.Dir
		if dir == "" {
			dir = moduleDir(cacheRoot, m.Path, m.Version)
		}
		lic, files := detectLicense(dir)
		if isMain && lic == "" {
			lic = "PROJECT (see root LICENSE)"
		}
		if lic == "" {
			lic = "UNKNOWN"
		}
		entries = append(entries, inventoryEntry{
			Module:       m.Path,
			Version:      m.Version,
			Direct:       direct[m.Path],
			License:      lic,
			LicenseFiles: files,
			ModuleDir:    dir,
		})
		summary[lic]++
		if isMain {
			continue
		}
		for _, f := range forbidden {
			f = strings.TrimSpace(f)
			if f == "" {
				continue
			}
			if strings.Contains(strings.ToUpper(lic), strings.ToUpper(f)) {
				violations = append(violations, fmt.Sprintf("%s@%s: %s", m.Path, m.Version, lic))
			}
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Module == entries[j].Module {
			return entries[i].Version < entries[j].Version
		}
		return entries[i].Module < entries[j].Module
	})
	sort.Strings(violations)

	return &inventoryReport{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		GoVersion:   strings.TrimSpace(string(goVer)),
		ModuleRoot:  repoRoot,
		ModuleCount: len(entries),
		Entries:     entries,
		Summary:     summary,
		Violations:  violations,
	}, nil
}

func listModules(repoRoot string) ([]moduleInfo, map[string]bool, error) {
	cmd := exec.Command("go", "list", "-m", "-json", "all")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return nil, nil, fmt.Errorf("go list -m -json all: %w", err)
	}

	directSet := map[string]bool{}
	if dout, derr := exec.Command("go", "list", "-m", "-json", "-f", "{{.Path}}").Output(); derr == nil {
		_ = dout
	}
	// Direct requires from go.mod require block
	reqCmd := exec.Command("go", "list", "-m", "-f", "{{if not .Indirect}}{{.Path}}{{end}}", "all")
	reqCmd.Dir = repoRoot
	if reqOut, reqErr := reqCmd.Output(); reqErr == nil {
		for _, line := range strings.Split(string(reqOut), "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				directSet[line] = true
			}
		}
	}

	var modules []moduleInfo
	dec := json.NewDecoder(strings.NewReader(string(out)))
	for dec.More() {
		var m moduleInfo
		if err := dec.Decode(&m); err != nil {
			return nil, nil, fmt.Errorf("decode module: %w", err)
		}
		modules = append(modules, m)
	}
	return modules, directSet, nil
}

func moduleCacheRoot() string {
	if gopath := os.Getenv("GOMODCACHE"); gopath != "" {
		return gopath
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "go", "pkg", "mod")
}

func moduleDir(cacheRoot, path, version string) string {
	if cacheRoot == "" || path == "" || version == "" {
		return ""
	}
	escaped := strings.ReplaceAll(path, "/", string(filepath.Separator))
	dir := filepath.Join(cacheRoot, escaped+"@"+version)
	if st, err := os.Stat(dir); err == nil && st.IsDir() {
		return dir
	}
	return dir
}

func detectLicense(dir string) (string, []string) {
	if dir == "" {
		return "", nil
	}
	names := []string{
		"LICENSE", "LICENSE.md", "LICENSE.txt", "LICENSE-MIT", "LICENSE-APACHE",
		"Licence", "Licence.md", "COPYING", "COPYING.txt", "UNLICENSE",
	}
	var foundFiles []string
	for _, name := range names {
		p := filepath.Join(dir, name)
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		foundFiles = append(foundFiles, name)
		if spdx := parseSPDXFromLicenseText(string(b)); spdx != "" {
			return spdx, foundFiles
		}
	}
	// Fallback: any file starting with LICENSE
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		upper := strings.ToUpper(e.Name())
		if strings.HasPrefix(upper, "LICENSE") || upper == "COPYING" {
			p := filepath.Join(dir, e.Name())
			b, err := os.ReadFile(p)
			if err != nil {
				continue
			}
			foundFiles = append(foundFiles, e.Name())
			if spdx := parseSPDXFromLicenseText(string(b)); spdx != "" {
				return spdx, foundFiles
			}
		}
	}
	if len(foundFiles) > 0 {
		return "UNKNOWN (see license file)", foundFiles
	}
	return "", nil
}

func parseSPDXFromLicenseText(text string) string {
	upper := strings.ToUpper(text)
	switch {
	case strings.Contains(upper, "APACHE LICENSE") && strings.Contains(upper, "VERSION 2"):
		return "Apache-2.0"
	case strings.Contains(upper, "MIT LICENSE") || strings.HasPrefix(strings.TrimSpace(upper), "MIT"):
		return "MIT"
	case strings.Contains(upper, "BSD 3-CLAUSE") || strings.Contains(upper, "BSD-3-CLAUSE"):
		return "BSD-3-Clause"
	case strings.Contains(upper, "BSD 2-CLAUSE") || strings.Contains(upper, "BSD-2-CLAUSE"):
		return "BSD-2-Clause"
	case strings.Contains(upper, "ISC LICENSE") || strings.Contains(upper, "THE ISC LICENSE"):
		return "ISC"
	case strings.Contains(upper, "MOZILLA PUBLIC LICENSE") && strings.Contains(upper, "2.0"):
		return "MPL-2.0"
	case strings.Contains(upper, "GNU GENERAL PUBLIC LICENSE"):
		if strings.Contains(upper, "VERSION 3") {
			return "GPL-3.0"
		}
		if strings.Contains(upper, "VERSION 2") {
			return "GPL-2.0"
		}
		return "GPL"
	case strings.Contains(upper, "GNU LESSER GENERAL PUBLIC LICENSE"):
		return "LGPL"
	case strings.Contains(upper, "UNLICENSE"):
		return "Unlicense"
	case strings.Contains(upper, "SIL OPEN FONT LICENSE"):
		return "OFL-1.1"
	}
	// SPDX identifier line
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToUpper(line), "SPDX-LICENSE-IDENTIFIER:") {
			return strings.TrimSpace(line[len("SPDX-License-Identifier:"):])
		}
	}
	return ""
}
