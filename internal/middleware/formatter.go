package middleware

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"

	"github.com/sirupsen/logrus"
)

// ANSI color codes.
const (
	colorReset  = "\033[0m"
	colorGray   = "\033[90m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
)

const (
	statusWidth    = 3
	methodWidth    = 4
	pathWidth      = 80
	latencyWidth   = 6
	routeWidth     = 10
	requestIDWidth = 32
	pathEllipsis   = "..."
	pathSeparator  = " => "
)

var methodAbbreviations = map[string]string{
	http.MethodGet:     "GET",
	http.MethodHead:    "HEAD",
	http.MethodPost:    "POST",
	http.MethodPut:     "PUT",
	http.MethodPatch:   "PCH",
	http.MethodDelete:  "DEL",
	http.MethodConnect: "CONN",
	http.MethodOptions: "OPT",
	http.MethodTrace:   "TRCE",
}

// AccessLogFormatter formats access log entries as:
//
//	2006-01-02 15:04:05  200  POST  /crg/users/sign-in  4ms  crg  b75f5624…
type AccessLogFormatter struct{}

func (f *AccessLogFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	var b bytes.Buffer

	ts := entry.Time.Format("2006-01-02 15:04:05")

	status, _ := entry.Data["status"].(int)
	method, _ := entry.Data["method"].(string)
	path, _ := entry.Data["path"].(string)
	latencyMS, _ := entry.Data["latency_ms"].(int64)
	routeName, _ := entry.Data["route_name"].(string)
	requestID, _ := entry.Data["request_id"].(string)

	statusField := fmt.Sprintf("%*d", statusWidth, status)
	methodField := fmt.Sprintf("%-*s", methodWidth, trimMethodToWidth(method, methodWidth))
	pathField := fmt.Sprintf("%-*s", pathWidth, trimPathToWidth(path, pathWidth))
	latencyDisplay := "-ms"
	if latencyMS != 0 {
		latencyDisplay = fmt.Sprintf("%dms", latencyMS)
	}
	latencyField := fmt.Sprintf("%*s", latencyWidth, latencyDisplay)
	routeField := fmt.Sprintf("%-*s", routeWidth, bracket(routeName))
	requestIDField := trimToWidth(requestID, requestIDWidth)

	statusColor := colorGreen
	switch {
	case status >= 500:
		statusColor = colorRed
	case status >= 400:
		statusColor = colorYellow
	}

	fmt.Fprintf(&b, "%s%s%s  ", colorGray, ts, colorReset)
	fmt.Fprintf(&b, "%s%s%s  ", colorBold+statusColor, statusField, colorReset)
	fmt.Fprintf(&b, "%s%s%s  ", colorCyan, methodField, colorReset)
	fmt.Fprintf(&b, "%s%s%s  ", colorBold, pathField, colorReset)
	fmt.Fprintf(&b, "%s%s%s  ", colorGray, latencyField, colorReset)
	fmt.Fprintf(&b, "%s%-*s%s", colorGray, routeWidth, routeField, colorReset)
	if requestIDField != "" {
		fmt.Fprintf(&b, "  %s%s%s", colorGray, requestIDField, colorReset)
	}
	b.WriteByte('\n')

	return b.Bytes(), nil
}

func bracket(value string) string {
	if value == "" {
		return ""
	}
	return "[" + value + "]"
}

func trimToWidth(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if len(value) <= width {
		return value
	}
	if width == 1 {
		return value[:1]
	}
	trimmed := strings.TrimSpace(value)
	if len(trimmed) <= width {
		return trimmed
	}
	return trimmed[:width-1] + "~"
}

func trimMethodToWidth(method string, width int) string {
	if width <= 0 {
		return ""
	}

	trimmed := strings.ToUpper(strings.TrimSpace(method))
	abbr, ok := methodAbbreviations[trimmed]
	if ok {
		return abbr
	}

	if len(trimmed) <= width {
		return trimmed
	}

	return trimmed[:width]
}

func trimPathToWidth(path string, width int) string {
	if width <= 0 {
		return ""
	}

	trimmed := strings.TrimSpace(path)
	if len(trimmed) <= width {
		return trimmed
	}

	parts := strings.SplitN(trimmed, pathSeparator, 2)
	if len(parts) == 2 {
		prefix := strings.TrimSpace(parts[0]) + pathSeparator
		tail := strings.TrimSpace(parts[1])
		if len(prefix) < width {
			return prefix + trimPathTail(tail, width-len(prefix))
		}
	}

	return trimPathTail(trimmed, width)
}

func trimPathTail(path string, width int) string {
	if width <= 0 {
		return ""
	}

	if len(path) <= width {
		return path
	}

	if width <= len(pathEllipsis) {
		return pathEllipsis[:width]
	}

	tailWidth := width - len(pathEllipsis)
	return pathEllipsis + path[len(path)-tailWidth:]
}
