// 扫描自选「强」等信号标签（与前端 icePointSignals 一致，经 Node 计算）
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"go-stock/backend/data"
	"go-stock/backend/db"
)

type barPayload struct {
	Code    string    `json:"code"`
	Name    string    `json:"name"`
	Closes  []float64 `json:"closes"`
	Opens   []float64 `json:"opens"`
	Highs   []float64 `json:"highs"`
	Lows    []float64 `json:"lows"`
	Volumes []float64 `json:"volumes"`
	DayKeys []string  `json:"dayKeys"`
}

func normalizeDay(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "/", "-")
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

func main() {
	root := os.Getenv("STOCK_ROOT")
	if root == "" {
		root = `D:\stock`
	}
	dbPath := filepath.Join(root, `build`, `bin`, `data`, `stock.db`)
	db.Init(dbPath + "?_busy_timeout=10000&_journal_mode=WAL")

	var follows []data.FollowedStock
	db.Dao.Where("is_del = ?", 0).Order("sort asc").Find(&follows)
	if len(follows) == 0 {
		fmt.Println("自选为空")
		return
	}

	api := data.NewEastMoneyKLineApi(&data.SettingConfig{
		Settings: &data.Settings{CrawlTimeOut: 30},
	})
	indexK := api.GetKLineData("000001.SH", "101", "", 800)
	indexClose := map[string]float64{}
	if indexK != nil {
		for _, r := range *indexK {
			k := normalizeDay(r.Day)
			if k != "" {
				indexClose[k] = parseF(r.Close)
			}
		}
	}

	payload := struct {
		Stocks     []barPayload       `json:"stocks"`
		IndexClose map[string]float64 `json:"indexClose"`
	}{IndexClose: indexClose}

	for _, f := range follows {
		code := strings.ToLower(strings.TrimSpace(f.StockCode))
		if code == "" {
			continue
		}
		kl := api.GetKLineData(code, "101", "", 800)
		if kl == nil || len(*kl) < 20 {
			continue
		}
		p := barPayload{Code: code, Name: f.Name}
		for _, r := range *kl {
			c := parseF(r.Close)
			if c == 0 {
				continue
			}
			p.Closes = append(p.Closes, c)
			p.Opens = append(p.Opens, or(c, parseF(r.Open)))
			p.Highs = append(p.Highs, or(c, parseF(r.High)))
			p.Lows = append(p.Lows, or(c, parseF(r.Low)))
			p.Volumes = append(p.Volumes, parseF(r.Volume))
			p.DayKeys = append(p.DayKeys, normalizeDay(r.Day))
		}
		if len(p.Closes) >= 20 {
			payload.Stocks = append(payload.Stocks, p)
		}
	}

	script := filepath.Join(root, "scripts", "scansignals", "scan.ts")
	_ = script
	npx, npxErr := exec.LookPath("npx")
	if npxErr != nil {
		fmt.Fprintf(os.Stderr, "未找到 npx: %v\n", npxErr)
		os.Exit(1)
	}
	in, _ := json.Marshal(payload)
	payloadFile := filepath.Join(root, "_scan_payload.json")
	if err := os.WriteFile(payloadFile, in, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "write payload: %v\n", err)
		os.Exit(1)
	}

	cmd := exec.Command(npx, "tsx", "../scripts/scansignals/scan.ts", payloadFile)
	cmd.Dir = filepath.Join(root, "frontend")
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "scan failed: %v\n%s\n", err, string(out))
		os.Exit(1)
	}
	// 仅输出 Node 结果（Go 日志走 stderr）
	if idx := strings.LastIndex(string(out), "=== 今日"); idx >= 0 {
		fmt.Print(string(out)[idx:])
	} else {
		fmt.Print(string(out))
	}
}

func parseF(s string) float64 {
	var f float64
	fmt.Sscanf(strings.TrimSpace(s), "%f", &f)
	return f
}

func or(a, b float64) float64 {
	if b != 0 {
		return b
	}
	return a
}
