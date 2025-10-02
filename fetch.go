package fetch

import (
	"bytes"
	"compress/gzip"
	"crypto/tls"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/bytedance/sonic"
)

type Fetch struct {
	client         *http.Client
	Transport      *http.Transport
	Timeout        time.Duration
	Jar            http.CookieJar
	UserAgent      string
	mu             sync.RWMutex
	headers        map[string]string // 每个 Fetch 实例自己的基础 headers
	user           string
	password       string
	Header         http.Header // 最近一次响应的 headers
	disableCookies bool        // 禁用 Cookie 同步
	GlobalRespHook RespHook    // 全局统一响应 Hook
}

func (fetch *Fetch) SetRespHook(hook RespHook) {
	fetch.GlobalRespHook = hook
}

func (fetch *Fetch) DisableCookies() {
	fetch.disableCookies = true
}

// ===== Constructor =====
func New(options ...any) *Fetch {
	jar := NewCookieJar()
	fetch := &Fetch{
		UserAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36",
		Jar:       jar,
		Transport: &http.Transport{
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
			DisableKeepAlives: true,
		},
		headers: make(map[string]string),
		Timeout: time.Second * 110,
	}

	if len(options) > 0 {
		if opts, ok := options[0].(map[string]any); ok {
			for k, v := range opts {
				switch k {
				case "userAgent":
					fetch.UserAgent = v.(string)
				case "proxy":
					if px, err := url.Parse(v.(string)); err == nil {
						if px.Scheme == "http" {
							fetch.Transport.Proxy = http.ProxyURL(px)
						} else {
							fetch.Transport = Socks5Proxy(px.Host)
						}
					}
				case "headers":
					if hdrs, ok := v.(map[string]string); ok {
						fetch.SetHeaders(hdrs)
					}
				case "Timeout", "timeout":
					if t, ok := v.(time.Duration); ok {
						fetch.Timeout = t
					}
				}
			}
		}
	}
	return fetch
}

// ===== Headers =====

func (fetch *Fetch) SetHeaders(headers map[string]string) {
	fetch.mu.Lock()
	defer fetch.mu.Unlock()
	for k, v := range headers {
		fetch.headers[k] = v
	}
}

func (fetch *Fetch) getBaseHeaders() map[string]string {
	fetch.mu.RLock()
	defer fetch.mu.RUnlock()
	copyMap := make(map[string]string, len(fetch.headers))
	for k, v := range fetch.headers {
		copyMap[k] = v
	}
	return copyMap
}

func (fetch *Fetch) GetHeader(key string) string {
	return fetch.Header.Get(key)
}

// ===== Proxy =====
func (fetch *Fetch) SetProxy(proxy string) error {
	proxy = strings.ToLower(proxy)
	if px, err := url.Parse(proxy); err == nil {
		if px.Scheme == "http" {
			fetch.Transport.Proxy = http.ProxyURL(px)
		} else {
			fetch.Transport = Socks5Proxy(px.Host)
		}
		return nil
	} else {
		return err
	}
}

// ===== GET =====
func (fetch *Fetch) Get(u string, params ...any) (code int, buf []byte, err error) {
	addr, err := url.Parse(u)
	if err != nil {
		return
	}

	skippedFirstMap := false
	for i, p := range params {
		if m, ok := p.(map[string]string); ok {
			q := addr.Query()
			for k, v := range m {
				q.Set(k, v)
			}
			addr.RawQuery = q.Encode()
			skippedFirstMap = true
			// 把剩余参数原样传下去
			var extra []any
			extra = append(extra, params[:i]...)
			extra = append(extra, params[i+1:]...)
			req, _ := http.NewRequest("GET", addr.String(), nil)
			return fetch.do(req, extra...)
		}
	}
	// 没传 query 的情况，直接透传所有额外参数
	req, _ := http.NewRequest("GET", addr.String(), nil)
	if !skippedFirstMap && len(params) > 0 {
		return fetch.do(req, params...)
	}
	return fetch.do(req)
}

// ===== Delete =====
func (fetch *Fetch) Del(u string, params ...any) (code int, buf []byte, err error) {
	addr, err := url.Parse(u)
	if err != nil {
		return
	}

	skippedFirstMap := false
	for i, p := range params {
		if m, ok := p.(map[string]string); ok {
			q := addr.Query()
			for k, v := range m {
				q.Set(k, v)
			}
			addr.RawQuery = q.Encode()
			skippedFirstMap = true
			// 把剩余参数原样传下去
			var extra []any
			extra = append(extra, params[:i]...)
			extra = append(extra, params[i+1:]...)
			req, _ := http.NewRequest("DELETE", addr.String(), nil)
			return fetch.do(req, extra...)
		}
	}
	// 没传 query 的情况，直接透传所有额外参数
	req, _ := http.NewRequest("DELETE", addr.String(), nil)
	if !skippedFirstMap && len(params) > 0 {
		return fetch.do(req, params...)
	}
	return fetch.do(req)
}

// ===== POST =====
func (fetch *Fetch) Post(u string, params map[string]string, args ...any) (code int, buf []byte, err error) {
	addr, err := url.Parse(u)
	if err != nil {
		log.Println(err.Error())
		return
	}

	form := url.Values{}
	for k, v := range params {
		form.Add(k, v)
	}

	req, _ := http.NewRequest("POST", addr.String(), strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	return fetch.do(req, args...)
}

// ===== Payload (JSON POST) =====
var paramPool = sync.Pool{New: func() any { return &bytes.Buffer{} }}

func (fetch *Fetch) Payload(u string, params any, args ...any) (code int, buf []byte, err error) {
	addr, err := url.Parse(u)
	if err != nil {
		log.Println(err.Error())
		return
	}

	param := paramPool.Get().(*bytes.Buffer)
	defer paramPool.Put(param)
	param.Reset()

	if params != nil {
		switch v := params.(type) {
		case string:
			param.WriteString(v)
		case []byte:
			param.Write(v)
		default:
			if b, e := sonic.Marshal(v); e == nil {
				param.Write(b)
			}
		}
	}
	req, _ := http.NewRequest("POST", addr.String(), param)
	req.Header.Add("Content-Type", "application/json; charset=utf-8")
	return fetch.do(req, args...)
}

// ===== BasicAuth =====
func (fetch *Fetch) BasicAuth(us, pw string) {
	fetch.user = us
	fetch.password = pw
}

// ===== Core HTTP Do =====
func (fetch *Fetch) do(req *http.Request, args ...any) (code int, buf []byte, err error) {
	var reqHook ReqHook = defaultReqHook
	var respHook RespHook = defaultResHook

	tempHeaders := make(map[string]string)
	noCookie := false
	for _, arg := range args {
		switch v := arg.(type) {
		case ReqHook:
			reqHook = v
		case RespHook:
			respHook = v
		case func(*http.Request, []byte):
			reqHook = v
		case func(int, []byte, error) ([]byte, error):
			respHook = v
		case map[string]string:
			for k, val := range v {
				if strings.ToLower(k) == "__nocookie__" && strings.ToLower(val) == "true" {
					noCookie = true
					continue
				}
				tempHeaders[k] = val
			}
		}
	}

	wrappedRespHook := func(code int, body []byte, err error) ([]byte, error) {
		b, e := respHook(code, body, err)
		if fetch.GlobalRespHook != nil {
			return fetch.GlobalRespHook(code, b, e)
		}
		return b, e
	}

	req.Header.Set("User-Agent", fetch.UserAgent)
	req.Header.Set("Accept-Language", "en")
	// 合并 headers
	for k, v := range fetch.getBaseHeaders() {
		req.Header.Set(k, v)
	}
	for k, v := range tempHeaders {
		req.Header.Set(k, v)
	}

	if fetch.client == nil {
		fetch.client = &http.Client{
			Timeout:   fetch.Timeout,
			Jar:       fetch.Jar,
			Transport: fetch.Transport,
		}
	} else {
		fetch.client.Transport = fetch.Transport
	}

	// 如果禁用 Cookie，同步一个空 Jar
	if fetch.disableCookies || noCookie {
		fetch.client.Jar = NewCookieJar() // 新建空的 CookieJar
	} else {
		fetch.client.Jar = fetch.Jar
	}

	if len(fetch.user) > 0 && len(fetch.password) > 0 {
		req.SetBasicAuth(fetch.user, fetch.password)
		fetch.user, fetch.password = "", ""
	}

	// 读取原始 body 供 reqHook 使用，并保存以便重试时复位
	var savedBody []byte
	if req.Body != nil {
		savedBody, _ = io.ReadAll(req.Body)
		// 调用 reqHook（可添加签名头等），随后恢复 body（如果 hook 想自定义 body，也可在 hook 里自行设置 req.Body）
		reqHook(req, savedBody)
		// 如果 hook 没改 body，这里恢复原始；若 hook 改了 body，也不会影响我们再次设置 —— 因为我们需要保证请求可重试
	}
	// 重试前置：构造一个函数在每次 attempt 之前复位 body
	resetBody := func() {
		if savedBody != nil {
			req.Body = io.NopCloser(bytes.NewReader(savedBody))
		} else {
			req.Body = nil
		}
	}

	maxAttempts := 3 // 默认最多3次尝试
	var lastErr error
	for attempt := range maxAttempts {
		// 每次尝试前复位 body
		resetBody()

		resp, e := fetch.client.Do(req)

		if e != nil {
			e = unwrapNetError(e) // 统一处理
			lastErr = e
			if shouldRetry(e) && attempt < maxAttempts {
				duration := 200 * time.Duration(1<<attempt+1) * time.Millisecond
				// core.Erro("%s 后重新尝试[%d/%d]: %s", duration, attempt+1, maxAttempts, e.Error())
				time.Sleep(duration)
				continue
			}
			buf, e = wrappedRespHook(code, nil, e)
			return 0, buf, e
		}
		if resp == nil {
			if attempt < maxAttempts {
				duration := 200 * time.Duration(1<<attempt+1) * time.Millisecond
				// core.Erro("%s 后重新尝试[%d/%d]: [%s] resp is null", duration, attempt+1, maxAttempts, resp)
				time.Sleep(duration)
				continue
			}
			lastErr = errors.New("network err or server invalid")
			buf, err = wrappedRespHook(code, nil, lastErr)
			return 0, buf, err
		}

		code = resp.StatusCode
		fetch.Header = resp.Header

		// 读取响应（支持 gzip）
		var reader io.Reader = resp.Body

		if resp.Header.Get("Content-Encoding") == "gzip" {
			gr, ge := gzip.NewReader(resp.Body)
			if ge != nil {
				buf, ge = wrappedRespHook(code, nil, ge)
				return code, buf, ge
			}
			defer gr.Close()
			reader = gr
		}
		defer resp.Body.Close()

		buf, err = io.ReadAll(reader)

		// 响应 Hook
		buf, err = wrappedRespHook(code, buf, err)
		// if attempt > 0 {
		// core.Info("%d/%d 次重新尝试成功(code=%d)", attempt, maxAttempts, code)
		// }
		return code, buf, err
	}

	// 如果所有尝试都失败
	buf, err = wrappedRespHook(0, nil, lastErr)
	return code, buf, err
}

// ===== 统一解开 *url.Error 等底层错误 =====
func unwrapNetError(err error) error {
	if err == nil {
		return nil
	}
	var ue *url.Error
	if errors.As(err, &ue) {
		return ue.Err // 只返回底层网络错误
	}
	return err
}

// ReqHook：可读取请求体，并直接改写 req 的 Header（甚至可自行替换 req.Body）
type ReqHook func(req *http.Request, body []byte)

// RespHook：可基于状态码和响应体做变换（如 AES 解密），返回的新字节将作为最终返回值
type RespHook func(code int, body []byte, err error) ([]byte, error)

// 计算请求体 SHA256 并写入自定义头
var defaultReqHook ReqHook = func(req *http.Request, body []byte) {}

// 响应解密（可选）
var defaultResHook RespHook = func(code int, b []byte, err error) ([]byte, error) { return b, err }
