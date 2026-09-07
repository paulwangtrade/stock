package diagnostic

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const maxCrashNames = 5

type crashFile struct {
	name    string
	modTime int64
}

// CollectCrashReportNames returns recent crash report basenames only (no stack content).
func CollectCrashReportNames() (present bool, names []string) {
	dir := filepath.Join("data", "crash_reports")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, nil
	}
	files := make([]crashFile, 0)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, "crash-") || !strings.HasSuffix(name, ".json") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, crashFile{name: name, modTime: info.ModTime().Unix()})
	}
	if len(files) == 0 {
		return false, nil
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime > files[j].modTime
	})
	if len(files) > maxCrashNames {
		files = files[:maxCrashNames]
	}
	names = make([]string, 0, len(files))
	for _, f := range files {
		names = append(names, f.name)
	}
	return true, names
}
