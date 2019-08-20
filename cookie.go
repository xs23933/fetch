package fetch

import (
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sync"
)

/*
Implements the normal http cookie jar interface but also usefully
allows you to dump all the stored cookies without
having to know any of the domains involved, which helps a lot
*/
func NewCookieJar() http.CookieJar {
	realJar, _ := cookiejar.New(nil)

	e := &CookieJar{
		jar:        realJar,
		allCookies: make(map[url.URL][]*http.Cookie),
	}

	return e

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

	copied := make(map[url.URL][]*http.Cookie)
	for u, c := range jar.allCookies {
		copied[u] = c
	}

	return copied
}

type CookieJar struct {
	jar        *cookiejar.Jar
	allCookies map[url.URL][]*http.Cookie
	sync.RWMutex
}
