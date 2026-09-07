package data

import (
	"context"
	"fmt"
	"go-stock/backend/logger"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// 仅拉 Cookie 时仍需冷启动浏览器；Phase9-C 稳定性：缩短最小超时，避免 UI 永久 Loading。
const eastMoneyCookieChromedpMinTimeout = 12 * time.Second

// eastMoneyCookieChromedpMaxTimeout 硬上限，防止 CrawlTimeOut+90s 把 Wails 调用拖到数分钟。
const eastMoneyCookieChromedpMaxTimeout = 20 * time.Second

// EastMoneyCookieCacheTTL Cookie 缓存有效期；过期后下次 K 线请求才会再次 chromedp（K 线 HTTP 仍每次都发真实请求）
const EastMoneyCookieCacheTTL = 12 * time.Minute

const quoteEastMoneyPage = "https://quote.eastmoney.com/"

// cookieCacheItem 单个 Cookie 缓存项
type cookieCacheItem struct {
	header string
	expiry time.Time
}

// cookieCache 多页面 Cookie 缓存结构
type cookieCache struct {
	mu    sync.Mutex
	items map[string]*cookieCacheItem // key: cacheKey (browserPath + urlCacheKey)
}

var eastMoneyCookieCache = &cookieCache{
	items: make(map[string]*cookieCacheItem),
}

// cookieFlightCall 同 key 并发 chromedp 只起一次浏览器（singleflight）。
type cookieFlightCall struct {
	wg     sync.WaitGroup
	header string
	err    error
}

var (
	cookieFlightMu sync.Mutex
	cookieFlight   = map[string]*cookieFlightCall{}
)

func clampEastMoneyCookieTimeout(timeout time.Duration) time.Duration {
	if timeout < eastMoneyCookieChromedpMinTimeout {
		timeout = eastMoneyCookieChromedpMinTimeout
	}
	if timeout > eastMoneyCookieChromedpMaxTimeout {
		timeout = eastMoneyCookieChromedpMaxTimeout
	}
	return timeout
}

// doEastMoneyCookieSingleflight 保证同一 cacheKey 仅一个 chromedp 在飞。
func doEastMoneyCookieSingleflight(cacheKey string, fn func() (string, error)) (string, error) {
	cookieFlightMu.Lock()
	if call, ok := cookieFlight[cacheKey]; ok {
		cookieFlightMu.Unlock()
		call.wg.Wait()
		return call.header, call.err
	}
	call := &cookieFlightCall{}
	call.wg.Add(1)
	cookieFlight[cacheKey] = call
	cookieFlightMu.Unlock()

	call.header, call.err = fn()
	call.wg.Done()

	cookieFlightMu.Lock()
	delete(cookieFlight, cacheKey)
	cookieFlightMu.Unlock()
	return call.header, call.err
}

// InvalidateEastMoneyCookieCache 清空 Cookie 缓存（例如切换浏览器路径或调试时可调用）
func InvalidateEastMoneyCookieCache() {
	eastMoneyCookieCache.mu.Lock()
	defer eastMoneyCookieCache.mu.Unlock()
	eastMoneyCookieCache.items = make(map[string]*cookieCacheItem)
}

// cleanExpiredCookies 清理过期的 Cookie 缓存项（内部调用，不暴露）
func cleanExpiredCookies() {
	eastMoneyCookieCache.mu.Lock()
	defer eastMoneyCookieCache.mu.Unlock()

	now := time.Now()
	for key, item := range eastMoneyCookieCache.items {
		if now.After(item.expiry) {
			delete(eastMoneyCookieCache.items, key)
		}
	}
}

// GetEastMoneyCookieCacheInfo 获取当前 Cookie 缓存信息（用于调试/监控）
func GetEastMoneyCookieCacheInfo() map[string]interface{} {
	eastMoneyCookieCache.mu.Lock()
	defer eastMoneyCookieCache.mu.Unlock()

	now := time.Now()
	info := make(map[string]interface{})
	items := make([]map[string]string, 0, len(eastMoneyCookieCache.items))

	for key, item := range eastMoneyCookieCache.items {
		isExpired := now.After(item.expiry)
		items = append(items, map[string]string{
			"cacheKey": key,
			"expiry":   item.expiry.Format(time.RFC3339),
			"expired":  fmt.Sprintf("%v", isExpired),
		})
	}

	info["count"] = len(items)
	info["items"] = items
	info["ttl_minutes"] = fmt.Sprintf("%.0f", EastMoneyCookieCacheTTL.Minutes())

	return info
}

// getURLCacheKey 从 URL 中提取缓存键（排除查询参数和片段）
// 例如：https://quote.eastmoney.com/sz000001.html?foo=bar#top -> https://quote.eastmoney.com/sz000001.html
func getURLCacheKey(pageURL string) string {
	u, err := url.Parse(pageURL)
	if err != nil {
		// 如果解析失败，返回原始 URL
		return pageURL
	}
	// 重建不含查询参数和片段的 URL
	result := u.Scheme + "://" + u.Host + u.Path
	return result
}

// EastMoneyCookieHeaderForPush2his 供所有访问 push2his.eastmoney.com 的 HTTP 请求复用，与 K 线共用 chromedp Cookie 缓存。
// browserPath 为空时自动检测系统浏览器（Edge/Chrome/Firefox），检测失败时返回空串。
func EastMoneyCookieHeaderForPush2his(config *SettingConfig) string {
	if config == nil || config.Settings == nil {
		return ""
	}
	browserPath := strings.TrimSpace(config.BrowserPath)
	crawl := time.Duration(config.CrawlTimeOut) * time.Second
	if crawl <= 0 {
		crawl = 15 * time.Second
	}
	// 稳定性：Cookie 拉取超时与 CrawlTimeOut 解耦上限，避免阻塞顶栏/持仓 UI。
	cdTimeout := clampEastMoneyCookieTimeout(crawl)
	// 获取 push2his 接口的 Cookie，需要访问 quote 页面
	h, err := FetchEastMoneyCookiesViaChromedp(browserPath, cdTimeout, quoteEastMoneyPage)
	if err != nil {
		logger.SugaredLogger.Warnf("东财 chromedp 获取 cookie 失败，push2his 请求将不带 Cookie: %v", err)
		return ""
	}
	return h
}

// FetchEastMoneyCookiesViaChromedp 带缓存：命中则直接返回已缓存的 Cookie 头，不启动浏览器；
// K 线数据不在此函数内请求，调用方须每次对 push2his 发真实 HTTP（见 fetchKLineJSONBytesByHTTP）。
// 该函数为导出版本，供外部包调用。
// pageURL: 需要访问的页面 URL，用于获取该页面的 Cookie（例如：https://quote.eastmoney.com/）
func FetchEastMoneyCookiesViaChromedp(browserPath string, timeout time.Duration, pageURL string) (cookieHeader string, err error) {
	return fetchEastMoneyCookiesViaChromedp(browserPath, timeout, pageURL)
}

// fetchEastMoneyCookiesViaChromedp 带缓存：命中则直接返回已缓存的 Cookie 头，不启动浏览器；
// K 线数据不在此函数内请求，调用方须每次对 push2his 发真实 HTTP（见 fetchKLineJSONBytesByHTTP）。
// browserPath 为空时自动检测系统浏览器（Edge/Chrome/Firefox）
// pageURL: 需要访问的页面 URL，用于获取该页面的 Cookie
// 缓存键为 pageURL 的路径部分（排除查询参数），例如：
//   - https://quote.eastmoney.com/sz000001.html?foo=bar 和 https://quote.eastmoney.com/sz000001.html 共用同一缓存
//   - https://quote.eastmoney.com/ 和 https://quote.eastmoney.com/sz000001.html 使用不同缓存
//
// 支持同时缓存多个页面的 Cookie
func fetchEastMoneyCookiesViaChromedp(browserPath string, timeout time.Duration, pageURL string) (cookieHeader string, err error) {
	browserPath = strings.TrimSpace(browserPath)
	if browserPath == "" {
		// 自动检测系统浏览器
		browserPath, _ = CheckBrowser()
		if browserPath == "" {
			return "", fmt.Errorf("chromedp: 未配置浏览器路径且未检测到系统浏览器 (Edge/Chrome/Firefox)")
		}
		logger.SugaredLogger.Infof("chromedp: 自动检测到浏览器路径：%s", browserPath)
	}
	//logger.SugaredLogger.Debugf("chromedp: 获取 Cookie，浏览器路径：%s，URL：%s", browserPath, pageURL)

	now := time.Now()
	// 使用 URL 路径部分作为缓存键（排除查询参数）
	urlCacheKey := getURLCacheKey(pageURL)
	// 组合键：浏览器路径 + URL 路径，确保不同浏览器和不同页面的 Cookie 独立缓存
	cacheKey := browserPath + "||" + urlCacheKey

	eastMoneyCookieCache.mu.Lock()
	if item, ok := eastMoneyCookieCache.items[cacheKey]; ok && now.Before(item.expiry) {
		eastMoneyCookieCache.mu.Unlock()
		//logger.SugaredLogger.Debugf("东财 Cookie 使用缓存（URL: %s），至 %s 失效", urlCacheKey, item.expiry.Format(time.RFC3339))
		return item.header, nil
	}
	eastMoneyCookieCache.mu.Unlock()

	timeout = clampEastMoneyCookieTimeout(timeout)
	h, err := doEastMoneyCookieSingleflight(cacheKey, func() (string, error) {
		// 进入 flight 后再查一次缓存，避免并发重复起浏览器。
		eastMoneyCookieCache.mu.Lock()
		if item, ok := eastMoneyCookieCache.items[cacheKey]; ok && time.Now().Before(item.expiry) {
			header := item.header
			eastMoneyCookieCache.mu.Unlock()
			return header, nil
		}
		eastMoneyCookieCache.mu.Unlock()

		header, onceErr := eastMoneyCookiesViaChromedpOnce(browserPath, timeout, pageURL)
		if onceErr != nil {
			return "", onceErr
		}
		eastMoneyCookieCache.mu.Lock()
		eastMoneyCookieCache.items[cacheKey] = &cookieCacheItem{
			header: header,
			expiry: time.Now().Add(EastMoneyCookieCacheTTL),
		}
		eastMoneyCookieCache.mu.Unlock()
		return header, nil
	})
	if err != nil {
		return "", err
	}
	return h, nil
}

// eastMoneyCookiesViaChromedpOnce 单次 chromedp 拉 Cookie（无缓存）
// pageURL: 需要访问的页面 URL（可以包含查询参数），用于获取该页面的 Cookie
// 注意：Cookie 缓存键为 URL 路径部分（排除查询参数），但实际访问时使用完整的 pageURL
func eastMoneyCookiesViaChromedpOnce(browserPath string, timeout time.Duration, pageURL string) (cookieHeader string, err error) {
	timeout = clampEastMoneyCookieTimeout(timeout)

	parent, cancelParent := context.WithTimeout(context.Background(), timeout)
	defer cancelParent()

	// 1. 构建无特征的Chrome启动选项
	opts := append(
		chromedp.DefaultExecAllocatorOptions[:],
		//chromedp.ExecPath(browserPath),
		// 禁用自动化扩展
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-extensions-except", ""),
		chromedp.Flag("disable-extensions-file-access-check", true),
		// 禁用自动化提示条
		chromedp.Flag("disable-infobars", true),
		// 禁用开发者工具
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-popup-blocking", true),
		// 禁用图片加载（可选，提升速度）
		chromedp.Flag("blink-settings", "imagesEnabled=false"),
		// 模拟真实用户代理（替换为最新的Chrome UA）
		chromedp.UserAgent(getRandomUA()),
		// 禁用WebDriver特征
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		// 启用图片解码（避免指纹异常）
		chromedp.Flag("enable-features", "NetworkService,NetworkServiceInProcess"),
		// 禁用GPU（避免部分环境报错）
		chromedp.Flag("disable-gpu", true),
		// 禁用沙箱（部分Linux环境需要）
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-webgl", true),
		chromedp.Flag("headless", "new"), // 新版无头模式（更接近真实浏览器）
		// 设置窗口大小（模拟真实屏幕）
		chromedp.WindowSize(1920, 1080),
	)

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(parent, opts...)
	defer cancelAlloc()

	ctx, cancelCtx := chromedp.NewContext(allocCtx,
		chromedp.WithLogf(logger.SugaredLogger.Infof),
		chromedp.WithErrorf(logger.SugaredLogger.Errorf),
	)
	defer cancelCtx()

	var cookies []*network.Cookie
	err = chromedp.Run(ctx,
		chromedp.ActionFunc(func(actx context.Context) error {
			return network.Enable().Do(actx)
		}),
		chromedp.Navigate(quoteEastMoneyPage),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(1000*time.Millisecond),
		chromedp.Navigate(pageURL),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(1000*time.Millisecond),
		chromedp.ActionFunc(func(actx context.Context) error {
			var inner error
			cookies, inner = network.GetCookies().WithURLs([]string{
				pageURL,
			}).Do(actx)
			return inner
		}),
	)
	if err != nil {
		return "", err
	}
	if len(cookies) == 0 {
		return "", nil
	}
	var b strings.Builder
	first := true
	for _, c := range cookies {
		if c == nil || c.Name == "" {
			continue
		}
		if !first {
			b.WriteString("; ")
		}
		first = false
		b.WriteString(c.Name)
		b.WriteByte('=')
		b.WriteString(c.Value)
	}
	return b.String(), nil
}
