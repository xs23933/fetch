package fetch

import (
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sync"
)

// ===== CookieJar =====
type CookieJar struct {
	jar        *cookiejar.Jar
	allCookies map[url.URL][]*http.Cookie
	sync.RWMutex
}

func NewCookieJar() http.CookieJar {
	realJar, _ := cookiejar.New(nil)
	return &CookieJar{
		jar:        realJar,
		allCookies: make(map[url.URL][]*http.Cookie),
	}
}

func (jar *CookieJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	jar.Lock()
	defer jar.Unlock()
	jar.allCookies[*u] = cookies
	jar.jar.SetCookies(u, cookies)
}

func (jar *CookieJar) Cookies(u *url.URL) []*http.Cookie {
	return jar.jar.Cookies(u)
}

func (jar *CookieJar) ExportAllCookies() map[url.URL][]*http.Cookie {
	jar.RLock()
	defer jar.RUnlock()

	copied := make(map[url.URL][]*http.Cookie, len(jar.allCookies))
	for u, c := range jar.allCookies {
		tmp := make([]*http.Cookie, len(c))
		copy(tmp, c)
		copied[u] = tmp
	}
	return copied
}
