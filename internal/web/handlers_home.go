package web

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	"sqoush-app/internal/model"
)

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

// avoid unused import warning for time in this file when build tags change
