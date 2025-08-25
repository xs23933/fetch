package fetch

import (
	"context"
	"net"
	"net/http"

	"golang.org/x/net/proxy"
)

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
