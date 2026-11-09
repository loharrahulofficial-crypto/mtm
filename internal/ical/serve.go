package ical

// HTTP serving for the read-only local endpoint. The handler serves
// exactly one resource — the calendar feed — and nothing else: no file
// access, no directory listing, no other paths.

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
)

// FeedPath is the canonical feed path when no token is set.
const FeedPath = "/mtm.ics"

// FeedHandlerFunc serves the feed produced by feed on each request.
// Only FeedPath (or /<token>/mtm.ics when token is set) is served; every
// other path is 404 and every method other than GET/HEAD is 405.
func FeedHandlerFunc(token string, feed func() ([]byte, error)) http.Handler {
	path := FeedPath
	if token != "" {
		path = "/" + token + FeedPath
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, err := feed()
		if err != nil {
			http.Error(w, "feed unavailable", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Disposition", `inline; filename="mtm.ics"`)
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Length", strconv.Itoa(len(body)))
			return
		}
		_, _ = w.Write(body)
	})
}

// IsLoopback reports whether host is a loopback address (by literal IP or
// the name "localhost"). An empty host is NOT loopback — ":8765" binds
// every interface.
func IsLoopback(host string) bool {
	switch host {
	case "":
		return false
	case "localhost":
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// WarnIfExposed writes a loud warning when bind addresses a non-loopback
// host; it never refuses — the caller prints and continues.
func WarnIfExposed(w io.Writer, bind, token string) {
	host, _, err := net.SplitHostPort(bind)
	if err != nil {
		host = bind // malformed host: surface it in the warning either way
	}
	if IsLoopback(host) {
		return
	}
	fmt.Fprintf(w, "WARNING: binding to %s serves your task calendar beyond this machine —\n", bind)
	if token == "" {
		fmt.Fprintf(w, "         there is no --token set: anyone who can reach this address can read it.\n")
		fmt.Fprintf(w, "         pass --token <secret> to require a secret path, or use loopback (127.0.0.1).\n")
		return
	}
	fmt.Fprintf(w, "         it requires the secret path, but only transport-integrity protects it (no TLS).\n")
	fmt.Fprintf(w, "         binding to loopback (127.0.0.1) is the safe default.\n")
}
