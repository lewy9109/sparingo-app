package web

import (
	"net/http"

	"sqoush-app/internal/store"

	"github.com/go-chi/chi/v5"
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
	r.Post("/leagues/{leagueID}/join-requests/{requestID}/approve", s.handleJoinRequestApprove)
	r.Post("/leagues/{leagueID}/join-requests/{requestID}/reject", s.handleJoinRequestReject)
	r.Post("/leagues/{leagueID}/end", s.handleLeagueEnd)
	r.Post("/leagues/{leagueID}/matches", s.handleMatchCreate)
	r.Post("/matches/{matchID}/confirm", s.handleMatchConfirm)
	r.Post("/matches/{matchID}/reject", s.handleMatchReject)

	return r
}
