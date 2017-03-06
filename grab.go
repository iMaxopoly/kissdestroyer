package main

import (
	"bufio"
	"errors"
	"github.com/cardigann/go-cloudflare-scraper"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

type grab struct {
	rotating_proxies []string
	get_proxy_mutex  sync.Mutex
}

func (g *grab) load_proxies(filename string) {
	file, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		g.rotating_proxies = append(g.rotating_proxies, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
}

func (g *grab) get_proxy(desired_pos int) (proxy string, proxy_pos int) {
	g.get_proxy_mutex.Lock()
	defer g.get_proxy_mutex.Unlock()

	if desired_pos >= len(g.rotating_proxies) {
		proxy_pos = 0
		proxy = g.rotating_proxies[proxy_pos]
		return proxy, proxy_pos
	}
	proxy_pos = desired_pos
	proxy = g.rotating_proxies[proxy_pos]
	return proxy, proxy_pos
}

func (g *grab) by_the_proxy(proxy_string, link string) (body_string string, err error) {
	proxy, err := url.Parse(proxy_string)
	if err != nil {
		log.Fatal(err)
	}
	var custom_transport http.RoundTripper = &http.Transport{
		Proxy: http.ProxyURL(proxy),
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	custom_client := http.Client{Transport: scraper.NewTransport(custom_transport)}

	res, err := custom_client.Get(link)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	body_string = strings.TrimSpace(string(body))

	if strings.Contains(body_string, "Access Denied") {
		return "", errors.New("Overriding Access Denied")
	}
	if !strings.Contains(body_string, `<h1 style="background: transparent url(/Content/images/logo.png?id=2) no-repeat;">`) {
		return "", errors.New("Overriding Corrupt HTML")
	}
	return body_string, nil
}
