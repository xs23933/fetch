package fetch

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/xs23933/core"
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
		DialContext: d.DialContext,
		Dial:        d.Dial,
	}
}

// New New fetch
func New(options ...interface{}) *Fetch {
	jar := NewCookieJar()
	fetch := &Fetch{
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/68.0.3440.84 Safari/537.36",
		Jar:       jar,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
		headers:   make(map[string]string),
		Timeout:   0,
	}

	if options != nil {
		for k, v := range options[0].(map[string]interface{}) {
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
			case "Timeout":
				fetch.Timeout = v.(time.Duration)
			}
		}
	}
	return fetch
}

// Get 获得数据
func (fetch *Fetch) Get(u string, params ...interface{}) (buf []byte, err error) {
	req := new(http.Request)
	addr := new(url.URL)
	addr, err = url.Parse(u)
	if err != nil {
		core.Log(err.Error())
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

	req, err = http.NewRequest("GET", addr.String(), nil)
	buf, err = fetch.do(req)
	return
}

// setHeaders 设置 header
func (fetch *Fetch) setHeaders(headers map[string]string) {
	for k, v := range headers {
		fetch.headers[k] = v
	}
}

// Get 获得数据
func Get(u string, params ...interface{}) ([]byte, error) {
	fetch := New()
	query := make(map[string]string)
	if len(params) > 0 {
		for key, item := range params[0].(map[string]interface{}) {
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

// ProxyGet 配置代理采集
//  u       string                 网址
//  proxy   string                 代理网址 http://127.0.0.1:8080
//  params  map[string]interface{} 这里面包含了 请求的 query数据 或 headers
//    e.g map[string]interface{} {
//	         "params": map[string]string { // 如果有 query 参数就配置 params
//				"key": "value",
//			 },
//           "headers" map[string]string { // 如果有 header 配置 headers
//				"header": "value",
//           },
//        }
func ProxyGet(u, proxy string, params ...interface{}) ([]byte, error) {
	fetch := New(map[string]interface{}{
		"proxy": proxy,
	})
	query := make(map[string]string)
	if len(params) > 0 {
		for key, item := range params[0].(map[string]interface{}) {
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
//  u       string                 网址
//  proxy   string                 代理网址 http://127.0.0.1:8080
//  params  map[string]interface{} 请求json数据
//  headers map[string]string      可配置header在里面
func ProxyPost(u, proxy string, params map[string]string, headers ...interface{}) ([]byte, error) {
	fetch := New(map[string]interface{}{
		"proxy": proxy,
	})
	if len(headers) > 0 {
		fetch.setHeaders(headers[0].(map[string]string))
	}
	return fetch.Post(u, params)
}

// ProxyPayload 代理Post请求
//  u       string                 网址
//  proxy   string                 代理网址 http://127.0.0.1:8080
//  params  map[string]interface{} 请求json数据
//  headers map[string]string      可配置header在里面
func ProxyPayload(u, proxy string, params map[string]interface{}, headers ...interface{}) ([]byte, error) {
	fetch := New(map[string]interface{}{
		"proxy": proxy,
	})
	if len(headers) > 0 {
		fetch.setHeaders(headers[0].(map[string]string))
	}
	return fetch.Payload(u, params)
}

// Post Post 数据
//  u       string                 网址
//  proxy   string                 代理网址 http://127.0.0.1:8080
//  params  map[string]string      请求post数据
//  headers map[string]string      可配置header在里面
func (fetch *Fetch) Post(u string, params map[string]string, headers ...interface{}) (buf []byte, err error) {
	req := new(http.Request)
	addr := new(url.URL)
	addr, err = url.Parse(u)
	if err != nil {
		core.Log(err.Error())
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

	req, err = http.NewRequest("POST", addr.String(), strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	buf, err = fetch.do(req)
	return
}

// Payload payload 请求数据
//  u       string                 网址
//  params  map[string]interface{} 请求json数据
//  headers map[string]string      可配置header在里面
func (fetch *Fetch) Payload(u string, params map[string]interface{}, header ...interface{}) (buf []byte, err error) {
	req := new(http.Request)
	addr := new(url.URL)
	addr, err = url.Parse(u)
	js := make([]byte, 0)
	if err != nil {
		core.Log(err.Error())
		return
	}

	js, err = json.Marshal(params)
	if err != nil {
		core.Log(err.Error())
		return
	}
	param := bytes.NewBuffer(js)
	req, err = http.NewRequest("POST", addr.String(), param)
	buf, err = fetch.do(req)
	return
}

func (fetch *Fetch) do(req *http.Request) (buf []byte, err error) {
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
	}
	resp := new(http.Response)
	resp, err = fetch.client.Do(req)
	if err != nil {
		// core.Log("Request failed %v", err)
		return
	}
	defer resp.Body.Close()
	buf, err = ioutil.ReadAll(resp.Body)
	return
}
