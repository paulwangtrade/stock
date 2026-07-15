package data

import (
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
	"go-stock/backend/models"
)

func TestGetHotConceptPlatesRongziRongquan(t *testing.T) {
	plates := NewStockDataApi().GetHotConceptPlates(30)
	if len(plates) == 0 {
		u := "https://push2.eastmoney.com/api/qt/clist/get?pn=1&pz=5&po=1&np=1&fltt=2&invt=2&fid=f62&fs=m:90+t:3&fields=f12,f14,f2,f3,f62"
		resp, err := resty.New().SetTimeout(8 * time.Second).R().
			SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36").
			SetHeader("Referer", "https://quote.eastmoney.com/").
			Get(u)
		if err != nil {
			t.Fatalf("GetHotConceptPlates empty; direct GET err=%v", err)
		}
		body := resp.Body()
		if len(body) > 300 {
			body = body[:300]
		}
		t.Fatalf("GetHotConceptPlates empty; direct status=%d body=%q", resp.StatusCode(), string(body))
	}
	var code, name string
	for _, p := range plates {
		if p["name"] == "融资融券" {
			code = p["code"].(string)
			name = p["name"].(string)
			break
		}
	}
	if code == "" {
		t.Fatalf("融资融券 not in hot plates, first=%v", plates[0])
	}
	t.Logf("plate code=%s name=%s", code, name)

	res := NewStockDataApi().GetAllStocks(1, 20, "", "", name, code, models.TechnicalIndicators{})
	if res == nil {
		t.Fatal("nil response")
	}
	t.Logf("success=%v count=%d dataLen=%d", res.Success, res.Result.Count, len(res.Result.Data))
	if len(res.Result.Data) == 0 {
		t.Fatalf("expected constituents for %s (%s)", name, code)
	}
}
