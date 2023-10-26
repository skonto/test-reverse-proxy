package rep

import (
	"net/http"
	"os"
	"sync"
	"testing"
)

func TestProxyBehindEnvoy(t *testing.T) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConnsPerHost = 10000
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

			var url string
			if url = os.Getenv("INGRESS_URL"); url == "" {
				url = "0.0.0.0:10000"
			}

			requestHost := os.Getenv("REQUEST_HOST")
			// send to envoy
			for i := 0; i < 1000; i++ {
				if err := send(c, url, body, requestHost); err != nil {
					t.Errorf("error during request: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()
}
