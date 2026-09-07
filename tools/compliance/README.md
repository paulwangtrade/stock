# Compliance tooling (isolated from product runtime)

Go CLI tools under this directory **must not** import:

- `go-stock/backend/tradeplan`
- `go-stock/backend/execution`
- `go-stock/backend/broker`
- Strategy / controlled trading write-chain packages

## Commands

```powershell
go run ./tools/compliance/goinventory -out compliance/inventory/go-modules.json
go run ./tools/compliance/mergenotices -out THIRD_PARTY_NOTICES.md
go test ./tools/compliance/...
```

Or use the orchestrator: `scripts/license/generate-inventory.ps1`.
