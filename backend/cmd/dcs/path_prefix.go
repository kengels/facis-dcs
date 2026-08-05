package main

import (
	"net/http"
	"os"
	"strings"

	goahttp "goa.design/goa/v3/http"
)

type prefixedMuxer struct {
	mux    goahttp.Muxer
	prefix string
}

func newPrefixedMuxer(mux goahttp.Muxer, prefix string) goahttp.Muxer {
	if prefix == "" || prefix == "/" {
		return mux
	}
	prefix = strings.TrimSuffix(prefix, "/")
	return &prefixedMuxer{mux: mux, prefix: prefix}
}

func (p *prefixedMuxer) Handle(method, pattern string, handler http.HandlerFunc) {
	prefixedPattern := p.prefix + pattern
	p.mux.Handle(method, prefixedPattern, handler)
}

func (p *prefixedMuxer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.mux.ServeHTTP(w, r)
}

func (p *prefixedMuxer) Vars(r *http.Request) map[string]string {
	return p.mux.Vars(r)
}

func getAPIPathPrefix() string {
	return os.Getenv("DCS_API_PATH")
}
