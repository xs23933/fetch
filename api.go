package fetch

import (
	"strings"
	"time"

	"github.com/xs23933/core/v2"
)

// ---------- 兼容你的原有简易函数 ----------

func Get(u string, params ...any) (int, []byte, error) {
	f := New()
	query := map[string]string{}
	hdr := map[string]string{}
	timeoutOverridden := false

	if len(params) > 0 {
		if m, ok := params[0].(map[string]any); ok {
			for key, item := range m {
				switch strings.ToLower(key) {
				case "timeout":
					if d, ok := item.(time.Duration); ok {
						f.Timeout = d
						timeoutOverridden = true
					}
				case "headers":
					if h, ok := item.(map[string]string); ok {
						hdr = h
					}
				case "params":
					if q, ok := item.(map[string]string); ok {
						query = q
					}
				}
			}
		} else if s, ok := params[0].(core.Map); ok {
			for key, item := range s {
				switch strings.ToLower(key) {
				case "timeout":
					if d, ok := item.(time.Duration); ok {
						f.Timeout = d
						timeoutOverridden = true
					}
				case "headers":
					if h, ok := item.(map[string]string); ok {
						hdr = h
					}
				case "params":
					if q, ok := item.(map[string]string); ok {
						query = q
					}
				}
			}
		}
	}
	// 立即生效覆盖超时
	if timeoutOverridden && f.client != nil {
		f.client.Timeout = f.Timeout
	}
	return f.Get(u, query, hdr)
}

func Del(u string, params ...any) (int, []byte, error) {
	f := New()
	query := map[string]string{}
	hdr := map[string]string{}
	timeoutOverridden := false

	if len(params) > 0 {
		if m, ok := params[0].(map[string]any); ok {
			for key, item := range m {
				switch strings.ToLower(key) {
				case "timeout":
					if d, ok := item.(time.Duration); ok {
						f.Timeout = d
						timeoutOverridden = true
					}
				case "headers":
					if h, ok := item.(map[string]string); ok {
						hdr = h
					}
				case "params":
					if q, ok := item.(map[string]string); ok {
						query = q
					}
				}
			}
		} else if s, ok := params[0].(core.Map); ok {
			for key, item := range s {
				switch strings.ToLower(key) {
				case "timeout":
					if d, ok := item.(time.Duration); ok {
						f.Timeout = d
						timeoutOverridden = true
					}
				case "headers":
					if h, ok := item.(map[string]string); ok {
						hdr = h
					}
				case "params":
					if q, ok := item.(map[string]string); ok {
						query = q
					}
				}
			}
		}
	}
	// 立即生效覆盖超时
	if timeoutOverridden && f.client != nil {
		f.client.Timeout = f.Timeout
	}
	return f.Del(u, query, hdr)
}

func ProxyGet(u, proxyURL string, params ...any) (int, []byte, error) {
	f := New(map[string]any{
		"proxy": proxyURL,
	})
	query := map[string]string{}
	hdr := map[string]string{}
	if len(params) > 0 {
		if m, ok := params[0].(map[string]any); ok {
			for key, item := range m {
				switch strings.ToLower(key) {
				case "headers":
					if h, ok := item.(map[string]string); ok {
						hdr = h
					}
				case "params":
					if q, ok := item.(map[string]string); ok {
						query = q
					}
				}
			}
		}
	}
	return f.Get(u, query, hdr)
}

func ProxyPost(u, proxyURL string, params map[string]string, headers ...any) (int, []byte, error) {
	f := New(map[string]any{
		"proxy": proxyURL,
	})
	var hdr map[string]string
	if len(headers) > 0 {
		if h, ok := headers[0].(map[string]string); ok {
			hdr = h
		}
	}
	return f.Post(u, params, hdr)
}

func ProxyPayload(u, proxyURL string, params any, headers ...any) (int, []byte, error) {
	f := New(map[string]any{
		"proxy": proxyURL,
	})
	var hdr map[string]string
	if len(headers) > 0 {
		if h, ok := headers[0].(map[string]string); ok {
			hdr = h
		}
	}
	return f.Payload(u, params, hdr)
}

func Post(u string, params map[string]string, headers ...any) (int, []byte, error) {
	f := New()
	var hdr map[string]string
	if len(headers) > 0 {
		if h, ok := headers[0].(map[string]string); ok {
			hdr = h
		}
	}
	return f.Post(u, params, hdr)
}

func Payload(u string, params any, headers ...any) (int, []byte, error) {
	f := New()
	var hdr map[string]string
	if len(headers) > 0 {
		if h, ok := headers[0].(map[string]string); ok {
			hdr = h
		}
	}
	return f.Payload(u, params, hdr)
}
