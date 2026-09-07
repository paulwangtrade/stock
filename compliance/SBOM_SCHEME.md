# License Compliance — SBOM Generation Scheme

| Document | `compliance/SBOM_SCHEME.md` |
|---|---|
| Phase | L1 Engineering (no root LICENSE change) |
| Reference | `PHASE13_LICENSE_STRATEGY_DESIGN.md` §3.5 · §5.5 |

---

## 1. Purpose

Provide a **reproducible, automated** Software Bill of Materials (SBOM) for go-stock releases without coupling to TradePlan, Execution, Broker, or database schema code.

Two legal layers remain separate:

- **Copyright license** — root `LICENSE` (GPL-3.0 today)
- **Product license** — `backend/license` Pro/Enterprise keys (feature gating)

This scheme covers **third-party** components only.

---

## 2. Inventory surfaces

| Surface | Scope | Tool | Output |
|---|---|---|---|
| Go modules | All modules in `go list -m all` | `tools/compliance/goinventory` | `compliance/inventory/go-modules.json` |
| Frontend npm (production) | Packages in shipped `frontend/dist` bundle tree | `license-checker --production` | `compliance/inventory/frontend-packages.json` |
| Binary SBOM (optional) | Built `go-stock.exe` | `syft` | `compliance/inventory/sbom.cyclonedx.json` |
| Human notices | Merged markdown | `tools/compliance/mergenotices` | `THIRD_PARTY_NOTICES.md` |

---

## 3. Release workflow

```text
1. wails build                          # produce binary + embed frontend/dist
2. scripts/license/generate-inventory.ps1
3. Verify go-modules.json violations[] empty (or allowlisted)
4. Attach to release artifact:
     - THIRD_PARTY_NOTICES.md
     - compliance/inventory/*.json
     - (optional) sbom.cyclonedx.json
5. Legal review sign-off (L0 decision: GPL path A vs dual-license B, etc.)
```

---

## 4. CI integration (recommended)

```yaml
# Pseudocode — adapt to your CI runner
steps:
  - run: go test ./tools/compliance/...
  - run: pwsh scripts/license/generate-inventory.ps1 -SkipSyft
  - run: |
      $v = (Get-Content compliance/inventory/go-modules.json | ConvertFrom-Json).violations
      if ($v.Count -gt 0) { exit 1 }
  - upload-artifact: compliance/inventory/
  - upload-artifact: THIRD_PARTY_NOTICES.md
```

**Gate:** fail on new GPL/AGPL/LGPL in Go inventory unless `compliance/policy.yaml` allowlist updated with legal approval.

---

## 5. Tooling layout

```text
compliance/
  policy.yaml              # forbidden licenses + allowlist
  SBOM_SCHEME.md           # this document
  inventory/               # generated JSON artifacts

scripts/license/
  generate-inventory.ps1   # orchestrator
  package.json             # pins license-checker
  README.md

tools/compliance/
  goinventory/             # Go module scanner (stdlib + go list)
  mergenotices/            # Markdown merger
```

**Isolation rule:** `tools/compliance/**` must never import `backend/tradeplan`, `execution`, `broker`, or strategy packages.

---

## 6. Optional advanced SBOM

| Tool | Install | Command |
|---|---|---|
| syft | `choco install syft` / GitHub releases | `syft build/bin/go-stock.exe -o cyclonedx-json=...` |
| cyclonedx-gomod | `go install github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@latest` | `cyclonedx-gomod mod -json -output ...` |
| go-licenses | `go install github.com/google/go-licenses@latest` | `go-licenses save ./... --save_path=THIRD_PARTY/go/` |

Built-in `goinventory` is sufficient for L1; syft adds binary-level verification after embed.

---

## 7. Frontend caveats

- `naive-ui`, `@vicons/*`, `vite` may appear as devDependencies but are **bundled into dist** — always scan production tree after `npm run build`.
- Review dual-license packages (`jszip`: MIT OR GPL-3.0) — document choice in legal memo.
- Bundled fonts: include `frontend/src/assets/fonts/OFL.txt` in release `LICENSES/` folder.

---

## 8. ai-assistant-web (sidecar)

If distributed separately, run inventory against `ai-assistant-web/frontend` with the same `license-checker` pattern and store as `compliance/inventory/frontend-ai-assistant-packages.json`.

---

## 9. Acceptance (L1 engineering)

| # | Criterion |
|---|---|
| 1 | One command regenerates Go + frontend inventories |
| 2 | `THIRD_PARTY_NOTICES.md` updates from machine-readable JSON |
| 3 | Policy violations surfaced in `go-modules.json` |
| 4 | Zero coupling to trading write chain |
| 5 | Documented optional CycloneDX path for release engineering |
