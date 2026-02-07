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
	r.Get("/leagues/new", s.handleLeagueNew)
	r.Get("/leagues/search", s.handleLeagueSearch)
	r.Get("/leagues/search/results", s.handleLeagueSearchResults)
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

	recentActivity := []RecentActivityItem{}
	recentPending := []RecentActivityItem{}
	type activityEntry struct {
		PlayedAt time.Time
		Item     RecentActivityItem
	}
	activityEntries := []activityEntry{}
	recentFriendlies := []FriendlyMatchView{}

	pendingEntries := []activityEntry{}
	for _, league := range leagues {
		matches := s.store.ListMatches(league.ID)
		for _, match := range matches {
			if match.PlayerAID != currentUser.ID && match.PlayerBID != currentUser.ID {
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
			if match.Status == model.MatchConfirmed {
				activityEntries = append(activityEntries, activityEntry{
					PlayedAt: match.CreatedAt,
					Item:     item,
				})
			} else if match.Status == model.MatchPending {
				pendingEntries = append(pendingEntries, activityEntry{
					PlayedAt: match.CreatedAt,
					Item:     item,
				})
			}
		}
	}

	for _, match := range s.store.ListFriendlyMatches() {
		if match.PlayerAID != currentUser.ID && match.PlayerBID != currentUser.ID {
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
		if match.Status == model.MatchConfirmed {
			activityEntries = append(activityEntries, activityEntry{
				PlayedAt: match.PlayedAt,
				Item:     item,
			})
		} else if match.Status == model.MatchPending {
			pendingEntries = append(pendingEntries, activityEntry{
				PlayedAt: match.PlayedAt,
				Item:     item,
			})
		}
		if match.Status == model.MatchConfirmed {
			recentFriendlies = append(recentFriendlies, view)
			if len(recentFriendlies) >= 10 {
				break
			}
		}
	}

	sort.Slice(activityEntries, func(i, j int) bool {
		return activityEntries[i].PlayedAt.After(activityEntries[j].PlayedAt)
	})
	for _, entry := range activityEntries {
		recentActivity = append(recentActivity, entry.Item)
		if len(recentActivity) >= 10 {
			break
		}
	}
	sort.Slice(pendingEntries, func(i, j int) bool {
		return pendingEntries[i].PlayedAt.After(pendingEntries[j].PlayedAt)
	})
	for _, entry := range pendingEntries {
		recentPending = append(recentPending, entry.Item)
		if len(recentPending) >= 10 {
			break
		}
	}

	view := HomeView{
		BaseView: BaseView{
			Title:       "Dashboard",
			CurrentUser: currentUser,
			Users:       users,
			IsAuthenticated: currentUser.ID != "",
		},
		Leagues:               leagues,
		RecentActivity:        recentActivity,
		RecentPending:         recentPending,
		RecentFriendlyMatches: recentFriendlies,
	}
	if err := s.templates.Render(w, "home.html", view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	view := AuthView{
		BaseView: BaseView{
			Title: "Logowanie",
		},
	}
	if err := s.templates.Render(w, "login.html", view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleLoginPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "nieprawidlowe dane", http.StatusBadRequest)
		return
	}
	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")
	user, ok := s.store.GetUserByEmail(email)
	if !ok || !checkPassword(user.PasswordHash, password) {
		view := AuthView{
			BaseView: BaseView{Title: "Logowanie"},
			Error:    "Nieprawidlowy email lub haslo",
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
		},
		RecaptchaSiteKey: cfg.SiteKey,
	}
	if err := s.templates.Render(w, "register.html", view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleRegisterPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "nieprawidlowe dane", http.StatusBadRequest)
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
			BaseView: BaseView{Title: "Rejestracja"},
			Error:    "Wypelnij wszystkie pola",
			RecaptchaSiteKey: cfg.SiteKey,
		}
		if err := s.templates.Render(w, "register.html", view); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	if len(password) < 8 {
		view := AuthView{
			BaseView: BaseView{Title: "Rejestracja"},
			Error:    "Haslo musi miec min. 8 znakow",
			RecaptchaSiteKey: cfg.SiteKey,
		}
		if err := s.templates.Render(w, "register.html", view); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	if !containsUppercase(password) {
		view := AuthView{
			BaseView: BaseView{Title: "Rejestracja"},
			Error:    "Haslo musi zawierac jedna duza litere",
			RecaptchaSiteKey: cfg.SiteKey,
		}
		if err := s.templates.Render(w, "register.html", view); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	if password != confirmPassword {
		view := AuthView{
			BaseView: BaseView{Title: "Rejestracja"},
			Error:    "Hasla nie sa takie same",
			RecaptchaSiteKey: cfg.SiteKey,
		}
		if err := s.templates.Render(w, "register.html", view); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	if !cfg.Enabled {
		view := AuthView{
			BaseView: BaseView{Title: "Rejestracja"},
			Error:    "reCAPTCHA nie jest skonfigurowana",
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
			BaseView: BaseView{Title: "Rejestracja"},
			Error:    "Brak tokenu reCAPTCHA",
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
			BaseView: BaseView{Title: "Rejestracja"},
			Error:    "Nieudana weryfikacja reCAPTCHA",
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
			BaseView: BaseView{Title: "Rejestracja"},
			Error:    "Nie mozna utworzyc konta: " + err.Error(),
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
			Title:       "Nowy mecz towarzyski",
			CurrentUser: currentUser,
			Users:       s.store.ListUsers(),
			IsAuthenticated: true,
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
	if err := s.templates.RenderPartial(w, "friendly_dashboard.html", view); err != nil {
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

func (s *Server) handleFriendlyCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "nieprawidlowe dane", http.StatusBadRequest)
		return
	}
	opponentID := r.FormValue("opponent_id")
	if opponentID == "" {
		http.Error(w, "wybierz przeciwnika", http.StatusBadRequest)
		return
	}
	opponent, ok := s.store.GetUser(opponentID)
	if !ok {
		http.Error(w, "nie znaleziono gracza", http.StatusNotFound)
		return
	}
	currentUser := s.currentUser(r)
	if opponent.ID == currentUser.ID {
		http.Error(w, "nie mozesz wybrac siebie", http.StatusBadRequest)
		return
	}
	setsCount := parseSetsCount(r.FormValue("sets_count"), 20)
	sets := parseSets(r, setsCount)
	if len(sets) == 0 {
		http.Error(w, "uzupelnij wynik", http.StatusBadRequest)
		return
	}
	playedAt, err := parsePlayedAt(r.FormValue("played_at"))
	if err != nil {
		http.Error(w, "nieprawidlowa data", http.StatusBadRequest)
		return
	}

	match := model.FriendlyMatch{
		ID:        uuid.NewString(),
		PlayerAID: currentUser.ID,
		PlayerBID: opponent.ID,
		Sets:      sets,
		Status:    model.MatchPending,
		ReportedBy: currentUser.ID,
		PlayedAt:  playedAt,
		CreatedAt: time.Now(),
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
	http.Redirect(w, r, "/", http.StatusSeeOther)
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
		http.Error(w, "nie mozesz potwierdzic wlasnego wyniku", http.StatusForbidden)
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
		http.Error(w, "nie mozesz odrzucic wlasnego wyniku", http.StatusForbidden)
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
		Title:       "Nowa liga",
		CurrentUser: s.currentUser(r),
		Users:       s.store.ListUsers(),
		IsAuthenticated: true,
	}
	if err := s.templates.Render(w, "league_new.html", view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleLeagueSearch(w http.ResponseWriter, r *http.Request) {
	currentUser := s.currentUser(r)
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	results := s.searchLeagues(query)
	view := LeagueSearchView{
		BaseView: BaseView{
			Title:       "Wyszukaj ligi",
			CurrentUser: currentUser,
			Users:       s.store.ListUsers(),
			IsAuthenticated: currentUser.ID != "",
		},
		Query:      query,
		Results:    results,
		EmptyQuery: query == "",
	}
	if err := s.templates.Render(w, "league_search.html", view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleLeagueSearchResults(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	view := LeagueSearchView{
		Query:      query,
		Results:    s.searchLeagues(query),
		EmptyQuery: query == "",
	}
	if err := s.templates.RenderPartial(w, "league_search_results.html", view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleLeagueCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "nieprawidlowe dane", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	location := strings.TrimSpace(r.FormValue("location"))
	description := strings.TrimSpace(r.FormValue("description"))
	setsPerMatch := parseSetsPerMatch(r.FormValue("sets_per_match"))
	startDate, err := parseLeagueDate(r.FormValue("start_date"))
	if err != nil {
		http.Error(w, "nieprawidlowa data startu", http.StatusBadRequest)
		return
	}
	endDate, err := parseOptionalLeagueDate(r.FormValue("end_date"))
	if err != nil {
		http.Error(w, "nieprawidlowa data konca", http.StatusBadRequest)
		return
	}
	if name == "" {
		http.Error(w, "nazwa jest wymagana", http.StatusBadRequest)
		return
	}
	if endDate != nil && endDate.Before(startDate) {
		http.Error(w, "data konca musi byc po dacie startu", http.StatusBadRequest)
		return
	}

	currentUser := s.currentUser(r)
	status := leagueStatusForDates(startDate, endDate, time.Now())
	league := model.League{
		ID:          uuid.NewString(),
		Name:        name,
		Description: description,
		Location:    location,
		OwnerID:     currentUser.ID,
		AdminRoles:  map[string]model.LeagueAdminRole{currentUser.ID: model.LeagueAdminPlayer},
		PlayerIDs:   []string{currentUser.ID},
		SetsPerMatch: setsPerMatch,
		StartDate:   startDate,
		EndDate:     endDate,
		Status:      status,
		CreatedAt:   time.Now(),
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
			Title:       league.Name,
			CurrentUser: currentUser,
			Users:       s.store.ListUsers(),
			IsAuthenticated: true,
		},
		League:    league,
		Players:   players,
		Standings: BuildStandings(players, matches),
		Matches:   matchViews,
		SetsRange: setsRange,
		IsAdmin:   isLeagueAdmin(league, currentUser.ID),
		IsPlayer:  isLeaguePlayer(league, currentUser.ID),
		Admins:    s.leagueAdmins(league),
		AdminCandidates: s.leagueAdminCandidates(league),
		Page:       page,
		TotalPages: totalPages,
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
		http.Error(w, "brak uprawnien", http.StatusForbidden)
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
		http.Error(w, "brak uprawnien", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "nieprawidlowe dane", http.StatusBadRequest)
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
		http.Error(w, "brak uprawnien", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "nieprawidlowe dane", http.StatusBadRequest)
		return
	}
	userID := r.FormValue("user_id")
	if userID == "" {
		http.Error(w, "brak gracza", http.StatusBadRequest)
		return
	}
	role := model.LeagueAdminRole(strings.TrimSpace(r.FormValue("role")))
	if role != model.LeagueAdminPlayer && role != model.LeagueAdminModerator {
		http.Error(w, "nieprawidlowa rola", http.StatusBadRequest)
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
		http.Error(w, "brak uprawnien", http.StatusForbidden)
		return
	}
	if adminID == league.OwnerID {
		http.Error(w, "nie mozna zmienic roli wlasciciela", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "nieprawidlowe dane", http.StatusBadRequest)
		return
	}
	role := model.LeagueAdminRole(strings.TrimSpace(r.FormValue("role")))
	if role != model.LeagueAdminPlayer && role != model.LeagueAdminModerator {
		http.Error(w, "nieprawidlowa rola", http.StatusBadRequest)
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
		http.Error(w, "brak uprawnien", http.StatusForbidden)
		return
	}
	if adminID == league.OwnerID {
		http.Error(w, "nie mozna usunac wlasciciela", http.StatusBadRequest)
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
		http.Error(w, "brak uprawnien", http.StatusForbidden)
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
		http.Error(w, "brak uprawnien", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "nieprawidlowe dane", http.StatusBadRequest)
		return
	}
	playerA := r.FormValue("player_a")
	playerB := r.FormValue("player_b")
	if playerA == "" || playerB == "" || playerA == playerB {
		http.Error(w, "wybierz dwoch roznych graczy", http.StatusBadRequest)
		return
	}
	sets := parseSets(r, league.SetsPerMatch)
	if len(sets) == 0 {
		http.Error(w, "uzupelnij wynik", http.StatusBadRequest)
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
		http.Error(w, "nie mozesz potwierdzic wlasnego wyniku", http.StatusForbidden)
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
		http.Error(w, "nie mozesz odrzucic wlasnego wyniku", http.StatusForbidden)
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
		Period:      period,
		CustomMonth: customMonth,
		CustomYear:  customYear,
		MonthOptions: monthOptions(),
		YearOptions:  yearOptions(matches, now.Year()),
		OpponentID: params.OpponentID,
		Opponents:  s.opponentsForUser(currentUser.ID),
		Matches:    []FriendlyMatchView{},
		Page:       page,
		TotalPages: totalPages,
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
