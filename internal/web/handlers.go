package web

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"sqoush-app/internal/model"
	"sqoush-app/internal/store"
)

type Server struct {
	store     store.Store
	templates *Templates
}

func NewServer(store store.Store, templates *Templates) *Server {
	return &Server{store: store, templates: templates}
}

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	r.Get("/", s.handleHome)
	r.Post("/dev/switch-user", s.handleDevSwitchUser)
	r.Get("/dashboard/tab/recent", s.handleDashboardRecentTab)
	r.Get("/dashboard/tab/pending", s.handleDashboardPendingTab)
	r.Get("/dashboard/tab/friendly", s.handleDashboardFriendlyTab)
	r.Get("/matches/recent", s.handleMatchesRecent)
	r.Get("/matches/pending", s.handleMatchesPending)
	r.Get("/login", s.handleLogin)
	r.Post("/login", s.handleLoginPost)
	r.Get("/register", s.handleRegister)
	r.Post("/register", s.handleRegisterPost)
	r.Post("/logout", s.handleLogout)
	r.Get("/friendlies/new", s.handleFriendlyNew)
	r.Get("/friendlies/search", s.handleFriendlySearch)
	r.Get("/friendlies/select", s.handleFriendlySelect)
	r.Get("/friendlies/dashboard", s.handleFriendlyDashboard)
	r.Post("/friendlies", s.handleFriendlyCreate)
	r.Post("/friendlies/{matchID}/confirm", s.handleFriendlyConfirm)
	r.Post("/friendlies/{matchID}/reject", s.handleFriendlyReject)
	r.Get("/reports/new", s.handleReportNew)
	r.Post("/reports", s.handleReportCreate)
	r.Get("/reports", s.handleReportsList)
	r.Get("/leagues/new", s.handleLeagueNew)
	r.Get("/leagues/search", s.handleLeagueSearch)
	r.Get("/leagues/search/results", s.handleLeagueSearchResults)
	r.Post("/leagues/{leagueID}/join", s.handleLeagueJoin)
	r.Post("/leagues", s.handleLeagueCreate)
	r.Get("/leagues/{leagueID}", s.handleLeagueShow)
	r.Get("/leagues/{leagueID}/players/search", s.handlePlayerSearch)
	r.Post("/leagues/{leagueID}/players", s.handlePlayerAdd)
	r.Post("/leagues/{leagueID}/admins", s.handleLeagueAdminAdd)
	r.Post("/leagues/{leagueID}/admins/{adminID}/role", s.handleLeagueAdminRoleUpdate)
	r.Post("/leagues/{leagueID}/admins/{adminID}/remove", s.handleLeagueAdminRemove)
	r.Post("/leagues/{leagueID}/end", s.handleLeagueEnd)
	r.Post("/leagues/{leagueID}/matches", s.handleMatchCreate)
	r.Post("/matches/{matchID}/confirm", s.handleMatchConfirm)
	r.Post("/matches/{matchID}/reject", s.handleMatchReject)

	return r
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	currentUser := s.currentUser(r)
	users := s.store.ListUsers()
	leagues := s.leaguesForUser(currentUser.ID)
	notice := strings.TrimSpace(r.URL.Query().Get("notice"))
	flash := ""
	switch notice {
	case "friendly_added":
		flash = "Dodano mecz towarzyski."
	case "report_added":
		flash = "Dziękujemy! Zgłoszenie zostało zapisane."
	}
	recentEntries := s.leagueActivityEntries(currentUser, map[model.MatchStatus]bool{
		model.MatchPending:   true,
		model.MatchConfirmed: true,
	})
	sort.Slice(recentEntries, func(i, j int) bool {
		return recentEntries[i].When.After(recentEntries[j].When)
	})

	pendingEntries := s.leagueActivityEntries(currentUser, map[model.MatchStatus]bool{
		model.MatchPending: true,
	})
	sort.Slice(pendingEntries, func(i, j int) bool {
		if pendingEntries[i].Item.CanConfirm != pendingEntries[j].Item.CanConfirm {
			return pendingEntries[i].Item.CanConfirm
		}
		return pendingEntries[i].When.After(pendingEntries[j].When)
	})

	friendlyEntries := s.friendlyActivityEntries(currentUser, map[model.MatchStatus]bool{
		model.MatchPending:   true,
		model.MatchConfirmed: true,
	})
	sort.Slice(friendlyEntries, func(i, j int) bool {
		return friendlyEntries[i].When.After(friendlyEntries[j].When)
	})

	view := HomeView{
		BaseView: BaseView{
			Title:           "Dashboard",
			CurrentUser:     currentUser,
			Users:           users,
			IsAuthenticated: currentUser.ID != "",
			IsDev:           isDevMode(),
			FlashSuccess:    flash,
		},
		Leagues:           leagues,
		DashboardRecent:   buildDashboardTabView(recentEntries, 10, 6, "/matches/recent", "Brak wyników do wyświetlenia."),
		DashboardPending:  buildDashboardTabView(pendingEntries, 0, 6, "/matches/pending", "Brak meczów do potwierdzenia."),
		DashboardFriendly: buildDashboardTabView(friendlyEntries, 0, 6, "/friendlies/dashboard", "Brak wyników do wyświetlenia."),
		LeagueSearch:      s.leagueSearchView("", currentUser),
	}
	if err := s.templates.Render(w, "home.html", view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleDashboardRecentTab(w http.ResponseWriter, r *http.Request) {
	currentUser := s.currentUser(r)
	entries := s.leagueActivityEntries(currentUser, map[model.MatchStatus]bool{
		model.MatchPending:   true,
		model.MatchConfirmed: true,
	})
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].When.After(entries[j].When)
	})
	view := buildDashboardTabView(entries, 10, 6, "/matches/recent", "Brak wyników do wyświetlenia.")
	if err := s.templates.RenderPartial(w, "dashboard_tab.html", view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleDashboardPendingTab(w http.ResponseWriter, r *http.Request) {
	currentUser := s.currentUser(r)
	entries := s.leagueActivityEntries(currentUser, map[model.MatchStatus]bool{
		model.MatchPending: true,
	})
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Item.CanConfirm != entries[j].Item.CanConfirm {
			return entries[i].Item.CanConfirm
		}
		return entries[i].When.After(entries[j].When)
	})
	view := buildDashboardTabView(entries, 0, 6, "/matches/pending", "Brak meczów do potwierdzenia.")
	if err := s.templates.RenderPartial(w, "dashboard_tab.html", view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleDashboardFriendlyTab(w http.ResponseWriter, r *http.Request) {
	currentUser := s.currentUser(r)
	entries := s.friendlyActivityEntries(currentUser, map[model.MatchStatus]bool{
		model.MatchPending:   true,
		model.MatchConfirmed: true,
	})
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].When.After(entries[j].When)
	})
	view := buildDashboardTabView(entries, 0, 6, "/friendlies/dashboard", "Brak wyników do wyświetlenia.")
	if err := s.templates.RenderPartial(w, "dashboard_tab.html", view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleMatchesRecent(w http.ResponseWriter, r *http.Request) {
	currentUser := s.currentUser(r)
	entries := s.leagueActivityEntries(currentUser, map[model.MatchStatus]bool{
		model.MatchPending:   true,
		model.MatchConfirmed: true,
	})
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].When.After(entries[j].When)
	})
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	view := s.buildMatchesListView(entries, page, 10, "/matches/recent", currentUser, "Ostatnio rozgrywane mecze")
	if err := s.templates.Render(w, "matches_list.html", view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleMatchesPending(w http.ResponseWriter, r *http.Request) {
	currentUser := s.currentUser(r)
	entries := s.leagueActivityEntries(currentUser, map[model.MatchStatus]bool{
		model.MatchPending: true,
	})
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Item.CanConfirm != entries[j].Item.CanConfirm {
			return entries[i].Item.CanConfirm
		}
		return entries[i].When.After(entries[j].When)
	})
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	view := s.buildMatchesListView(entries, page, 10, "/matches/pending", currentUser, "Potwierdź mecze")
	if err := s.templates.Render(w, "matches_list.html", view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

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
	if !cfg.Enabled {
		view := AuthView{
			BaseView:         BaseView{Title: "Rejestracja", IsDev: isDevMode()},
			Error:            "reCAPTCHA nie jest skonfigurowana",
			RecaptchaSiteKey: cfg.SiteKey,
		}
		if err := s.templates.Render(w, "register.html", view); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
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

func (s *Server) handleFriendlyNew(w http.ResponseWriter, r *http.Request) {
	currentUser := s.currentUser(r)
	if currentUser.ID == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	view := FriendlyFormView{
		LeagueUsers: s.store.ListUsers(),
		Search: FriendlySearchView{
			EmptyQuery: true,
		},
		SearchInput: FriendlySearchInputView{
			Query: "",
		},
		PlayedAt: time.Now().Format("2006-01-02T15:04"),
	}
	page := struct {
		BaseView
		Form FriendlyFormView
	}{
		BaseView: BaseView{
			Title:           "Nowy mecz towarzyski",
			CurrentUser:     currentUser,
			Users:           s.store.ListUsers(),
			IsAuthenticated: true,
			IsDev:           isDevMode(),
		},
		Form: view,
	}
	if err := s.templates.Render(w, "friendly_new.html", page); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleFriendlyDashboard(w http.ResponseWriter, r *http.Request) {
	currentUser := s.currentUser(r)
	params := friendlyDashboardParamsFromRequest(r)
	view := s.friendlyDashboardView(currentUser, params)
	if isHTMX(r) {
		if err := s.templates.RenderPartial(w, "friendly_dashboard.html", view); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	page := struct {
		BaseView
		Dashboard FriendlyDashboardView
	}{
		BaseView: BaseView{
			Title:           "Mecze towarzyskie",
			CurrentUser:     currentUser,
			Users:           s.store.ListUsers(),
			IsAuthenticated: currentUser.ID != "",
			IsDev:           isDevMode(),
		},
		Dashboard: view,
	}
	if err := s.templates.Render(w, "friendly_dashboard_page.html", page); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleFriendlySearch(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	currentUser := s.currentUser(r)
	view := s.friendlySearchView(query, currentUser.ID)
	if err := s.templates.RenderPartial(w, "friendly_search_results.html", view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleFriendlySelect(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("clear") == "1" {
		view := FriendlyFormView{
			Search: FriendlySearchView{
				EmptyQuery: true,
				SwapOOB:    true,
			},
			SearchInput: FriendlySearchInputView{
				Query:   "",
				SwapOOB: true,
			},
		}
		if err := s.templates.RenderPartial(w, "friendly_selected.html", view); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, "brak gracza", http.StatusBadRequest)
		return
	}
	user, ok := s.store.GetUser(userID)
	if !ok {
		http.Error(w, "nie znaleziono gracza", http.StatusNotFound)
		return
	}
	view := FriendlyFormView{
		Selected: &user,
		Search: FriendlySearchView{
			EmptyQuery: true,
			SwapOOB:    true,
		},
		SearchInput: FriendlySearchInputView{
			Query:   "",
			SwapOOB: true,
		},
	}
	if err := s.templates.RenderPartial(w, "friendly_selected.html", view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleReportNew(w http.ResponseWriter, r *http.Request) {
	currentUser := s.currentUser(r)
	view := struct {
		BaseView
		Form ReportFormView
	}{
		BaseView: BaseView{
			Title:           "Zgłoś",
			CurrentUser:     currentUser,
			Users:           s.store.ListUsers(),
			IsAuthenticated: currentUser.ID != "",
			IsDev:           isDevMode(),
		},
		Form: ReportFormView{},
	}
	if err := s.templates.Render(w, "reports_new.html", view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleReportCreate(w http.ResponseWriter, r *http.Request) {
	currentUser := s.currentUser(r)
	if currentUser.ID == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "nieprawidłowe dane", http.StatusBadRequest)
		return
	}
	rType := strings.TrimSpace(r.FormValue("type"))
	title := strings.TrimSpace(r.FormValue("title"))
	description := strings.TrimSpace(r.FormValue("description"))
	if rType != string(model.ReportBug) && rType != string(model.ReportFeature) {
		s.renderReportFormError(w, currentUser, "Wybierz typ zgłoszenia.", rType, title, description)
		return
	}
	if title == "" || description == "" {
		s.renderReportFormError(w, currentUser, "Wypełnij tytuł i opis.", rType, title, description)
		return
	}
	report := model.Report{
		ID:          uuid.NewString(),
		UserID:      currentUser.ID,
		Type:        model.ReportType(rType),
		Title:       title,
		Description: description,
		Status:      model.ReportOpen,
		CreatedAt:   time.Now(),
	}
	if _, err := s.store.CreateReport(report); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/?notice=report_added", http.StatusSeeOther)
}

func (s *Server) renderReportFormError(w http.ResponseWriter, currentUser model.User, message string, rType string, title string, description string) {
	view := struct {
		BaseView
		Form ReportFormView
	}{
		BaseView: BaseView{
			Title:           "Zgłoś",
			CurrentUser:     currentUser,
			Users:           s.store.ListUsers(),
			IsAuthenticated: currentUser.ID != "",
			IsDev:           isDevMode(),
		},
		Form: ReportFormView{
			Type:        rType,
			Title:       title,
			Description: description,
			Error:       message,
		},
	}
	if err := s.templates.Render(w, "reports_new.html", view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleReportsList(w http.ResponseWriter, r *http.Request) {
	currentUser := s.currentUser(r)
	if !isSuperAdmin(currentUser) {
		http.Error(w, "brak uprawnień", http.StatusForbidden)
		return
	}
	reports := s.store.ListReports()
	items := make([]ReportListItem, 0, len(reports))
	for _, report := range reports {
		user, _ := s.store.GetUser(report.UserID)
		items = append(items, ReportListItem{Report: report, Reporter: user})
	}
	view := ReportsView{
		BaseView: BaseView{
			Title:           "Zgłoszenia",
			CurrentUser:     currentUser,
			Users:           s.store.ListUsers(),
			IsAuthenticated: true,
			IsDev:           isDevMode(),
		},
		Items: items,
	}
	if err := s.templates.Render(w, "reports_list.html", view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleFriendlyCreate(w http.ResponseWriter, r *http.Request) {
	currentUser := s.currentUser(r)
	if currentUser.ID == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "nieprawidłowe dane", http.StatusBadRequest)
		return
	}
	opponentID := r.FormValue("opponent_id")
	if opponentID == "" {
		s.renderFriendlyFormError(w, currentUser, "Nie wybrano przeciwnika", r.FormValue("played_at"))
		return
	}
	opponent, ok := s.store.GetUser(opponentID)
	if !ok {
		s.renderFriendlyFormError(w, currentUser, "Nie wybrano przeciwnika", r.FormValue("played_at"))
		return
	}
	if opponent.ID == currentUser.ID {
		s.renderFriendlyFormError(w, currentUser, "Nie można wybrać siebie", r.FormValue("played_at"))
		return
	}
	setsCount := parseSetsCount(r.FormValue("sets_count"), 20)
	sets := parseSets(r, setsCount)
	if len(sets) == 0 {
		s.renderFriendlyFormError(w, currentUser, "Uzupełnij przynajmniej jeden set (możesz zostawić puste pozostałe).", r.FormValue("played_at"))
		return
	}
	playedAt, err := parsePlayedAt(r.FormValue("played_at"))
	if err != nil {
		http.Error(w, "nieprawidłowa data", http.StatusBadRequest)
		return
	}

	match := model.FriendlyMatch{
		ID:         uuid.NewString(),
		PlayerAID:  currentUser.ID,
		PlayerBID:  opponent.ID,
		Sets:       sets,
		Status:     model.MatchPending,
		ReportedBy: currentUser.ID,
		PlayedAt:   playedAt,
		CreatedAt:  time.Now(),
	}
	if _, err := s.store.CreateFriendlyMatch(match); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if isHTMX(r) {
		view := s.friendlyDashboardView(currentUser, friendlyDashboardParams{})
		if err := s.templates.RenderPartial(w, "friendly_dashboard.html", view); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	http.Redirect(w, r, "/?notice=friendly_added", http.StatusSeeOther)
}

func (s *Server) renderFriendlyFormError(w http.ResponseWriter, currentUser model.User, message string, playedAt string) {
	if playedAt == "" {
		playedAt = time.Now().Format("2006-01-02T15:04")
	}
	view := FriendlyFormView{
		LeagueUsers: s.store.ListUsers(),
		Search: FriendlySearchView{
			EmptyQuery: true,
		},
		SearchInput: FriendlySearchInputView{
			Query: "",
		},
		PlayedAt: playedAt,
		Error:    message,
	}
	page := struct {
		BaseView
		Form FriendlyFormView
	}{
		BaseView: BaseView{
			Title:           "Nowy mecz towarzyski",
			CurrentUser:     currentUser,
			Users:           s.store.ListUsers(),
			IsAuthenticated: true,
			IsDev:           isDevMode(),
		},
		Form: view,
	}
	if err := s.templates.Render(w, "friendly_new.html", page); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleFriendlyConfirm(w http.ResponseWriter, r *http.Request) {
	matchID := chi.URLParam(r, "matchID")
	match, ok := s.store.GetFriendlyMatch(matchID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	currentUser := s.currentUser(r)
	if match.ReportedBy == currentUser.ID {
		http.Error(w, "nie możesz potwierdzić własnego wyniku", http.StatusForbidden)
		return
	}
	match.Status = model.MatchConfirmed
	match.ConfirmedBy = currentUser.ID
	if err := s.store.UpdateFriendlyMatch(match); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if isHTMX(r) {
		view := s.friendlyDashboardView(currentUser, friendlyDashboardParams{})
		if err := s.templates.RenderPartial(w, "friendly_dashboard.html", view); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleFriendlyReject(w http.ResponseWriter, r *http.Request) {
	matchID := chi.URLParam(r, "matchID")
	match, ok := s.store.GetFriendlyMatch(matchID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	currentUser := s.currentUser(r)
	if match.ReportedBy == currentUser.ID {
		http.Error(w, "nie możesz odrzucić własnego wyniku", http.StatusForbidden)
		return
	}
	match.Status = model.MatchRejected
	if err := s.store.UpdateFriendlyMatch(match); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if isHTMX(r) {
		view := s.friendlyDashboardView(currentUser, friendlyDashboardParams{})
		if err := s.templates.RenderPartial(w, "friendly_dashboard.html", view); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleLeagueNew(w http.ResponseWriter, r *http.Request) {
	view := BaseView{
		Title:           "Nowa liga",
		CurrentUser:     s.currentUser(r),
		Users:           s.store.ListUsers(),
		IsAuthenticated: true,
		IsDev:           isDevMode(),
	}
	if err := s.templates.Render(w, "league_new.html", view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleLeagueSearch(w http.ResponseWriter, r *http.Request) {
	currentUser := s.currentUser(r)
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	view := s.leagueSearchView(query, currentUser)
	view.Title = "Wyszukaj ligi"
	view.Users = s.store.ListUsers()
	view.IsAuthenticated = currentUser.ID != ""
	view.IsDev = isDevMode()
	if err := s.templates.Render(w, "league_search.html", view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleLeagueSearchResults(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	currentUser := s.currentUser(r)
	view := s.leagueSearchView(query, currentUser)
	if err := s.templates.RenderPartial(w, "league_search_results.html", view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleLeagueJoin(w http.ResponseWriter, r *http.Request) {
	leagueID := chi.URLParam(r, "leagueID")
	currentUser := s.currentUser(r)
	if currentUser.ID == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	league, ok := s.store.GetLeague(leagueID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := s.store.AddPlayerToLeague(league.ID, currentUser.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/leagues/"+league.ID, http.StatusSeeOther)
}

func (s *Server) handleLeagueCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "nieprawidłowe dane", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	location := strings.TrimSpace(r.FormValue("location"))
	description := strings.TrimSpace(r.FormValue("description"))
	setsPerMatch := parseSetsPerMatch(r.FormValue("sets_per_match"))
	startDate, err := parseLeagueDate(r.FormValue("start_date"))
	if err != nil {
		http.Error(w, "nieprawidłowa data startu", http.StatusBadRequest)
		return
	}
	endDate, err := parseOptionalLeagueDate(r.FormValue("end_date"))
	if err != nil {
		http.Error(w, "nieprawidłowa data końca", http.StatusBadRequest)
		return
	}
	if name == "" {
		http.Error(w, "nazwa jest wymagana", http.StatusBadRequest)
		return
	}
	if endDate != nil && endDate.Before(startDate) {
		http.Error(w, "data końca musi być po dacie startu", http.StatusBadRequest)
		return
	}

	currentUser := s.currentUser(r)
	status := leagueStatusForDates(startDate, endDate, time.Now())
	league := model.League{
		ID:           uuid.NewString(),
		Name:         name,
		Description:  description,
		Location:     location,
		OwnerID:      currentUser.ID,
		AdminRoles:   map[string]model.LeagueAdminRole{currentUser.ID: model.LeagueAdminPlayer},
		PlayerIDs:    []string{currentUser.ID},
		SetsPerMatch: setsPerMatch,
		StartDate:    startDate,
		EndDate:      endDate,
		Status:       status,
		CreatedAt:    time.Now(),
	}
	league, err = s.store.CreateLeague(league)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/leagues/"+league.ID, http.StatusSeeOther)
}

func (s *Server) handleLeagueShow(w http.ResponseWriter, r *http.Request) {
	leagueID := chi.URLParam(r, "leagueID")
	league, ok := s.store.GetLeague(leagueID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	currentUser := s.currentUser(r)
	players := s.leaguePlayers(league)
	setsRange := buildSetsRange(league.SetsPerMatch)

	matches := s.store.ListMatches(league.ID)
	const pageSize = 10
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	totalMatches := len(matches)
	totalPages := int(math.Ceil(float64(totalMatches) / float64(pageSize)))
	if totalPages > 0 && page > totalPages {
		page = totalPages
	}
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > totalMatches {
		start = totalMatches
	}
	if end > totalMatches {
		end = totalMatches
	}
	pagedMatches := matches
	if totalMatches > 0 {
		pagedMatches = matches[start:end]
	}
	matchViews := make([]MatchView, 0, len(pagedMatches))
	for _, match := range pagedMatches {
		matchViews = append(matchViews, s.matchView(match, s.currentUser(r)))
	}

	view := LeagueView{
		BaseView: BaseView{
			Title:           league.Name,
			CurrentUser:     currentUser,
			Users:           s.store.ListUsers(),
			IsAuthenticated: true,
			IsDev:           isDevMode(),
		},
		League:          league,
		Players:         players,
		Standings:       BuildStandings(players, matches),
		Matches:         matchViews,
		SetsRange:       setsRange,
		IsAdmin:         isLeagueAdmin(league, currentUser.ID),
		IsPlayer:        isLeaguePlayer(league, currentUser.ID),
		Admins:          s.leagueAdmins(league),
		AdminCandidates: s.leagueAdminCandidates(league),
		Page:            page,
		TotalPages:      totalPages,
		PlayersPanel: LeaguePlayersPanelView{
			League:  league,
			Players: players,
		},
		PlayerSearch: PlayerSearchView{
			LeagueID:   league.ID,
			EmptyQuery: true,
			CanManage:  isLeagueAdmin(league, currentUser.ID),
		},
	}
	if totalPages > 0 {
		view.Pages = make([]int, 0, totalPages)
		for i := 1; i <= totalPages; i++ {
			view.Pages = append(view.Pages, i)
		}
	}
	view.HasPrev = page > 1
	view.HasNext = totalPages > 0 && page < totalPages
	if view.HasPrev {
		view.PrevPage = page - 1
	}
	if view.HasNext {
		view.NextPage = page + 1
	}

	if err := s.templates.Render(w, "league.html", view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handlePlayerSearch(w http.ResponseWriter, r *http.Request) {
	leagueID := chi.URLParam(r, "leagueID")
	league, ok := s.store.GetLeague(leagueID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	currentUser := s.currentUser(r)
	if !isLeagueAdmin(league, currentUser.ID) {
		http.Error(w, "brak uprawnień", http.StatusForbidden)
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	view := s.playerSearchView(league, currentUser.ID, query, false)
	if err := s.templates.RenderPartial(w, "player_search_results.html", view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handlePlayerAdd(w http.ResponseWriter, r *http.Request) {
	leagueID := chi.URLParam(r, "leagueID")
	league, ok := s.store.GetLeague(leagueID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	currentUser := s.currentUser(r)
	if !isLeagueAdmin(league, currentUser.ID) {
		http.Error(w, "brak uprawnień", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "nieprawidłowe dane", http.StatusBadRequest)
		return
	}
	userID := r.FormValue("user_id")
	if userID == "" {
		http.Error(w, "brak gracza", http.StatusBadRequest)
		return
	}
	if err := s.store.AddPlayerToLeague(league.ID, userID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if isHTMX(r) {
		query := strings.TrimSpace(r.FormValue("q"))
		updatedLeague, _ := s.store.GetLeague(leagueID)
		view := s.playerSearchView(updatedLeague, currentUser.ID, query, true)
		if err := s.templates.RenderPartial(w, "player_search_results.html", view); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	http.Redirect(w, r, "/leagues/"+league.ID, http.StatusSeeOther)
}

func (s *Server) handleLeagueAdminAdd(w http.ResponseWriter, r *http.Request) {
	leagueID := chi.URLParam(r, "leagueID")
	league, ok := s.store.GetLeague(leagueID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	currentUser := s.currentUser(r)
	if !isLeagueAdmin(league, currentUser.ID) {
		http.Error(w, "brak uprawnień", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "nieprawidłowe dane", http.StatusBadRequest)
		return
	}
	userID := r.FormValue("user_id")
	if userID == "" {
		http.Error(w, "brak gracza", http.StatusBadRequest)
		return
	}
	role := model.LeagueAdminRole(strings.TrimSpace(r.FormValue("role")))
	if role != model.LeagueAdminPlayer && role != model.LeagueAdminModerator {
		http.Error(w, "nieprawidłowa rola", http.StatusBadRequest)
		return
	}
	if err := s.store.AddAdminToLeague(league.ID, userID, role); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/leagues/"+league.ID, http.StatusSeeOther)
}

func (s *Server) handleLeagueAdminRoleUpdate(w http.ResponseWriter, r *http.Request) {
	leagueID := chi.URLParam(r, "leagueID")
	adminID := chi.URLParam(r, "adminID")
	league, ok := s.store.GetLeague(leagueID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	currentUser := s.currentUser(r)
	if !isLeagueAdmin(league, currentUser.ID) {
		http.Error(w, "brak uprawnień", http.StatusForbidden)
		return
	}
	if adminID == league.OwnerID {
		http.Error(w, "nie można zmienić roli właściciela", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "nieprawidłowe dane", http.StatusBadRequest)
		return
	}
	role := model.LeagueAdminRole(strings.TrimSpace(r.FormValue("role")))
	if role != model.LeagueAdminPlayer && role != model.LeagueAdminModerator {
		http.Error(w, "nieprawidłowa rola", http.StatusBadRequest)
		return
	}
	if err := s.store.UpdateAdminRole(league.ID, adminID, role); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/leagues/"+league.ID, http.StatusSeeOther)
}

func (s *Server) handleLeagueAdminRemove(w http.ResponseWriter, r *http.Request) {
	leagueID := chi.URLParam(r, "leagueID")
	adminID := chi.URLParam(r, "adminID")
	league, ok := s.store.GetLeague(leagueID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	currentUser := s.currentUser(r)
	if !isLeagueAdmin(league, currentUser.ID) {
		http.Error(w, "brak uprawnień", http.StatusForbidden)
		return
	}
	if adminID == league.OwnerID {
		http.Error(w, "nie można usunąć właściciela", http.StatusBadRequest)
		return
	}
	if err := s.store.RemoveAdminFromLeague(league.ID, adminID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/leagues/"+league.ID, http.StatusSeeOther)
}

func (s *Server) handleLeagueEnd(w http.ResponseWriter, r *http.Request) {
	leagueID := chi.URLParam(r, "leagueID")
	league, ok := s.store.GetLeague(leagueID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	currentUser := s.currentUser(r)
	if !isLeagueAdmin(league, currentUser.ID) {
		http.Error(w, "brak uprawnień", http.StatusForbidden)
		return
	}
	league.Status = model.LeagueStatusFinished
	if err := s.store.UpdateLeague(league); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/leagues/"+league.ID, http.StatusSeeOther)
}
func (s *Server) handleMatchCreate(w http.ResponseWriter, r *http.Request) {
	leagueID := chi.URLParam(r, "leagueID")
	league, ok := s.store.GetLeague(leagueID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	currentUser := s.currentUser(r)
	if !isLeagueAdmin(league, currentUser.ID) && !isLeaguePlayer(league, currentUser.ID) {
		http.Error(w, "brak uprawnień", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "nieprawidłowe dane", http.StatusBadRequest)
		return
	}
	playerA := r.FormValue("player_a")
	playerB := r.FormValue("player_b")
	if playerA == "" || playerB == "" || playerA == playerB {
		http.Error(w, "wybierz dwóch różnych graczy", http.StatusBadRequest)
		return
	}
	sets := parseSets(r, league.SetsPerMatch)
	if len(sets) == 0 {
		http.Error(w, "uzupełnij wynik", http.StatusBadRequest)
		return
	}

	match := model.Match{
		ID:         uuid.NewString(),
		LeagueID:   league.ID,
		PlayerAID:  playerA,
		PlayerBID:  playerB,
		Sets:       sets,
		Status:     model.MatchPending,
		ReportedBy: currentUser.ID,
		CreatedAt:  time.Now(),
	}
	match, err := s.store.CreateMatch(match)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if isHTMX(r) {
		view := s.matchView(match, currentUser)
		if err := s.templates.RenderPartial(w, "match_row.html", view); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	http.Redirect(w, r, "/leagues/"+league.ID, http.StatusSeeOther)
}

func (s *Server) handleMatchConfirm(w http.ResponseWriter, r *http.Request) {
	matchID := chi.URLParam(r, "matchID")
	match, ok := s.store.GetMatch(matchID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	currentUser := s.currentUser(r)
	if match.ReportedBy == currentUser.ID {
		http.Error(w, "nie możesz potwierdzić własnego wyniku", http.StatusForbidden)
		return
	}
	match.Status = model.MatchConfirmed
	match.ConfirmedBy = currentUser.ID
	if err := s.store.UpdateMatch(match); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if isHTMX(r) {
		view := s.matchView(match, currentUser)
		if err := s.templates.RenderPartial(w, "match_row.html", view); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	http.Redirect(w, r, "/leagues/"+match.LeagueID, http.StatusSeeOther)
}

func (s *Server) handleMatchReject(w http.ResponseWriter, r *http.Request) {
	matchID := chi.URLParam(r, "matchID")
	match, ok := s.store.GetMatch(matchID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	currentUser := s.currentUser(r)
	if match.ReportedBy == currentUser.ID {
		http.Error(w, "nie możesz odrzucić własnego wyniku", http.StatusForbidden)
		return
	}
	match.Status = model.MatchRejected
	if err := s.store.UpdateMatch(match); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if isHTMX(r) {
		view := s.matchView(match, currentUser)
		if err := s.templates.RenderPartial(w, "match_row.html", view); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	http.Redirect(w, r, "/leagues/"+match.LeagueID, http.StatusSeeOther)
}

func (s *Server) matchView(match model.Match, currentUser model.User) MatchView {
	playerA, _ := s.store.GetUser(match.PlayerAID)
	playerB, _ := s.store.GetUser(match.PlayerBID)

	scoreParts := make([]string, 0, len(match.Sets))
	for _, set := range match.Sets {
		scoreParts = append(scoreParts, fmt.Sprintf("%d:%d", set.A, set.B))
	}
	scoreLine := strings.Join(scoreParts, ", ")

	statusText := map[model.MatchStatus]string{
		model.MatchScheduled: "Zaplanowany",
		model.MatchPending:   "Oczekuje na potwierdzenie",
		model.MatchConfirmed: "Potwierdzony",
		model.MatchRejected:  "Odrzucony",
	}[match.Status]

	canConfirm := match.Status == model.MatchPending && match.ReportedBy != currentUser.ID && (currentUser.ID == match.PlayerAID || currentUser.ID == match.PlayerBID)
	canReject := canConfirm

	return MatchView{
		Match:      match,
		PlayerA:    playerA,
		PlayerB:    playerB,
		ScoreLine:  scoreLine,
		CanConfirm: canConfirm,
		CanReject:  canReject,
		StatusText: statusText,
	}
}

func parseLeagueDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("missing date")
	}
	return time.Parse("2006-01-02", value)
}

func parseOptionalLeagueDate(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

type friendlyDashboardParams struct {
	Period     string
	Month      int
	Year       int
	OpponentID string
	Page       int
}

func friendlyDashboardParamsFromRequest(r *http.Request) friendlyDashboardParams {
	q := r.URL.Query()
	period := q.Get("period")
	if period == "" {
		period = "month"
	}
	month, _ := strconv.Atoi(q.Get("month"))
	year, _ := strconv.Atoi(q.Get("year"))
	page, _ := strconv.Atoi(q.Get("page"))
	return friendlyDashboardParams{
		Period:     period,
		Month:      month,
		Year:       year,
		OpponentID: strings.TrimSpace(q.Get("opponent_id")),
		Page:       page,
	}
}

func (s *Server) friendlyDashboardView(currentUser model.User, params friendlyDashboardParams) FriendlyDashboardView {
	matches := s.store.ListFriendlyMatches()
	now := time.Now()
	period := params.Period
	if period == "" {
		period = "month"
	}
	customMonth := params.Month
	customYear := params.Year
	if customMonth < 1 || customMonth > 12 {
		customMonth = int(now.Month())
	}
	if customYear < 1 {
		customYear = now.Year()
	}

	filtered := make([]model.FriendlyMatch, 0)
	for _, match := range matches {
		if !matchBelongsToPeriod(match, period, customMonth, customYear, now) {
			continue
		}
		if params.OpponentID != "" {
			if match.PlayerAID != params.OpponentID && match.PlayerBID != params.OpponentID {
				continue
			}
		}
		filtered = append(filtered, match)
	}

	const pageSize = 10
	totalMatches := len(filtered)
	totalPages := int(math.Ceil(float64(totalMatches) / float64(pageSize)))
	page := params.Page
	if page < 1 {
		page = 1
	}
	if totalPages > 0 && page > totalPages {
		page = totalPages
	}
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > totalMatches {
		start = totalMatches
	}
	if end > totalMatches {
		end = totalMatches
	}

	view := FriendlyDashboardView{
		Period:       period,
		CustomMonth:  customMonth,
		CustomYear:   customYear,
		MonthOptions: monthOptions(),
		YearOptions:  yearOptions(matches, now.Year()),
		OpponentID:   params.OpponentID,
		Opponents:    s.opponentsForUser(currentUser.ID),
		Matches:      []FriendlyMatchView{},
		Page:         page,
		TotalPages:   totalPages,
	}

	paged := filtered
	if totalMatches > 0 {
		paged = filtered[start:end]
	}

	for _, match := range paged {
		view.Matches = append(view.Matches, s.friendlyMatchView(match, currentUser))
	}

	if params.OpponentID != "" {
		if opponent, ok := s.store.GetUser(params.OpponentID); ok {
			summary := buildFriendlySummary(currentUser.ID, opponent.FullName(), filtered)
			view.Summary = &summary
		}
	}

	if totalPages > 0 {
		view.Pages = make([]int, 0, totalPages)
		for i := 1; i <= totalPages; i++ {
			view.Pages = append(view.Pages, i)
		}
	}
	view.HasPrev = page > 1
	view.HasNext = totalPages > 0 && page < totalPages
	if view.HasPrev {
		view.PrevPage = page - 1
	}
	if view.HasNext {
		view.NextPage = page + 1
	}

	return view
}

func isLeagueAdmin(league model.League, userID string) bool {
	if userID == "" {
		return false
	}
	if league.OwnerID == userID {
		return true
	}
	if league.AdminRoles != nil {
		if _, ok := league.AdminRoles[userID]; ok {
			return true
		}
	}
	return false
}

func isLeaguePlayer(league model.League, userID string) bool {
	if userID == "" {
		return false
	}
	for _, id := range league.PlayerIDs {
		if id == userID {
			return true
		}
	}
	return false
}

func isSuperAdmin(user model.User) bool {
	return user.Role == model.RoleSuperAdmin
}

func isDevMode() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("APP")), "dev")
}

func leagueStatusForDates(start time.Time, end *time.Time, now time.Time) model.LeagueStatus {
	if end != nil && end.Before(now) {
		return model.LeagueStatusFinished
	}
	if start.After(now) {
		return model.LeagueStatusUpcoming
	}
	return model.LeagueStatusActive
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
	siteKey := strings.TrimSpace(os.Getenv("RECAPTCHA_SITE_KEY"))
	secret := strings.TrimSpace(os.Getenv("RECAPTCHA_SECRET_KEY"))
	minScore := 0.5
	if raw := strings.TrimSpace(os.Getenv("RECAPTCHA_MIN_SCORE")); raw != "" {
		if parsed, err := strconv.ParseFloat(raw, 64); err == nil {
			minScore = parsed
		}
	}
	return recaptchaConfig{
		SiteKey:  siteKey,
		Secret:   secret,
		MinScore: minScore,
		Enabled:  siteKey != "" && secret != "",
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

func (s *Server) leagueAdmins(league model.League) []LeagueAdminView {
	seen := map[string]struct{}{}
	admins := make([]LeagueAdminView, 0)
	if league.OwnerID != "" {
		if u, ok := s.store.GetUser(league.OwnerID); ok {
			admins = append(admins, LeagueAdminView{User: u, Role: model.LeagueAdminPlayer})
			seen[league.OwnerID] = struct{}{}
		}
	}
	for id, role := range league.AdminRoles {
		if _, ok := seen[id]; ok {
			continue
		}
		if u, ok := s.store.GetUser(id); ok {
			admins = append(admins, LeagueAdminView{User: u, Role: role})
		}
	}
	return admins
}

func (s *Server) leagueAdminCandidates(league model.League) []model.User {
	admins := map[string]struct{}{}
	if league.OwnerID != "" {
		admins[league.OwnerID] = struct{}{}
	}
	for id := range league.AdminRoles {
		admins[id] = struct{}{}
	}
	users := s.store.ListUsers()
	candidates := make([]model.User, 0, len(users))
	for _, u := range users {
		if _, ok := admins[u.ID]; ok {
			continue
		}
		candidates = append(candidates, u)
	}
	return candidates
}

func (s *Server) leaguesForUser(userID string) []model.League {
	if userID == "" {
		return nil
	}
	leagues := s.store.ListLeagues()
	filtered := make([]model.League, 0)
	for _, league := range leagues {
		if isLeaguePlayer(league, userID) || isLeagueAdmin(league, userID) {
			filtered = append(filtered, league)
		}
	}
	return filtered
}

func (s *Server) searchLeagues(query string) []model.League {
	if query == "" {
		return nil
	}
	leagues := s.store.ListLeagues()
	results := []model.League{}
	lower := strings.ToLower(query)
	for _, league := range leagues {
		if strings.Contains(strings.ToLower(league.Name), lower) ||
			strings.Contains(strings.ToLower(league.Description), lower) ||
			strings.Contains(strings.ToLower(league.Location), lower) {
			results = append(results, league)
		}
	}
	return results
}

func (s *Server) leagueSearchView(query string, currentUser model.User) LeagueSearchView {
	results := s.searchLeagues(query)
	items := make([]LeagueSearchResultView, 0, len(results))
	for _, league := range results {
		items = append(items, LeagueSearchResultView{
			League:   league,
			InLeague: isLeaguePlayer(league, currentUser.ID) || isLeagueAdmin(league, currentUser.ID),
		})
	}
	return LeagueSearchView{
		BaseView: BaseView{
			CurrentUser: currentUser,
		},
		Query:      query,
		Results:    items,
		EmptyQuery: query == "",
	}
}

type activityEntry struct {
	When time.Time
	Item RecentActivityItem
}

func (s *Server) leagueActivityEntries(currentUser model.User, statuses map[model.MatchStatus]bool) []activityEntry {
	entries := []activityEntry{}
	for _, league := range s.leaguesForUser(currentUser.ID) {
		matches := s.store.ListMatches(league.ID)
		for _, match := range matches {
			if match.PlayerAID != currentUser.ID && match.PlayerBID != currentUser.ID {
				continue
			}
			if !statuses[match.Status] {
				continue
			}
			view := s.matchView(match, currentUser)
			item := RecentActivityItem{
				Kind:          "league",
				MatchID:       match.ID,
				PlayerA:       view.PlayerA,
				PlayerB:       view.PlayerB,
				ScoreLine:     view.ScoreLine,
				StatusText:    view.StatusText,
				CanConfirm:    view.CanConfirm,
				CanReject:     view.CanReject,
				PlayedAtLabel: match.CreatedAt.Format("02 Jan 2006 15:04"),
				LeagueName:    league.Name,
			}
			entries = append(entries, activityEntry{
				When: match.CreatedAt,
				Item: item,
			})
		}
	}
	return entries
}

func (s *Server) friendlyActivityEntries(currentUser model.User, statuses map[model.MatchStatus]bool) []activityEntry {
	entries := []activityEntry{}
	for _, match := range s.store.ListFriendlyMatches() {
		if match.PlayerAID != currentUser.ID && match.PlayerBID != currentUser.ID {
			continue
		}
		if !statuses[match.Status] {
			continue
		}
		view := s.friendlyMatchView(match, currentUser)
		item := RecentActivityItem{
			Kind:          "friendly",
			MatchID:       match.ID,
			PlayerA:       view.PlayerA,
			PlayerB:       view.PlayerB,
			ScoreLine:     view.ScoreLine,
			StatusText:    view.StatusText,
			CanConfirm:    view.CanConfirm,
			CanReject:     view.CanReject,
			PlayedAtLabel: view.PlayedAtLabel,
		}
		entries = append(entries, activityEntry{
			When: match.PlayedAt,
			Item: item,
		})
	}
	return entries
}

func buildDashboardTabView(entries []activityEntry, maxTotal int, maxDisplay int, moreURL string, emptyText string) DashboardTabView {
	total := len(entries)
	if maxTotal > 0 && total > maxTotal {
		entries = entries[:maxTotal]
	}
	items := make([]RecentActivityItem, 0, len(entries))
	for _, entry := range entries {
		items = append(items, entry.Item)
	}
	view := DashboardTabView{
		Items:     items,
		HasMore:   total > maxDisplay,
		MoreURL:   moreURL,
		EmptyText: emptyText,
	}
	if len(view.Items) > maxDisplay {
		view.Items = view.Items[:maxDisplay]
	}
	return view
}

func (s *Server) buildMatchesListView(entries []activityEntry, page int, pageSize int, basePath string, currentUser model.User, title string) MatchesListView {
	if page < 1 {
		page = 1
	}
	total := len(entries)
	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))
	if totalPages > 0 && page > totalPages {
		page = totalPages
	}
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	items := make([]RecentActivityItem, 0, pageSize)
	for _, entry := range entries[start:end] {
		items = append(items, entry.Item)
	}

	view := MatchesListView{
		BaseView: BaseView{
			Title:           title,
			CurrentUser:     currentUser,
			Users:           s.store.ListUsers(),
			IsAuthenticated: currentUser.ID != "",
			IsDev:           isDevMode(),
		},
		Items:      items,
		Page:       page,
		TotalPages: totalPages,
		BasePath:   basePath,
	}
	if totalPages > 0 {
		view.Pages = make([]int, 0, totalPages)
		for i := 1; i <= totalPages; i++ {
			view.Pages = append(view.Pages, i)
		}
	}
	view.HasPrev = page > 1
	view.HasNext = totalPages > 0 && page < totalPages
	if view.HasPrev {
		view.PrevPage = page - 1
	}
	if view.HasNext {
		view.NextPage = page + 1
	}
	return view
}

func (s *Server) friendlyMatchView(match model.FriendlyMatch, currentUser model.User) FriendlyMatchView {
	playerA, _ := s.store.GetUser(match.PlayerAID)
	playerB, _ := s.store.GetUser(match.PlayerBID)

	scoreParts := make([]string, 0, len(match.Sets))
	for _, set := range match.Sets {
		scoreParts = append(scoreParts, fmt.Sprintf("%d:%d", set.A, set.B))
	}
	scoreLine := strings.Join(scoreParts, ", ")

	statusText := map[model.MatchStatus]string{
		model.MatchScheduled: "Zaplanowany",
		model.MatchPending:   "Oczekuje na potwierdzenie",
		model.MatchConfirmed: "Potwierdzony",
		model.MatchRejected:  "Odrzucony",
	}[match.Status]

	canConfirm := match.Status == model.MatchPending && match.ReportedBy != currentUser.ID && (currentUser.ID == match.PlayerAID || currentUser.ID == match.PlayerBID)
	canReject := canConfirm

	return FriendlyMatchView{
		Match:         match,
		PlayerA:       playerA,
		PlayerB:       playerB,
		ScoreLine:     scoreLine,
		PlayedAtLabel: match.PlayedAt.Format("02 Jan 2006 15:04"),
		CanConfirm:    canConfirm,
		CanReject:     canReject,
		StatusText:    statusText,
	}
}

func (s *Server) opponentsForUser(userID string) []model.User {
	users := s.store.ListUsers()
	opponents := make([]model.User, 0, len(users))
	for _, u := range users {
		if u.ID == userID {
			continue
		}
		opponents = append(opponents, u)
	}
	return opponents
}

func matchBelongsToPeriod(match model.FriendlyMatch, period string, month int, year int, now time.Time) bool {
	switch period {
	case "year":
		return match.PlayedAt.Year() == now.Year()
	case "custom":
		return match.PlayedAt.Year() == year && int(match.PlayedAt.Month()) == month
	default:
		return match.PlayedAt.Year() == now.Year() && match.PlayedAt.Month() == now.Month()
	}
}

func monthOptions() []MonthOption {
	return []MonthOption{
		{Value: 1, Label: "Styczen"},
		{Value: 2, Label: "Luty"},
		{Value: 3, Label: "Marzec"},
		{Value: 4, Label: "Kwiecien"},
		{Value: 5, Label: "Maj"},
		{Value: 6, Label: "Czerwiec"},
		{Value: 7, Label: "Lipiec"},
		{Value: 8, Label: "Sierpien"},
		{Value: 9, Label: "Wrzesien"},
		{Value: 10, Label: "Pazdziernik"},
		{Value: 11, Label: "Listopad"},
		{Value: 12, Label: "Grudzien"},
	}
}

func yearOptions(matches []model.FriendlyMatch, currentYear int) []int {
	years := map[int]struct{}{currentYear: {}}
	for _, match := range matches {
		years[match.PlayedAt.Year()] = struct{}{}
	}
	result := make([]int, 0, len(years))
	for y := range years {
		result = append(result, y)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(result)))
	return result
}

func buildFriendlySummary(currentUserID string, opponentName string, matches []model.FriendlyMatch) FriendlySummaryView {
	summary := FriendlySummaryView{OpponentName: opponentName}
	for _, match := range matches {
		if match.Status != model.MatchConfirmed {
			continue
		}
		summary.Matches++
		setsWon := 0
		setsLost := 0
		pointsWon := 0
		pointsLost := 0
		for _, set := range match.Sets {
			a := set.A
			b := set.B
			if match.PlayerAID != currentUserID {
				a, b = b, a
			}
			pointsWon += a
			pointsLost += b
			if a > b {
				setsWon++
			} else if b > a {
				setsLost++
			}
		}
		summary.SetsWon += setsWon
		summary.SetsLost += setsLost
		summary.PointsWon += pointsWon
		summary.PointsLost += pointsLost
		if setsWon > setsLost {
			summary.Wins++
		} else if setsLost > setsWon {
			summary.Losses++
		}
	}
	return summary
}

func (s *Server) friendlySearchView(query string, excludeUserID string) FriendlySearchView {
	results := []FriendlySearchResult{}
	if query != "" {
		lowerQuery := strings.ToLower(query)
		for _, user := range s.store.ListUsers() {
			if user.ID == excludeUserID {
				continue
			}
			name := strings.ToLower(user.FullName())
			email := strings.ToLower(user.Email)
			if !strings.Contains(name, lowerQuery) && !strings.Contains(email, lowerQuery) {
				continue
			}
			results = append(results, FriendlySearchResult{User: user})
		}
	}
	return FriendlySearchView{
		Query:      query,
		Results:    results,
		EmptyQuery: query == "",
	}
}

func (s *Server) leaguePlayers(league model.League) []model.User {
	players := make([]model.User, 0, len(league.PlayerIDs))
	for _, playerID := range league.PlayerIDs {
		if u, ok := s.store.GetUser(playerID); ok {
			players = append(players, u)
		}
	}
	return players
}

func (s *Server) playerSearchView(league model.League, currentUserID string, query string, includePanel bool) PlayerSearchView {
	players := s.leaguePlayers(league)
	inLeague := make(map[string]struct{}, len(league.PlayerIDs))
	for _, id := range league.PlayerIDs {
		inLeague[id] = struct{}{}
	}

	results := []PlayerSearchResult{}
	if query != "" {
		lowerQuery := strings.ToLower(query)
		for _, user := range s.store.ListUsers() {
			name := strings.ToLower(user.FullName())
			email := strings.ToLower(user.Email)
			if !strings.Contains(name, lowerQuery) && !strings.Contains(email, lowerQuery) {
				continue
			}
			_, exists := inLeague[user.ID]
			results = append(results, PlayerSearchResult{
				User:     user,
				InLeague: exists,
			})
		}
	}

	view := PlayerSearchView{
		LeagueID:   league.ID,
		Query:      query,
		Results:    results,
		EmptyQuery: query == "",
		CanManage:  isLeagueAdmin(league, currentUserID),
	}
	if includePanel {
		view.IncludePanel = true
		view.PlayersPanel = LeaguePlayersPanelView{
			League:  league,
			Players: players,
			SwapOOB: true,
		}
	}
	return view
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

func parseSets(r *http.Request, maxSets int) []model.SetScore {
	sets := []model.SetScore{}
	if maxSets < 1 {
		maxSets = 5
	}
	for i := 1; i <= maxSets; i++ {
		aStr := r.FormValue(fmt.Sprintf("set_%d_a", i))
		bStr := r.FormValue(fmt.Sprintf("set_%d_b", i))
		if aStr == "" || bStr == "" {
			continue
		}
		a, errA := strconv.Atoi(aStr)
		b, errB := strconv.Atoi(bStr)
		if errA != nil || errB != nil {
			continue
		}
		sets = append(sets, model.SetScore{A: a, B: b})
	}
	return sets
}

func buildSetsRange(maxSets int) []int {
	if maxSets < 1 {
		maxSets = 5
	}
	sets := make([]int, 0, maxSets)
	for i := 1; i <= maxSets; i++ {
		sets = append(sets, i)
	}
	return sets
}

func parseSetsPerMatch(value string) int {
	if value == "" {
		return 5
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return 5
	}
	if parsed > 9 {
		return 9
	}
	return parsed
}

func parseSetsCount(value string, max int) int {
	if value == "" {
		return 5
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return 5
	}
	if max > 0 && parsed > max {
		return max
	}
	return parsed
}

func parsePlayedAt(value string) (time.Time, error) {
	if value == "" {
		return time.Now(), nil
	}
	parsed, err := time.ParseInLocation("2006-01-02T15:04", value, time.Local)
	if err != nil {
		return time.Time{}, err
	}
	return parsed, nil
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

func isHTMX(r *http.Request) bool {
	return strings.ToLower(r.Header.Get("HX-Request")) == "true"
}
