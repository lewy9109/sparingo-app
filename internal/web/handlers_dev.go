package web

import (
	"net/http"
	"os"
	"strings"

	"sqoush-app/internal/model"
)

func (s *Server) handleDevSwitchUser(w http.ResponseWriter, r *http.Request) {
	if !isDevMode() {
		http.NotFound(w, r)
		return
	}
	currentUser := s.currentUser(r)
	if !isSuperAdmin(currentUser) {
		http.Error(w, "brak uprawnień", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "nieprawidłowe dane", http.StatusBadRequest)
		return
	}
	userID := strings.TrimSpace(r.FormValue("user_id"))
	if userID == "" {
		http.Error(w, "brak użytkownika", http.StatusBadRequest)
		return
	}
	if _, ok := s.store.GetUser(userID); !ok {
		http.Error(w, "nie znaleziono użytkownika", http.StatusNotFound)
		return
	}
	setAuthCookie(w, userID)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func isSuperAdmin(user model.User) bool {
	return user.Role == model.RoleSuperAdmin
}

func isDevMode() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("APP")), "dev")
}
