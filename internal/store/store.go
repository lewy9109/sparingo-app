package store

import "sqoush-app/internal/model"

type Store interface {
	ListUsers() []model.User
	GetUser(id string) (model.User, bool)
	GetUserByEmail(email string) (model.User, bool)
	CreateUser(user model.User) (model.User, error)

	ListLeagues() []model.League
	GetLeague(id string) (model.League, bool)
	CreateLeague(league model.League) (model.League, error)
	UpdateLeague(league model.League) error
	AddPlayerToLeague(leagueID, userID string) error
	AddAdminToLeague(leagueID, userID string, role model.LeagueAdminRole) error
	UpdateAdminRole(leagueID, userID string, role model.LeagueAdminRole) error
	RemoveAdminFromLeague(leagueID, userID string) error

	ListMatches(leagueID string) []model.Match
	GetMatch(id string) (model.Match, bool)
	CreateMatch(match model.Match) (model.Match, error)
	UpdateMatch(match model.Match) error

	ListFriendlyMatches() []model.FriendlyMatch
	GetFriendlyMatch(id string) (model.FriendlyMatch, bool)
	CreateFriendlyMatch(match model.FriendlyMatch) (model.FriendlyMatch, error)
	UpdateFriendlyMatch(match model.FriendlyMatch) error
}
