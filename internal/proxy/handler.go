package proxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"api-gateway/internal/app"
	"api-gateway/internal/domain"

	"github.com/gin-gonic/gin"
)

type contextKey string

const routeKey contextKey = "proxy_route"

type routeInfo struct {
	upstream   *url.URL
	pathPrefix string
	name       string
	trimPath   bool
}

// Handler forwards matched requests to upstream services via httputil.ReverseProxy.
// WebSocket upgrade requests are tunnelled over a raw TCP connection.
type Handler struct {
	resolver *app.Resolver
	proxy    *httputil.ReverseProxy
}

func NewHandler(resolver *app.Resolver, timeout time.Duration) *Handler {
	h := &Handler{resolver: resolver}
	h.proxy = &httputil.ReverseProxy{
		Director:      h.director,
		Transport:     newTransport(timeout),
		FlushInterval: -1,
		ErrorHandler:  h.errorHandler,
	}
	return h
}

// newTransport returns an http.Transport tuned for a gateway proxy workload.
func newTransport(timeout time.Duration) http.RoundTripper {
	return &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   timeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          200,
		MaxIdleConnsPerHost:   50,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

func (h *Handler) ServeHTTP(c *gin.Context) {
	route, ok := h.resolver.Match(c.Request.Method, c.Request.URL.Path)
	if !ok {
		domain.WriteError(c, http.StatusNotFound, "ROUTE_NOT_FOUND", "No upstream route matched")
		return
	}

	info := routeInfo{
		upstream:   route.Upstream,
		pathPrefix: route.PathPrefix,
		name:       route.Name,
		trimPath:   route.TrimPath,
	}

	// Store route metadata for access logging.
	c.Set("route_name", route.Name)
	c.Set("upstream", route.Upstream.String())
	trimmedPath := c.Request.URL.Path
	if route.TrimPath {
		trimmedPath = stripPrefix(c.Request.URL.Path, route.PathPrefix)
	}
	c.Set("trimmed_path", trimmedPath)

	if isWebSocketUpgrade(c.Request) {
		c.Set("is_websocket", true)
		h.serveWebSocket(c, info)
		return
	}

	// Store route info in request context so Director can access it.
	c.Request = c.Request.WithContext(
		context.WithValue(c.Request.Context(), routeKey, info),
	)

	h.proxy.ServeHTTP(c.Writer, c.Request)
}

// director rewrites the outbound request URL to the matched upstream target.
// httputil.ReverseProxy calls this before forwarding the request.
func (h *Handler) director(req *http.Request) {
	info, ok := req.Context().Value(routeKey).(routeInfo)
	if !ok {
		return
	}
	target := info.upstream
	req.URL.Scheme = target.Scheme
	req.URL.Host = target.Host
	req.URL.Path = upstreamRequestPath(req.URL.Path, info)
	// Preserve the Host header as the upstream host.
	req.Host = target.Host
	// X-Forwarded-For is appended automatically by httputil.ReverseProxy
	// using req.RemoteAddr after Director returns; no manual handling needed.
}

// errorHandler translates upstream transport errors into gateway error responses
// using the standard domain error contract.
func (h *Handler) errorHandler(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusBadGateway
	code := "UPSTREAM_UNAVAILABLE"
	message := "Upstream request failed"

	if isTimeoutError(err) {
		status = http.StatusGatewayTimeout
		code = "UPSTREAM_TIMEOUT"
		message = "Upstream request timed out"
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(domain.ErrorPayload{
		Error:     domain.ErrorDetails{Code: code, Message: message},
		RequestID: r.Header.Get("X-Request-ID"),
	})
}

// isWebSocketUpgrade reports whether the request is a WebSocket upgrade.
func isWebSocketUpgrade(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket") &&
		strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade")
}

// serveWebSocket proxies a WebSocket connection by hijacking the client TCP
// connection and opening a raw TCP connection to the upstream, then tunnelling
// bidirectionally. No timeout is applied — the connection lives until either
// side closes it.
func (h *Handler) serveWebSocket(c *gin.Context, info routeInfo) {
	targetPath := upstreamRequestPath(c.Request.URL.Path, info)
	upAddr := info.upstream.Host

	// Dial upstream (TLS when scheme is https).
	var upConn net.Conn
	var err error
	if info.upstream.Scheme == "https" {
		upConn, err = tls.DialWithDialer(
			&net.Dialer{Timeout: 10 * time.Second},
			"tcp", upAddr, nil,
		)
	} else {
		upConn, err = net.DialTimeout("tcp", upAddr, 10*time.Second)
	}
	if err != nil {
		domain.WriteError(c, http.StatusBadGateway, "UPSTREAM_UNAVAILABLE", "Cannot connect to upstream")
		return
	}
	defer func() { _ = upConn.Close() }()

	// Clone the request and rewrite it for the upstream.
	upReq := c.Request.Clone(context.Background())
	upReq.URL.Scheme = info.upstream.Scheme
	upReq.URL.Host = upAddr
	upReq.URL.Path = targetPath
	upReq.Host = upAddr
	// Remove hop-by-hop headers that must not be forwarded.
	upReq.Header.Del("Te")
	upReq.Header.Del("Trailers")

	if err = upReq.Write(upConn); err != nil {
		domain.WriteError(c, http.StatusBadGateway, "UPSTREAM_UNAVAILABLE", "Failed to forward upgrade request")
		return
	}

	// Read the upstream HTTP response (expecting 101 Switching Protocols).
	upBufReader := bufio.NewReader(upConn)
	upResp, err := http.ReadResponse(upBufReader, upReq)
	if err != nil {
		domain.WriteError(c, http.StatusBadGateway, "UPSTREAM_UNAVAILABLE", "Failed to read upstream upgrade response")
		return
	}
	defer func() { _ = upResp.Body.Close() }()

	if upResp.StatusCode != http.StatusSwitchingProtocols {
		// Upstream rejected the upgrade; forward the status code and bail.
		c.Set("access_status", upResp.StatusCode)
		for k, vv := range upResp.Header {
			for _, v := range vv {
				c.Header(k, v)
			}
		}
		c.Status(upResp.StatusCode)
		return
	}

	// Hijack the client connection for raw access.
	hijacker, ok := c.Writer.(http.Hijacker)
	if !ok {
		domain.WriteError(c, http.StatusInternalServerError, "WS_HIJACK_FAILED", "WebSocket hijacking not supported")
		return
	}
	clientConn, clientBuf, err := hijacker.Hijack()
	if err != nil {
		return
	}
	defer func() { _ = clientConn.Close() }()

	// Forward the 101 response to the client.
	if err = upResp.Write(clientBuf); err != nil {
		return
	}
	if err = clientBuf.Flush(); err != nil {
		return
	}
	c.Set("access_status", http.StatusSwitchingProtocols)

	// Bidirectional tunnel. Block until upstream closes; the goroutine
	// will naturally exit when clientConn is closed by defer above.
	go func() { _, _ = io.Copy(upConn, clientBuf) }()
	_, _ = io.Copy(clientConn, upBufReader)
}

func upstreamRequestPath(incomingPath string, info routeInfo) string {
	path := incomingPath
	if info.trimPath {
		path = stripPrefix(path, info.pathPrefix)
	}
	return singleJoiningSlash(info.upstream.Path, path)
}

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "context deadline exceeded")
}

func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		if a == "" {
			return "/" + b
		}
		return a + "/" + b
	default:
		return a + b
	}
}

// stripPrefix removes the matched route prefix from the request path.
// The remaining path always starts with "/".
// Example: path="/users/123", prefix="/users" -> "/123"
//
//	path="/users", prefix="/users" -> "/"
func stripPrefix(path, prefix string) string {
	stripped := strings.TrimPrefix(path, prefix)
	if stripped == "" || stripped[0] != '/' {
		stripped = "/" + stripped
	}
	return stripped
}
