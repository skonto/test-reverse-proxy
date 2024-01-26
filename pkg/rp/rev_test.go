package rep

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"sync"
	"testing"
	"time"

	"knative.dev/serving/pkg/http/handler"
)

const (
	bodySize          = 32 * 1024
	parallelism       = 32
	disableKeepAlives = false
)

func TestProxyEchoOK(t *testing.T) {
	// The server responding with the sent body.
	echoServer := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, req *http.Request) {
			body, err := ioutil.ReadAll(req.Body)
			if err != nil {
				log.Printf("error reading body: %v", err)
				http.Error(w, fmt.Sprintf("error reading body: %v", err), http.StatusInternalServerError)
				return
			}
			if _, err := w.Write(body); err != nil {
				log.Printf("error writing body: %v", err)
			}
		},
	))
	defer echoServer.Close()

	// The server proxying requests to the echo server.
	echoURL, err := url.Parse(echoServer.URL)
	if err != nil {
		t.Fatalf("Failed to parse echo URL: %v", err)
	}
	proxy := httputil.NewSingleHostReverseProxy(echoURL)

	//proxy.FlushInterval = -1 // flush immediately
	proxyWithMiddleware := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rc := http.NewResponseController(w)
		_ = rc.EnableFullDuplex()
		proxy.ServeHTTP(w, r)
	})

	proxyServer := httptest.NewServer(proxyWithMiddleware)

	defer proxyServer.Close()

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DisableKeepAlives = disableKeepAlives
	c := &http.Client{
		Transport: transport,
	}

	body := make([]byte, bodySize)
	for i := 0; i < cap(body); i++ {
		body[i] = 42
	}

	var wg sync.WaitGroup
	wg.Add(parallelism)
	for i := 0; i < parallelism; i++ {
		go func(i int) {
			defer wg.Done()

			for i := 0; i < 10; i++ {
				if err := send(c, proxyServer.URL, body, ""); err != nil {
					t.Errorf("error during request: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()
}

func TestProxyEchoOKWithChainHandler(t *testing.T) {
	// The server responding with the sent body.
	echoServer := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, req *http.Request) {
			body, err := ioutil.ReadAll(req.Body)
			if err != nil {
				log.Printf("error reading body: %v", err)
				http.Error(w, fmt.Sprintf("error reading body: %v", err), http.StatusInternalServerError)
				return
			}

			if _, err := w.Write(body); err != nil {
				log.Printf("error writing body: %v", err)
			}
		},
	))
	defer echoServer.Close()

	// The server proxying requests to the echo server.
	echoURL, err := url.Parse(echoServer.URL)
	if err != nil {
		t.Fatalf("Failed to parse echo URL: %v", err)
	}
	// proxy := httputil.NewSingleHostReverseProxy(echoURL)
	proxy := NewHeaderPruningReverseProxy(echoURL.Host, "", []string{}, false)
	proxy.FlushInterval = 0
	proxyWithMiddleware := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxy.ServeHTTP(w, r)
	})

	composedHandler := handler.NewTimeoutHandler(proxyWithMiddleware, "request timeout", func(r *http.Request) (time.Duration, time.Duration, time.Duration) {
		return time.Minute, time.Minute, time.Minute
	})

	composedHandler2 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rc := http.NewResponseController(w)
		err = rc.EnableFullDuplex()
		if err != nil {
			fmt.Printf("ERROR2:%v\n", err)
		}
		composedHandler.ServeHTTP(w, r)
	})

	proxyServer := httptest.NewServer(composedHandler2)

	defer proxyServer.Close()

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DisableKeepAlives = disableKeepAlives
	c := &http.Client{
		Transport: transport,
	}

	body := make([]byte, bodySize)
	for i := 0; i < cap(body); i++ {
		body[i] = 42
	}

	var wg sync.WaitGroup
	wg.Add(parallelism)
	for i := 0; i < parallelism; i++ {
		go func(i int) {
			defer wg.Done()

			for i := 0; i < 1000; i++ {
				if err := send(c, proxyServer.URL, body, ""); err != nil {
					t.Errorf("error during request: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()
}

func TestProxyEchoFailWithChainHandler(t *testing.T) {
	// The server responding with the sent body.
	echoServer := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, req *http.Request) {
			body, err := ioutil.ReadAll(req.Body)
			if err != nil {
				log.Printf("error reading body: %v", err)
				http.Error(w, fmt.Sprintf("error reading body: %v", err), http.StatusInternalServerError)
				return
			}

			if _, err := w.Write(body); err != nil {
				log.Printf("error writing body: %v", err)
			}
		},
	))
	defer echoServer.Close()

	// The server proxying requests to the echo server.
	echoURL, err := url.Parse(echoServer.URL)
	if err != nil {
		t.Fatalf("Failed to parse echo URL: %v", err)
	}
	// proxy := httputil.NewSingleHostReverseProxy(echoURL)
	proxy := NewHeaderPruningReverseProxy(echoURL.Host, "", []string{}, false)
	proxy.FlushInterval = 0
	proxyWithMiddleware := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxy.ServeHTTP(w, r)
	})

	composedHandler := handler.NewTimeoutHandler(proxyWithMiddleware, "request timeout", func(r *http.Request) (time.Duration, time.Duration, time.Duration) {
		return time.Minute, time.Minute, time.Minute
	})

	proxyServer := httptest.NewServer(composedHandler)

	defer proxyServer.Close()

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DisableKeepAlives = disableKeepAlives
	c := &http.Client{
		Transport: transport,
	}

	body := make([]byte, bodySize)
	for i := 0; i < cap(body); i++ {
		body[i] = 42
	}

	var wg sync.WaitGroup
	wg.Add(parallelism)
	for i := 0; i < parallelism; i++ {
		go func(i int) {
			defer wg.Done()

			for i := 0; i < 1000; i++ {
				if err := send(c, proxyServer.URL, body, ""); err != nil {
					t.Errorf("error during request: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()
}

func TestProxyEchoFail(t *testing.T) {
	// The server responding with the sent body.
	echoServer := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, req *http.Request) {
			body, err := ioutil.ReadAll(req.Body)
			if err != nil {
				log.Printf("error reading body: %v", err)
				http.Error(w, fmt.Sprintf("error reading body: %v", err), http.StatusInternalServerError)
				return
			}

			if _, err := w.Write(body); err != nil {
				log.Printf("error writing body: %v", err)
			}
		},
	))
	defer echoServer.Close()

	// The server proxying requests to the echo server.
	echoURL, err := url.Parse(echoServer.URL)
	if err != nil {
		t.Fatalf("Failed to parse echo URL: %v", err)
	}
	proxy := httputil.NewSingleHostReverseProxy(echoURL)

	//proxy.FlushInterval = -1 // flush immediately

	proxyServer := httptest.NewServer(proxy)

	defer proxyServer.Close()

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DisableKeepAlives = disableKeepAlives
	c := &http.Client{
		Transport: transport,
	}

	body := make([]byte, bodySize)
	for i := 0; i < cap(body); i++ {
		body[i] = 42
	}

	var wg sync.WaitGroup
	wg.Add(parallelism)
	for i := 0; i < parallelism; i++ {
		go func(i int) {
			defer wg.Done()

			for i := 0; i < 1000; i++ {
				if err := send(c, proxyServer.URL, body, ""); err != nil {
					t.Errorf("error during request: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()
}

func send(client *http.Client, url string, body []byte, rHost string) error {
	r := bytes.NewBuffer(body)
	req, err := http.NewRequest("POST", url, r)

	if rHost != "" {
		req.Host = rHost
	}

	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	bd := io.Reader(resp.Body)

	rec, err := ioutil.ReadAll(bd)

	if err != nil {
		return fmt.Errorf("failed to read body: %w", err)
	}

	if _, err = io.Copy(io.Discard, resp.Body); err != nil {
		return fmt.Errorf("failed to discard body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	if len(rec) != len(body) {
		return fmt.Errorf("unexpected body length: %d", len(rec))
	}

	return nil
}

func NewHeaderPruningReverseProxy(target, hostOverride string, headersToRemove []string, useHTTPS bool) *httputil.ReverseProxy {
	UserAgentKey := "User-Agent"
	return &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			if useHTTPS {
				req.URL.Scheme = "https"
			} else {
				req.URL.Scheme = "http"
			}
			req.URL.Host = target

			if hostOverride != "" {
				req.Host = hostOverride
				req.Header.Add("K-Passthrough-Lb", "true")
			}

			// Copied from httputil.NewSingleHostReverseProxy.
			if _, ok := req.Header[UserAgentKey]; !ok {
				// explicitly disable User-Agent so it's not set to default value
				req.Header.Set(UserAgentKey, "")
			}

			for _, h := range headersToRemove {
				req.Header.Del(h)
			}
		},
	}
}
