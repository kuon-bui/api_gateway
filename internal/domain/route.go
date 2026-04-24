package domain

import "net/url"

type Route struct {
	Name       string
	Methods    map[string]struct{}
	PathPrefix string
	Upstream   *url.URL
}
