package web

import (
	"net/http"
	"strings"
)

func isHTMX(r *http.Request) bool {
	return strings.ToLower(r.Header.Get("HX-Request")) == "true"
}
