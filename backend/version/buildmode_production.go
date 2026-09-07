//go:build production

package version

// Wails production builds use -tags production. Upgrade default build_mode
// without requiring callers to remember an extra ldflag (ldflags still win when set
// to a non-dev value before/after init — see Bootstrap / ApplyOverrides).
func init() {
	if BuildMode == "" || BuildMode == BuildModeDev {
		BuildMode = BuildModeProduction
	}
}
