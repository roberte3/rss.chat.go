package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// RespondJSON sends a 200 OK response with JSON data.
func RespondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(data)
}

// RespondError sends a 503 Service Unavailable response with a plain-text error message
// in the format "Can't X because Y."
func RespondError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusServiceUnavailable)
	fmt.Fprint(w, message)
	if message[len(message)-1] != '.' {
		fmt.Fprint(w, ".")
	}
}

// denyCaching marks a response as never storable.
//
// The client sends credentials in the request URL rather than a body (see the
// security notes in README.md), so the URL of an authenticated request is
// itself a secret. no-store keeps browsers and any intermediary cache from
// writing that URL, and the response it produced, to disk.
//
// Must be called before the first write, since headers are flushed with the
// status line.
func denyCaching(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
}

// RespondTooManyRequests sends a 429 with the same plain-text error shape as
// RespondError, plus Retry-After. A distinct status from RespondError's 503 so
// callers can tell "slow down" from "something went wrong"; the client surfaces
// the message either way.
func RespondTooManyRequests(w http.ResponseWriter, retryAfter time.Duration, message string) {
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
	w.WriteHeader(http.StatusTooManyRequests)
	fmt.Fprint(w, message)
	if message[len(message)-1] != '.' {
		fmt.Fprint(w, ".")
	}
}

// RespondText sends a 200 OK response with plain-text data.
func RespondText(w http.ResponseWriter, text string) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, text)
}

// RespondXML sends a 200 OK response with XML data (RSS/OPML).
func RespondXML(w http.ResponseWriter, xml string) {
	w.Header().Set("Content-Type", "application/rss+xml")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, xml)
}

// RespondJSONString sends a 200 OK response with a JSON-encoded string.
// Used when the endpoint returns a string (like RSS feed or OPML).
func RespondJSONString(w http.ResponseWriter, s string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(s)
}

// RespondRedirect sends a redirect response with the given status code.
// Typically 302 or 303.
func RespondRedirect(w http.ResponseWriter, location string, statusCode int) {
	w.Header().Set("Location", location)
	w.WriteHeader(statusCode)
}

// addWebsubHeader adds WebSub Link header to a feed response if WebSub is enabled.
// Must be called before writing the response body.
func addWebsubHeader(w http.ResponseWriter, hubURL string, feedURL string) {
	if hubURL == "" || feedURL == "" {
		return
	}
	link := fmt.Sprintf("<%s>; rel=\"hub\", <%s>; rel=\"self\"", hubURL, feedURL)
	w.Header().Set("Link", link)
}
