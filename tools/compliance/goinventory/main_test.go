package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseSPDXFromLicenseText(t *testing.T) {
	t.Parallel()
	require.Equal(t, "MIT", parseSPDXFromLicenseText("MIT License\n\nCopyright (c) Example"))
	require.Equal(t, "Apache-2.0", parseSPDXFromLicenseText("Apache License\nVersion 2.0"))
	require.Equal(t, "BSD-3-Clause", parseSPDXFromLicenseText("BSD 3-Clause License"))
	require.Equal(t, "GPL-3.0", parseSPDXFromLicenseText("GNU GENERAL PUBLIC LICENSE\nVersion 3"))
	require.Equal(t, "OFL-1.1", parseSPDXFromLicenseText("SIL OPEN FONT LICENSE Version 1.1"))
	require.Equal(t, "MIT", parseSPDXFromLicenseText("SPDX-License-Identifier: MIT\n"))
}

func TestDetectLicenseFromDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "LICENSE")
	require.NoError(t, os.WriteFile(p, []byte("MIT License\nCopyright (c) Test"), 0o644))
	lic, files := detectLicense(dir)
	require.Equal(t, "MIT", lic)
	require.Contains(t, files, "LICENSE")
}
