# License Compliance — SBOM & Inventory

Engineering-only tooling for Phase 13 License Compliance (L1). **Does not modify runtime product code**, trading chain, database, or strategy logic.

## Quick start (Windows)

```powershell
Set-Location D:\stock
./scripts/license/generate-inventory.ps1
```

Outputs:

| Artifact | Path |
|---|---|
| Go module inventory | `compliance/inventory/go-modules.json` |
| Frontend production inventory | `compliance/inventory/frontend-packages.json` |
| CycloneDX SBOM (optional) | `compliance/inventory/sbom.cyclonedx.json` |
| Human-readable notices | `THIRD_PARTY_NOTICES.md` |

## Pipeline

```text
generate-inventory.ps1
  ├─ go run ./tools/compliance/goinventory     → go-modules.json
  ├─ npm ci (scripts/license) + license-checker (frontend)
  ├─ optional: syft / cyclonedx-gomod
  └─ go run ./tools/compliance/mergenotices    → THIRD_PARTY_NOTICES.md
```

## Policy gate

Forbidden license substrings (GPL/AGPL/LGPL) are configured in `compliance/policy.yaml`.  
`goinventory` writes violations into `go-modules.json` → `violations` array.  
CI/release should fail when violations are non-empty unless allowlisted.

## SBOM formats

| Tool | Input | Output | When |
|---|---|---|---|
| `goinventory` (built-in) | `go list -m all` + module cache LICENSE | JSON | Always |
| `license-checker` | `frontend/node_modules` | JSON | Always |
| `syft` | `build/bin/go-stock.exe` or repo | CycloneDX JSON | Optional (if installed) |
| `cyclonedx-gomod` | `go.mod` | CycloneDX JSON | Optional alternative |

Recommended release bundle:

```text
release/
  go-stock.exe
  THIRD_PARTY_NOTICES.md
  compliance/inventory/go-modules.json
  compliance/inventory/frontend-packages.json
  compliance/inventory/sbom.cyclonedx.json   # if syft available
```

## Isolation

- Tools live under `tools/compliance/` and `scripts/license/`.
- **No imports** from `backend/tradeplan`, `execution`, `broker`, or strategy packages.
- Product License domain (`backend/license`) is unrelated to this copyright compliance pipeline.

## Regeneration cadence

- Every release candidate build
- After any `go.mod` or `frontend/package.json` dependency change
- Before commercial readiness audit sign-off

See also: `compliance/SBOM_SCHEME.md`, `PHASE13_LICENSE_STRATEGY_DESIGN.md`.
