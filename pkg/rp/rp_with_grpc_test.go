package rep

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/skonto/test-reverse-proxy/pkg/grpc/pb"
	"k8s.io/apimachinery/pkg/util/wait"
)

type gServer struct {
	pb.GreetingServiceServer
}

func (s *gServer) Greeting(ctx context.Context, req *pb.GreetingServiceRequest) (*pb.GreetingServiceReply, error) {
	return &pb.GreetingServiceReply{
		Message: fmt.Sprintf("Hello, %s", req.Name),
	}, nil
}

func TestReverseProxyWithGrpc(t *testing.T) {

	//go func() {
	//
	//	listener, err := net.Listen("tcp", ":8080")
	//	if err != nil {
	//		panic(err)
	//	}
	//
	//	s := grpc.NewServer()
	//	log.Printf("Starting grpc server %s at %s ", "grpc", ":8080")
	//	pb.RegisterGreetingServiceServer(s, &gServer{})
	//	reflection.Register(s)
	//
	//	if err := s.Serve(listener); err != nil {
	//		log.Fatalf("failed to serve: %v", err)
	//	}
	//}()

	transport := newH2CTransport(true)
	sUrl := "http://0.0.0.0:50051"
	u, err := url.Parse(sUrl)
	if err != nil {
		panic(err)
	}
	httpProxy := httputil.NewSingleHostReverseProxy(u)
	httpProxy.Transport = transport
	httpProxy.ErrorHandler = ErrorHandler()
	proxyH := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Test") != "" {
			r.Header.Set("Content-type", "text/plain")
		}

		httpProxy.ServeHTTP(w, r)
	})

	server := &http.Server{
		Addr:              ":9999",
		Handler:           h2c.NewHandler(proxyH, &http2.Server{}),
		ReadHeaderTimeout: time.Minute,
	}

	//go func() {
	log.Printf("Starting http server %s at %s ", "rv", server.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
	//}()

}

func ErrorHandler() func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, req *http.Request, err error) {

		ss := readSockStat()
		log.Printf("error reverse proxying request; sockstat: %q, %v - %v", ss, err, req)
		http.Error(w, err.Error(), http.StatusBadGateway)
	}
}

func readSockStat() string {
	b, err := os.ReadFile("/proc/net/sockstat")
	if err != nil {
		log.Printf("Unable to read sockstat: %v", err)
		return ""
	}
	return string(b)
}

func newH2CTransport(disableCompression bool) http.RoundTripper {
	return &http2.Transport{
		AllowHTTP:          true,
		DisableCompression: disableCompression,
		DialTLS: func(netw, addr string, _ *tls.Config) (net.Conn, error) {
			return DialWithBackOff(context.Background(),
				netw, addr)
		},
	}
}

var backOffTemplate = wait.Backoff{
	Duration: 50 * time.Millisecond,
	Factor:   1.4,
	Jitter:   0.1, // At most 10% jitter.
	Steps:    15,
}

const sleep = 30 * time.Millisecond

var ErrTimeoutDialing = errors.New("timed out dialing")
var DialWithBackOff = NewBackoffDialer(backOffTemplate)

// NewBackoffDialer returns a dialer that executes `net.Dialer.DialContext()` with
// exponentially increasing dial timeouts. In addition it sleeps with random jitter
// between tries.
func NewBackoffDialer(backoffConfig wait.Backoff) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		return dialBackOffHelper(ctx, network, address, backoffConfig, nil)
	}
}

func dialBackOffHelper(ctx context.Context, network, address string, bo wait.Backoff, tlsConf *tls.Config) (net.Conn, error) {
	dialer := &net.Dialer{
		Timeout:   bo.Duration, // Initial duration.
		KeepAlive: 5 * time.Second,
		DualStack: true,
	}
	start := time.Now()
	for {
		var (
			c   net.Conn
			err error
		)
		if tlsConf == nil {
			c, err = dialer.DialContext(ctx, network, address)
		} else {
			c, err = tls.DialWithDialer(dialer, network, address, tlsConf)
		}
		if err != nil {
			var errNet net.Error
			if errors.As(err, &errNet) && errNet.Timeout() {
				if bo.Steps < 1 {
					break
				}
				dialer.Timeout = bo.Step()
				time.Sleep(wait.Jitter(sleep, 1.0)) // Sleep with jitter.
				continue
			}
			return nil, err
		}
		return c, nil
	}
	elapsed := time.Since(start)
	return nil, fmt.Errorf("%w %s after %.2fs", ErrTimeoutDialing, address, elapsed.Seconds())
}
