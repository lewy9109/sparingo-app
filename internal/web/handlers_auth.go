package web

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"sqoush-app/internal/model"

	"golang.org/x/crypto/bcrypt"
)

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	view := AuthView{
		BaseView: BaseView{
			Title: "Logowanie",
			IsDev: isDevMode(),
		},
	}
	if err := s.templates.Render(w, "login.html", view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleLoginPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "nieprawidłowe dane", http.StatusBadRequest)
		return
	}
	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")
	user, ok := s.store.GetUserByEmail(email)
	if !ok || !checkPassword(user.PasswordHash, password) {
		view := AuthView{
			BaseView: BaseView{Title: "Logowanie", IsDev: isDevMode()},
			Error:    "Nieprawidłowy email lub hasło",
		}
		if err := s.templates.Render(w, "login.html", view); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	setAuthCookie(w, user.ID)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	cfg := recaptchaConfigFromEnv()
	view := AuthView{
		BaseView: BaseView{
			Title: "Rejestracja",
			IsDev: isDevMode(),
		},
		RecaptchaSiteKey: cfg.SiteKey,
	}
	if err := s.templates.Render(w, "register.html", view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleRegisterPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "nieprawidłowe dane", http.StatusBadRequest)
		return
	}
	cfg := recaptchaConfigFromEnv()
	firstName := strings.TrimSpace(r.FormValue("first_name"))
	lastName := strings.TrimSpace(r.FormValue("last_name"))
	email := strings.TrimSpace(r.FormValue("email"))
	phone := strings.TrimSpace(r.FormValue("phone"))
	password := r.FormValue("password")
	confirmPassword := r.FormValue("password_confirm")
	if firstName == "" || lastName == "" || email == "" || password == "" {
		view := AuthView{
			BaseView:         BaseView{Title: "Rejestracja", IsDev: isDevMode()},
			Error:            "Wypełnij wszystkie pola",
			RecaptchaSiteKey: cfg.SiteKey,
		}
		if err := s.templates.Render(w, "register.html", view); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	if len(password) < 8 {
		view := AuthView{
			BaseView:         BaseView{Title: "Rejestracja", IsDev: isDevMode()},
			Error:            "Hasło musi mieć min. 8 znaków",
			RecaptchaSiteKey: cfg.SiteKey,
		}
		if err := s.templates.Render(w, "register.html", view); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	if !containsUppercase(password) {
		view := AuthView{
			BaseView:         BaseView{Title: "Rejestracja", IsDev: isDevMode()},
			Error:            "Hasło musi zawierać jedną dużą literę",
			RecaptchaSiteKey: cfg.SiteKey,
		}
		if err := s.templates.Render(w, "register.html", view); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	if password != confirmPassword {
		view := AuthView{
			BaseView:         BaseView{Title: "Rejestracja", IsDev: isDevMode()},
			Error:            "Hasła nie są takie same",
			RecaptchaSiteKey: cfg.SiteKey,
		}
		if err := s.templates.Render(w, "register.html", view); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	if cfg.Enabled {
		token := strings.TrimSpace(r.FormValue("recaptcha_token"))
		if token == "" {
			view := AuthView{
				BaseView:         BaseView{Title: "Rejestracja", IsDev: isDevMode()},
				Error:            "Brak tokenu reCAPTCHA",
				RecaptchaSiteKey: cfg.SiteKey,
			}
			if err := s.templates.Render(w, "register.html", view); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		ok, err := verifyRecaptchaToken(token, "register", cfg)
		if err != nil || !ok {
			view := AuthView{
				BaseView:         BaseView{Title: "Rejestracja", IsDev: isDevMode()},
				Error:            "Nieudana weryfikacja reCAPTCHA",
				RecaptchaSiteKey: cfg.SiteKey,
			}
			if err := s.templates.Render(w, "register.html", view); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
	}
	user := model.User{
		FirstName:    firstName,
		LastName:     lastName,
		Phone:        phone,
		Email:        email,
		PasswordHash: hashPassword(password),
		AvatarURL:    "https://i.pravatar.cc/100?u=" + url.QueryEscape(email),
		Skill:        model.SkillBeginner,
		Role:         model.RoleUser,
	}
	created, err := s.store.CreateUser(user)
	if err != nil {
		view := AuthView{
			BaseView: BaseView{Title: "Rejestracja", IsDev: isDevMode()},
			Error:    "Nie można utworzyć konta: " + err.Error(),
		}
		if err := s.templates.Render(w, "register.html", view); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	setAuthCookie(w, created.ID)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	clearAuthCookie(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func containsUppercase(value string) bool {
	for _, r := range value {
		if r >= 'A' && r <= 'Z' {
			return true
		}
	}
	return false
}

type recaptchaConfig struct {
	SiteKey  string
	Secret   string
	MinScore float64
	Enabled  bool
}

func recaptchaConfigFromEnv() recaptchaConfig {
	enabled := false
	if raw := strings.TrimSpace(os.Getenv("RECAPTCHA_ENABLED")); raw != "" {
		if parsed, err := strconv.ParseBool(raw); err == nil {
			enabled = parsed
		}
	}
	siteKey := strings.TrimSpace(os.Getenv("RECAPTCHA_SITE_KEY"))
	secret := strings.TrimSpace(os.Getenv("RECAPTCHA_SECRET_KEY"))
	minScore := 0.5
	if raw := strings.TrimSpace(os.Getenv("RECAPTCHA_MIN_SCORE")); raw != "" {
		if parsed, err := strconv.ParseFloat(raw, 64); err == nil {
			minScore = parsed
		}
	}
	if !enabled {
		siteKey = ""
		secret = ""
	}

	return recaptchaConfig{
		SiteKey:  siteKey,
		Secret:   secret,
		MinScore: minScore,
		Enabled:  enabled && siteKey != "" && secret != "",
	}
}

type recaptchaVerifyResponse struct {
	Success     bool     `json:"success"`
	Score       float64  `json:"score"`
	Action      string   `json:"action"`
	ErrorCodes  []string `json:"error-codes"`
	ChallengeTS string   `json:"challenge_ts"`
	Hostname    string   `json:"hostname"`
}

func verifyRecaptchaToken(token string, expectedAction string, cfg recaptchaConfig) (bool, error) {
	resp, err := http.PostForm("https://www.google.com/recaptcha/api/siteverify", url.Values{
		"secret":   []string{cfg.Secret},
		"response": []string{token},
	})
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	var payload recaptchaVerifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return false, err
	}
	if !payload.Success {
		return false, nil
	}
	if payload.Action != expectedAction {
		return false, nil
	}
	if payload.Score < cfg.MinScore {
		return false, nil
	}
	return true, nil
}

func (s *Server) currentUser(r *http.Request) model.User {
	cookie, err := r.Cookie(userCookieName)
	if err == nil {
		if user, ok := s.store.GetUser(cookie.Value); ok {
			return user
		}
	}
	return model.User{}
}

func setAuthCookie(w http.ResponseWriter, userID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     userCookieName,
		Value:    userID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(365 * 24 * time.Hour),
	})
}

func clearAuthCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     userCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func hashPassword(password string) string {
	if password == "" {
		return ""
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return ""
	}
	return string(hash)
}

func checkPassword(hash string, password string) bool {
	if hash == "" || password == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
