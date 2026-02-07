package web

import (
	"net/http"
	"strings"

	"sqoush-app/internal/store"
)

const userCookieName = "sqoush_user_id"

func WithCurrentUser(store store.Store, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		if cookie, err := r.Cookie(userCookieName); err == nil {
			if _, ok := store.GetUser(cookie.Value); ok {
				next.ServeHTTP(w, r)
				return
			}
		}
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	})
}

func isPublicPath(path string) bool {
	if path == "/login" || path == "/register" || path == "/healthz" || path == "/logout" {
		return true
	}
	return strings.HasPrefix(path, "/static/")
}
