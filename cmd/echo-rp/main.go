package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"time"
)

func main() {
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
		log.Fatalf("Failed to parse echo URL: %v", err)
	}
	proxy := httputil.NewSingleHostReverseProxy(echoURL)

	proxyWithMiddleware := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rc := http.NewResponseController(w)
		_ = rc.EnableFullDuplex()
		proxy.ServeHTTP(w, r)
	})
	proxyServer := httptest.NewServer(proxyWithMiddleware)

	// Uncomment to make it fail
	// proxyServer := httptest.NewServer(proxy)

	log.Printf("Proxy listening to :%s", proxyServer.URL)

	time.Sleep(15 * time.Minute)

	defer proxyServer.Close()

}
