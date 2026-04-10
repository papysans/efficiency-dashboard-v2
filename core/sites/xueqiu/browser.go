package xueqiu

import (
	"fmt"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

const defaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// Browser wraps a rod.Browser instance
type Browser struct {
	browser  *rod.Browser
	launcher *launcher.Launcher
	proxyURL string
}

// Config holds browser configuration
type Config struct {
	ProxyURL string // empty string means no proxy
	Headless bool   // true = headless (default), false = headed
}

// New creates a browser instance
func New(cfg Config) (*Browser, error) {
	return newBrowser(cfg.ProxyURL, cfg.Headless)
}

func newBrowser(proxyURL string, headless bool) (*Browser, error) {
	l := launcher.New().Headless(headless)

	if proxyURL != "" {
		l = l.Proxy(proxyURL)
	}

	url, err := l.Launch()
	if err != nil {
		return nil, fmt.Errorf("启动浏览器失败: %w", err)
	}
	browser := rod.New().ControlURL(url)
	if err = browser.Connect(); err != nil {
		return nil, fmt.Errorf("连接浏览器失败: %w", err)
	}

	b := &Browser{
		browser:  browser,
		launcher: l,
		proxyURL: proxyURL,
	}

	return b, nil
}

// GetProxyURL returns the proxy URL in use
func (b *Browser) GetProxyURL() string {
	return b.proxyURL
}

// NewPage creates a new browser page with anti-detection measures applied.
func (b *Browser) NewPage() (*rod.Page, error) {
	page, err := b.browser.Page(proto.TargetCreateTarget{})
	if err != nil {
		return nil, err
	}
	_ = page.SetUserAgent(&proto.NetworkSetUserAgentOverride{
		UserAgent: defaultUserAgent,
	})
	_, _ = page.EvalOnNewDocument(`Object.defineProperty(navigator, 'webdriver', {get: () => undefined});`)
	return page, nil
}

// Close shuts down the browser and cleans up resources
func (b *Browser) Close() error {
	if b.browser != nil {
		if err := b.browser.Close(); err != nil {
			return err
		}
	}
	if b.launcher != nil {
		b.launcher.Kill()
	}
	return nil
}
