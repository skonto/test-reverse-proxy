package main

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"golang.org/x/net/http2"
	"net"
	"net/http"
)

func main() {
	client := http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
				return net.Dial(network, addr)
			},
		},
	}

	res, err := client.Post("http://localhost:8080", "text/plain", bytes.NewReader([]byte{}))
	fmt.Printf("Res: %#v\nErr: %#v\n", res, err)
}
