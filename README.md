# Test fix for reverse proxy

For more see: https://github.com/golang/go/issues/40747 and https://github.com/knative/serving/issues/12387.
Make sure you use go 1.21 and it contains the fix.

Scenario: **Http Client -> Envoy (10000) -> Http Reverse Proxy -> Test Echo Server (random port)**

1. On one terminal run the app

```
$ go run ./cmd/echo-rp/
2023/10/19 18:36:54 Proxy listening to :http://127.0.0.1:34423
```

2. Patch config.yaml with the port output: `port_value: 34423`.

3. On another terminal run envoy
```
$docker run --rm -it --net=host \
-v $(pwd)/config.yaml:/config.yaml \
-p 9901:9901 \
-p 10000:10000 \
envoyproxy/envoy:v1.28-latest \
-c /config.yaml
```

4. Test end-to-end connectivity (envoy is listening to 10000 by default):
```
curl -X POST http://0.0.0.0:10000  -d "data"
data
```

5. Run Tests:

```
$ go test -run TestProxyBehindEnvoy ./pkg/rp/...
ok  	github.com/skonto/test-reverse-proxy/pkg/rp	12.262s
```

If you uncomment code in the test that uses no fullduplex then it should fail even with go 1.21:
```
$ go test -run TestProxyBehindEnvoy ./pkg/rp/...
--- FAIL: TestProxyBehindEnvoy (12.42s)
    envoy_test.go:30: error during request: failed to read body: unexpected EOF
    envoy_test.go:30: error during request: failed to read body: unexpected EOF
    envoy_test.go:30: error during request: failed to read body: unexpected EOF
    envoy_test.go:30: error during request: failed to read body: unexpected EOF
    envoy_test.go:30: error during request: failed to read body: unexpected EOF
    envoy_test.go:30: error during request: failed to read body: unexpected EOF
    envoy_test.go:30: error during request: failed to read body: unexpected EOF
    envoy_test.go:30: error during request: failed to read body: unexpected EOF
    envoy_test.go:30: error during request: failed to read body: unexpected EOF
    envoy_test.go:30: error during request: failed to read body: unexpected EOF
    envoy_test.go:30: error during request: failed to read body: unexpected EOF
    envoy_test.go:30: error during request: failed to read body: unexpected EOF
    envoy_test.go:30: error during request: failed to read body: unexpected EOF
    envoy_test.go:30: error during request: failed to read body: unexpected EOF
    envoy_test.go:30: error during request: failed to read body: unexpected EOF
    envoy_test.go:30: error during request: failed to read body: unexpected EOF
    envoy_test.go:30: error during request: failed to read body: unexpected EOF
    envoy_test.go:30: error during request: failed to read body: unexpected EOF
    envoy_test.go:30: error during request: failed to read body: unexpected EOF
    envoy_test.go:30: error during request: failed to read body: unexpected EOF
    envoy_test.go:30: error during request: failed to read body: unexpected EOF
    envoy_test.go:30: error during request: failed to read body: unexpected EOF
    envoy_test.go:30: error during request: failed to read body: unexpected EOF
FAIL
FAIL	github.com/skonto/test-reverse-proxy/pkg/rp	12.428s
FAIL

```

