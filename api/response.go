package api

import (
	"encoding/json"
	"fmt"
	"net/http"
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
