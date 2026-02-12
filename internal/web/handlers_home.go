package web

import (
	"math"
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
	flash := flashMessage(r.URL.Query().Get("notice"))
	joinRequests := s.dashboardJoinRequests(currentUser)
	availableLeagues, availablePage, availableTotalPages, availablePages, availableHasPrev, availableHasNext, availablePrevPage, availableNextPage := s.availableLeaguesPage(currentUser, r)

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
		Leagues:             leagues,
		AvailableLeagues:    availableLeagues,
		AvailablePage:       availablePage,
		AvailableTotalPages: availableTotalPages,
		AvailablePages:      availablePages,
		AvailableHasPrev:    availableHasPrev,
		AvailableHasNext:    availableHasNext,
		AvailablePrevPage:   availablePrevPage,
		AvailableNextPage:   availableNextPage,
		JoinRequests:        joinRequests,
		DashboardRecent:     buildDashboardTabView(recentEntries, 10, 6, "/matches/recent", "Brak wyników do wyświetlenia."),
		DashboardPending:    buildDashboardTabView(pendingEntries, 0, 6, "/matches/pending", "Brak meczów do potwierdzenia."),
		DashboardFriendly:   buildDashboardTabView(friendlyEntries, 0, 6, "/friendlies/dashboard", "Brak wyników do wyświetlenia."),
		LeagueSearch:        s.leagueSearchView("", currentUser, 1),
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

func (s *Server) dashboardJoinRequests(currentUser model.User) []JoinRequestDashboardItem {
	if currentUser.ID == "" {
		return nil
	}
	leagues := s.store.ListLeagues()
	items := []JoinRequestDashboardItem{}
	for _, league := range leagues {
		if !canManageLeague(league, currentUser) {
			continue
		}
		requests := s.store.ListJoinRequests(league.ID)
		for _, req := range requests {
			if req.Status != model.JoinRequestPending {
				continue
			}
			user, ok := s.store.GetUser(req.UserID)
			if !ok {
				continue
			}
			items = append(items, JoinRequestDashboardItem{
				League:  league,
				User:    user,
				Request: req,
			})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Request.CreatedAt.After(items[j].Request.CreatedAt)
	})
	return items
}

func (s *Server) availableLeaguesPage(currentUser model.User, r *http.Request) ([]model.League, int, int, []int, bool, bool, int, int) {
	const pageSize = 5
	page := 1
	if v := strings.TrimSpace(r.URL.Query().Get("available_page")); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			page = parsed
		}
	}

	leagues := s.store.ListLeagues()
	filtered := make([]model.League, 0, len(leagues))
	for _, league := range leagues {
		if isLeaguePlayer(league, currentUser.ID) || isLeagueAdmin(league, currentUser.ID) {
			continue
		}
		filtered = append(filtered, league)
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})

	total := len(filtered)
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
	paged := filtered
	if total > 0 {
		paged = filtered[start:end]
	} else {
		paged = []model.League{}
	}

	pages := make([]int, 0, totalPages)
	for i := 1; i <= totalPages; i++ {
		pages = append(pages, i)
	}
	hasPrev := page > 1
	hasNext := page < totalPages
	prevPage := 0
	nextPage := 0
	if hasPrev {
		prevPage = page - 1
	}
	if hasNext {
		nextPage = page + 1
	}
	return paged, page, totalPages, pages, hasPrev, hasNext, prevPage, nextPage
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
