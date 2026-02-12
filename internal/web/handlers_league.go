package web

import (
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"sqoush-app/internal/model"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (s *Server) handleLeagueNew(w http.ResponseWriter, r *http.Request) {
	currentUser := s.currentUser(r)
	if currentUser.ID == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	view := BaseView{
		Title:           "Nowa liga",
		CurrentUser:     currentUser,
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
	page := leagueSearchPage(r.URL.Query().Get("page"))
	view := s.leagueSearchView(query, currentUser, page)
	view.Title = "Wyszukaj ligi"
	view.Users = s.store.ListUsers()
	view.IsAuthenticated = currentUser.ID != ""
	view.IsDev = isDevMode()
	view.FlashSuccess = flashMessage(r.URL.Query().Get("notice"))
	if err := s.templates.Render(w, "league_search.html", view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleLeagueSearchResults(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	page := leagueSearchPage(r.URL.Query().Get("page"))
	currentUser := s.currentUser(r)
	view := s.leagueSearchView(query, currentUser, page)
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
	if isLeaguePlayer(league, currentUser.ID) || isLeagueAdmin(league, currentUser.ID) {
		http.Redirect(w, r, "/leagues/"+league.ID, http.StatusSeeOther)
		return
	}
	if s.store.HasPendingJoinRequest(league.ID, currentUser.ID) {
		redirectBack(w, r, "/leagues/"+league.ID, "")
		return
	}
	_, err := s.store.CreateJoinRequest(model.LeagueJoinRequest{
		LeagueID:  league.ID,
		UserID:    currentUser.ID,
		Status:    model.JoinRequestPending,
		CreatedAt: time.Now(),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	redirectBack(w, r, "/leagues/"+league.ID, "join_requested")
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
	if currentUser.ID == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
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
	if _, err := s.store.CreateLeague(league); err != nil {
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
	canManage := canManageLeague(league, currentUser)
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
			FlashSuccess:    flashMessage(r.URL.Query().Get("notice")),
		},
		League:          league,
		Players:         players,
		Standings:       BuildStandings(players, matches),
		Matches:         matchViews,
		SetsRange:       setsRange,
		IsAdmin:         canManage,
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
			CanManage:  canManage,
		},
	}
	if currentUser.ID != "" && !view.IsAdmin && !view.IsPlayer {
		view.PendingJoin = s.store.HasPendingJoinRequest(league.ID, currentUser.ID)
	}
	if canManage {
		view.JoinRequests = s.leagueJoinRequestViews(league)
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
	if !canManageLeague(league, currentUser) {
		http.Error(w, "brak uprawnień", http.StatusForbidden)
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	view := s.playerSearchView(league, currentUser, query, false)
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
	if !canManageLeague(league, currentUser) {
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
		view := s.playerSearchView(updatedLeague, currentUser, query, true)
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
	if !canManageLeague(league, currentUser) {
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
	adminID := chi.URLParam(r, "adminID")
	leagueID := chi.URLParam(r, "leagueID")
	league, ok := s.store.GetLeague(leagueID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	currentUser := s.currentUser(r)
	if !canManageLeague(league, currentUser) {
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
	adminID := chi.URLParam(r, "adminID")
	leagueID := chi.URLParam(r, "leagueID")
	league, ok := s.store.GetLeague(leagueID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	currentUser := s.currentUser(r)
	if !canManageLeague(league, currentUser) {
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

func (s *Server) handleJoinRequestApprove(w http.ResponseWriter, r *http.Request) {
	leagueID := chi.URLParam(r, "leagueID")
	requestID := chi.URLParam(r, "requestID")
	league, ok := s.store.GetLeague(leagueID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	currentUser := s.currentUser(r)
	if !canManageLeague(league, currentUser) {
		http.Error(w, "brak uprawnień", http.StatusForbidden)
		return
	}
	req, ok := s.store.GetJoinRequest(requestID)
	if !ok || req.LeagueID != league.ID {
		http.NotFound(w, r)
		return
	}
	if req.Status != model.JoinRequestPending {
		http.Error(w, "prośba została już rozpatrzona", http.StatusBadRequest)
		return
	}
	if !isLeaguePlayer(league, req.UserID) && !isLeagueAdmin(league, req.UserID) {
		if err := s.store.AddPlayerToLeague(league.ID, req.UserID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	now := time.Now()
	req.Status = model.JoinRequestApproved
	req.DecidedBy = currentUser.ID
	req.DecidedAt = &now
	if err := s.store.UpdateJoinRequest(req); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/leagues/"+league.ID, http.StatusSeeOther)
}

func (s *Server) handleJoinRequestReject(w http.ResponseWriter, r *http.Request) {
	leagueID := chi.URLParam(r, "leagueID")
	requestID := chi.URLParam(r, "requestID")
	league, ok := s.store.GetLeague(leagueID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	currentUser := s.currentUser(r)
	if !canManageLeague(league, currentUser) {
		http.Error(w, "brak uprawnień", http.StatusForbidden)
		return
	}
	req, ok := s.store.GetJoinRequest(requestID)
	if !ok || req.LeagueID != league.ID {
		http.NotFound(w, r)
		return
	}
	if req.Status != model.JoinRequestPending {
		http.Error(w, "prośba została już rozpatrzona", http.StatusBadRequest)
		return
	}
	now := time.Now()
	req.Status = model.JoinRequestRejected
	req.DecidedBy = currentUser.ID
	req.DecidedAt = &now
	if err := s.store.UpdateJoinRequest(req); err != nil {
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
	if !canManageLeague(league, currentUser) {
		http.Error(w, "brak uprawnień", http.StatusForbidden)
		return
	}
	if league.Status == model.LeagueStatusFinished {
		http.Redirect(w, r, "/leagues/"+league.ID, http.StatusSeeOther)
		return
	}
	now := time.Now()
	league.Status = model.LeagueStatusFinished
	league.EndDate = &now
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
	if !canManageLeague(league, currentUser) && !isLeaguePlayer(league, currentUser.ID) {
		http.Error(w, "brak uprawnień", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "nieprawidłowe dane", http.StatusBadRequest)
		return
	}
	playerA := r.FormValue("player_a_id")
	playerB := r.FormValue("player_b_id")
	if playerA == "" {
		playerA = r.FormValue("player_a")
	}
	if playerB == "" {
		playerB = r.FormValue("player_b")
	}
	if playerA == "" || playerB == "" || playerA == playerB {
		http.Error(w, "wybierz dwóch różnych graczy", http.StatusBadRequest)
		return
	}
	if !canManageLeague(league, currentUser) {
		if playerA != currentUser.ID {
			http.Error(w, "gracz A musi być tobą", http.StatusBadRequest)
			return
		}
		if playerB == currentUser.ID {
			http.Error(w, "wybierz innego gracza jako przeciwnika", http.StatusBadRequest)
			return
		}
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
	if _, err := s.store.CreateMatch(match); err != nil {
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

func (s *Server) leagueJoinRequestViews(league model.League) []LeagueJoinRequestView {
	requests := s.store.ListJoinRequests(league.ID)
	views := []LeagueJoinRequestView{}
	for _, req := range requests {
		if req.Status != model.JoinRequestPending {
			continue
		}
		user, ok := s.store.GetUser(req.UserID)
		if !ok {
			continue
		}
		views = append(views, LeagueJoinRequestView{
			Request: req,
			User:    user,
		})
	}
	return views
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
	leagues := s.store.ListLeagues()
	if query == "" {
		return leagues
	}
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

func (s *Server) leagueSearchView(query string, currentUser model.User, page int) LeagueSearchView {
	const pageSize = 20
	results := s.searchLeagues(query)
	total := len(results)
	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))
	if totalPages == 0 {
		totalPages = 1
	}
	if page > totalPages {
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
	paged := results
	if total > 0 {
		paged = results[start:end]
	} else {
		paged = []model.League{}
	}

	items := make([]LeagueSearchResultView, 0, len(paged))
	for _, league := range paged {
		inLeague := isLeaguePlayer(league, currentUser.ID) || isLeagueAdmin(league, currentUser.ID)
		pending := false
		if currentUser.ID != "" && !inLeague {
			pending = s.store.HasPendingJoinRequest(league.ID, currentUser.ID)
		}
		items = append(items, LeagueSearchResultView{
			League:   league,
			InLeague: inLeague,
			Pending:  pending,
		})
	}
	return LeagueSearchView{
		BaseView: BaseView{
			CurrentUser: currentUser,
		},
		Query:      query,
		Results:    items,
		EmptyQuery: query == "",
		Page:       page,
		TotalPages: totalPages,
		Pages:      leagueSearchPages(totalPages),
		HasPrev:    page > 1,
		HasNext:    page < totalPages,
		PrevPage:   page - 1,
		NextPage:   page + 1,
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

func (s *Server) playerSearchView(league model.League, currentUser model.User, query string, includePanel bool) PlayerSearchView {
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
		CanManage:  canManageLeague(league, currentUser),
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

func leagueSearchPage(raw string) int {
	page, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || page < 1 {
		return 1
	}
	return page
}

func leagueSearchPages(totalPages int) []int {
	pages := make([]int, 0, totalPages)
	for i := 1; i <= totalPages; i++ {
		pages = append(pages, i)
	}
	return pages
}

func canManageLeague(league model.League, user model.User) bool {
	return isSuperAdmin(user) || isLeagueAdmin(league, user.ID)
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

func redirectBack(w http.ResponseWriter, r *http.Request, fallback string, notice string) {
	ref := strings.TrimSpace(r.Referer())
	if ref != "" {
		if notice == "" {
			http.Redirect(w, r, ref, http.StatusSeeOther)
			return
		}
		if u, err := url.Parse(ref); err == nil {
			q := u.Query()
			q.Set("notice", notice)
			u.RawQuery = q.Encode()
			http.Redirect(w, r, u.String(), http.StatusSeeOther)
			return
		}
	}
	if notice == "" {
		http.Redirect(w, r, fallback, http.StatusSeeOther)
		return
	}
	if u, err := url.Parse(fallback); err == nil {
		q := u.Query()
		q.Set("notice", notice)
		u.RawQuery = q.Encode()
		http.Redirect(w, r, u.String(), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, fallback, http.StatusSeeOther)
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
