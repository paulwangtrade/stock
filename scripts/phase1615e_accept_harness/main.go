// Temporary acceptance harness for Phase16.15-E (not product code).
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go-stock/backend/api"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/research"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	root := `D:\stock`
	dbPath := filepath.Join(root, `build\bin\data\stock.db`)
	dist := filepath.Join(root, `frontend\dist`)
	addr := "127.0.0.1:18781"

	if _, err := os.Stat(dbPath); err != nil {
		log.Fatalf("db missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dist, "index.html")); err != nil {
		log.Fatalf("dist missing: %v", err)
	}

	dsn := dbPath + "?mode=ro&_pragma=query_only(1)"
	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger:                 logger.Default.LogMode(logger.Silent),
		SkipDefaultTransaction: true,
	})
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	sqlDB, _ := gdb.DB()
	sqlDB.SetMaxOpenConns(1)
	db.Dao = gdb

	list, err := research.ListCandidates(research.ListQuery{})
	if err != nil {
		log.Fatalf("list: %v", err)
	}
	var u26 models.SignalScanSnapshot
	_ = gdb.Select("id, scope, hit_total, trade_date").First(&u26, 26).Error
	latest := data.ListResearchCandidatesFromLatestSnapshot(60)
	fmt.Printf("PROBE snapshot_id=%d trade_date=%s item_count=%d\n", list.SnapshotID, list.TradeDate, len(list.Items))
	fmt.Printf("PROBE row26 id=%d scope=%q hit=%d date=%s\n", u26.ID, u26.Scope, u26.HitTotal, u26.TradeDate)
	fmt.Printf("PROBE latest_data_fn snapshot=%d items=%d\n", latest.SnapshotID, latest.ItemCount)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		path := filepath.Join(dist, filepath.Clean("/"+strings.TrimPrefix(r.URL.Path, "/")))
		if r.URL.Path == "/" || r.URL.Path == "" {
			http.ServeFile(w, r, filepath.Join(dist, "index.html"))
			return
		}
		if st, err := os.Stat(path); err == nil && !st.IsDir() {
			http.ServeFile(w, r, path)
			return
		}
		http.ServeFile(w, r, filepath.Join(dist, "index.html"))
	})

	h := api.ResearchCandidatesAssetMiddleware(next)
	mux := http.NewServeMux()
	mux.Handle("/", h)
	mux.HandleFunc("/__accept/probe", func(w http.ResponseWriter, r *http.Request) {
		out, _ := research.ListCandidates(research.ListQuery{})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"snapshot_id": out.SnapshotID,
			"trade_date":  out.TradeDate,
			"item_count":  len(out.Items),
			"ts":          time.Now().Format(time.RFC3339),
		})
	})
	mux.HandleFunc("/__accept/ui.html", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(acceptUIHTML))
	})

	fmt.Printf("HARNESS listening http://%s  db=%s\n", addr, dbPath)
	log.Fatal(http.ListenAndServe(addr, mux))
}

const acceptUIHTML = `<!DOCTYPE html>
<html lang="zh-CN"><head><meta charset="utf-8"/><title>Phase16.15-E Accept UI</title>
<style>
body{font-family:system-ui,sans-serif;margin:16px;background:#111;color:#eee}
.meta{display:flex;flex-wrap:wrap;gap:12px;margin-bottom:12px;font-size:14px;color:#bbb}
.wrap{min-height:280px;max-height:70vh;overflow:auto;border:1px solid #333;position:relative}
table{width:100%;border-collapse:collapse;font-size:13px}
th,td{padding:6px 8px;border-bottom:1px solid #222;text-align:left}
th{position:sticky;top:0;background:#1a1a1a}
.ok{color:#18a058}.bad{color:#d03050}
</style></head><body>
<h2>研究候选验收页（同源 API / 非产品壳）</h2>
<div class="meta" id="meta">加载中…</div>
<div class="wrap"><table><thead><tr>
<th>代码</th><th>名称</th><th>状态</th><th>信号分</th><th>标签</th><th>方向</th><th>价格</th>
</tr></thead><tbody id="tb"></tbody></table></div>
<script>
(async()=>{
  const r=await fetch('/api/research/candidates');
  const j=await r.json();
  const items=Array.isArray(j.items)?j.items:[];
  const meta=document.getElementById('meta');
  const ok=j.snapshot_id===25 && j.trade_date==='2026-09-03' && items.length===160;
  meta.innerHTML=[
    '交易日：'+ (j.trade_date||'—'),
    '快照 #'+ (j.snapshot_id||'—'),
    '本页 '+ items.length +' 条',
    '阈值：> '+(j.threshold||60),
    ok?'<span class="ok">ACCEPT META OK</span>':'<span class="bad">ACCEPT META FAIL</span>'
  ].join(' · ');
  document.getElementById('tb').innerHTML=items.map(it=>'<tr>'+
    '<td>'+(it.stock_code||'')+'</td>'+
    '<td>'+(it.stock_name||'')+'</td>'+
    '<td>'+(it.status||'')+'</td>'+
    '<td>'+(it.signal_score??'—')+'</td>'+
    '<td>'+(it.signal_tag||'—')+'</td>'+
    '<td>'+(it.direction||'—')+'</td>'+
    '<td>'+(it.price||'—')+'</td>'+
  '</tr>').join('');
})();
</script></body></html>
`
