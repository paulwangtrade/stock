package version

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ParseManifestJSON parses a version manifest JSON blob.
func ParseManifestJSON(b []byte) (Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return Manifest{}, fmt.Errorf("version: parse manifest: %w", err)
	}
	m.LatestVersion = strings.TrimSpace(m.LatestVersion)
	if m.LatestVersion == "" {
		return Manifest{}, fmt.Errorf("version: manifest missing latest_version")
	}
	return m, nil
}
