package middleware

import (
	"bytes"
	"fmt"
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
	methodWidth    = 7
	pathWidth      = 44
	latencyWidth   = 6
	routeWidth     = 10
	requestIDWidth = 32
)

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
	methodField := fmt.Sprintf("%-*s", methodWidth, method)
	pathField := fmt.Sprintf("%-*s", pathWidth, trimToWidth(path, pathWidth))
	latencyField := fmt.Sprintf("%*s", latencyWidth, fmt.Sprintf("%dms", latencyMS))
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
