package fetch

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"golang.org/x/net/proxy"
)

// Fetch http client
type Fetch struct {
	Proxy     string
	UserAgent string
	Jar       http.CookieJar
	Transport *http.Transport
	client    *http.Client
	headers   map[string]string
	Timeout   time.Duration
	user      string
	password  string
}

type dialer struct {
	addr   string
	socks5 proxy.Dialer
}

func (d *dialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return d.Dial(network, addr)
}

func (d *dialer) Dial(network, addr string) (net.Conn, error) {
	var err error
	if d.socks5 == nil {
		d.socks5, err = proxy.SOCKS5("tcp", d.addr, nil, proxy.Direct)
		if err != nil {
			return nil, err
		}
	}
	return d.socks5.Dial(network, addr)
}

func Socks5Proxy(addr string) *http.Transport {
	d := &dialer{addr: addr}
	return &http.Transport{
		DialContext:       d.DialContext,
		Dial:              d.Dial,
		DisableKeepAlives: true,
	}
}

// New New fetch
func New(options ...any) *Fetch {
	jar := NewCookieJar()
	fetch := &Fetch{
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/68.0.3440.84 Safari/537.36",
		Jar:       jar,
		Transport: &http.Transport{
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
			DisableKeepAlives: true,
		},
		headers: make(map[string]string),
		Timeout: 0,
	}

	if options != nil {
		for k, v := range options[0].(map[string]any) {
			switch k {
			case "userAgent": // 配置UA
				fetch.UserAgent = v.(string)
			case "proxy": // 配置代理
				proxy, err := url.Parse(v.(string))
				if err == nil {
					if proxy.Scheme == "http" {
						fetch.Transport.Proxy = http.ProxyURL(proxy)
					} else {
						fetch.Transport = Socks5Proxy(proxy.Host)
					}
				}
			case "headers":
				fetch.setHeaders(v.(map[string]string))
			case "Timeout", "timeout":
				fetch.Timeout = v.(time.Duration)
			}
		}
	}
	return fetch
}

// SetProxy 设置代理.
func (fetch *Fetch) SetProxy(proxy string) error {
	proxy = strings.ToLower(proxy)
	px, err := url.Parse(proxy)

	if err == nil {
		if px.Scheme == "http" {
			fetch.Transport.Proxy = http.ProxyURL(px)
		} else {
			fetch.Transport = Socks5Proxy(px.Host)
		}
	}

	return nil
}

// Get 获得数据
func (fetch *Fetch) Get(u string, params ...any) (code int, buf []byte, err error) {
	addr, err := url.Parse(u)
	if err != nil {
		return
	}

	if params != nil {
		q := addr.Query()
		for k, v := range params[0].(map[string]string) {
			q.Set(k, v)
		}
		addr.RawQuery = q.Encode()
	}

	if len(params) > 1 {
		fetch.setHeaders(params[1].(map[string]string))
	}

	req, _ := http.NewRequest("GET", addr.String(), nil)
	code, buf, err = fetch.do(req)
	return
}

// setHeaders 设置 header
func (fetch *Fetch) setHeaders(headers map[string]string) {
	for k, v := range headers {
		fetch.headers[k] = v
	}
}

// SetHeaders 设置头信息
func (fetch *Fetch) SetHeaders(headers map[string]string) {
	fetch.setHeaders(headers)
}

// Get 获得数据
func Get(u string, params ...any) (int, []byte, error) {
	fetch := New()
	query := make(map[string]string)
	if len(params) > 0 {
		for key, item := range params[0].(map[string]any) {
			switch key {
			case "Timeout", "timeout":
				fetch.Timeout = item.(time.Duration)
			case "headers":
				fetch.setHeaders(item.(map[string]string))
			case "params":
				query = item.(map[string]string)
			}
		}
	}
	return fetch.Get(u, query)
}

// ProxyGet 配置代理采集
//
//	 u       string                 网址
//	 proxy   string                 代理网址 http://127.0.0.1:8080
//	 params  map[string]any 这里面包含了 请求的 query数据 或 headers
//	   e.g map[string]any {
//		         "params": map[string]string { // 如果有 query 参数就配置 params
//					"key": "value",
//				 },
//	          "headers" map[string]string { // 如果有 header 配置 headers
//					"header": "value",
//	          },
//	       }
func ProxyGet(u, proxy string, params ...any) (int, []byte, error) {
	fetch := New(map[string]any{
		"proxy": proxy,
	})
	query := make(map[string]string)
	if len(params) > 0 {
		for key, item := range params[0].(map[string]any) {
			switch key {
			case "headers":
				fetch.setHeaders(item.(map[string]string))
			case "params":
				query = item.(map[string]string)
			}
		}
	}
	return fetch.Get(u, query)
}

// ProxyPost 代理post
//
//	u       string                 网址
//	proxy   string                 代理网址 http://127.0.0.1:8080
//	params  map[string]any 请求json数据
//	headers map[string]string      可配置header在里面
func ProxyPost(u, proxy string, params map[string]string, headers ...any) (int, []byte, error) {
	fetch := New(map[string]any{
		"proxy": proxy,
	})
	if len(headers) > 0 {
		fetch.setHeaders(headers[0].(map[string]string))
	}
	return fetch.Post(u, params)
}

// ProxyPayload 代理Post请求
//
//	u       string                 网址
//	proxy   string                 代理网址 http://127.0.0.1:8080
//	params  map[string]any 请求json数据
//	headers map[string]string      可配置header在里面
func ProxyPayload(u, proxy string, params any, headers ...any) (int, []byte, error) {
	fetch := New(map[string]any{
		"proxy": proxy,
	})
	if len(headers) > 0 {
		fetch.setHeaders(headers[0].(map[string]string))
	}
	return fetch.Payload(u, params)
}

// Post 代理post
//
//	u       string                 网址
//	params  map[string]any 请求json数据
//	headers map[string]string      可配置header在里面
func Post(u string, params map[string]string, headers ...any) (int, []byte, error) {
	fetch := New(map[string]any{})
	if len(headers) > 0 {
		fetch.setHeaders(headers[0].(map[string]string))
	}
	return fetch.Post(u, params)
}

// Payload 代理Post请求
//
//	u       string                 网址
//	params  map[string]any 请求json数据
//	headers map[string]string      可配置header在里面
func Payload(u string, params any, headers ...any) (int, []byte, error) {
	fetch := New()
	if len(headers) > 0 {
		fetch.setHeaders(headers[0].(map[string]string))
	}
	return fetch.Payload(u, params)
}

// Post Post 数据
//
//	u       string                 网址
//	params  map[string]string      请求post数据
//	headers map[string]string      可配置header在里面
func (fetch *Fetch) Post(u string, params map[string]string, headers ...any) (code int, buf []byte, err error) {
	addr, err := url.Parse(u)
	if err != nil {
		log.Println(err.Error())
		return
	}

	form := url.Values{}
	for k, v := range params {
		form.Add(k, v)
	}

	// 设置头信息
	if headers != nil {
		fetch.setHeaders(headers[0].(map[string]string))
	}

	req, _ := http.NewRequest("POST", addr.String(), strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	code, buf, err = fetch.do(req)
	return
}

var paramPool = sync.Pool{
	New: func() any {
		return &bytes.Buffer{}
	},
}

// Payload payload 请求数据
//
//	u       string                 网址
//	params  map[string]any 请求json数据
//	headers map[string]string      可配置header在里面
func (fetch *Fetch) Payload(u string, params any, args ...any) (code int, buf []byte, err error) {
	addr, err := url.Parse(u)
	if err != nil {
		log.Println(err.Error())
		return
	}

	hook := func(buf []byte) {}

	// 设置头信息
	for _, arg := range args {
		switch v := arg.(type) {
		case map[string]string:
			fetch.setHeaders(v)
		case func([]byte):
			hook = v
		}
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
			buf, err := sonic.Marshal(v)
			if err == nil {
				param.Write(buf)
			}
		}
	}
	hook(param.Bytes())
	req, _ := http.NewRequest("POST", addr.String(), param)
	req.Header.Add("Content-Type", "application/json")
	code, buf, err = fetch.do(req)
	return
}

// BasicAuth basic auth
func (fetch *Fetch) BasicAuth(us, pw string) {
	fetch.user = us
	fetch.password = pw
}

func (fetch *Fetch) do(req *http.Request) (code int, buf []byte, err error) {
	req.Header.Set("User-Agent", fetch.UserAgent)
	req.Header.Set("Accept-Language", "en")
	for k, v := range fetch.headers { // 设置传入的 head
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
	if len(fetch.user) > 0 && len(fetch.password) > 0 {
		req.SetBasicAuth(fetch.user, fetch.password)
		fetch.user = ""
		fetch.password = ""
	}
	resp, err := fetch.client.Do(req)
	if resp == nil {
		return 500, nil, errors.New("network err or server invalid")
	}
	code = resp.StatusCode
	if err != nil {
		// log.Println("Request failed %v", err)
		return
	}
	defer resp.Body.Close()
	resp.Close = true

	if resp.Header.Get("Content-Encoding") == "gzip" {
		resp.Body, err = gzip.NewReader(resp.Body)
		if err != nil {
			return
		}
	}

	buf, err = io.ReadAll(resp.Body)
	return
}
