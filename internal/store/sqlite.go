package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"sqoush-app/internal/model"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

type SQLiteOptions struct {
	MigrationsDir string
}

func NewSQLiteStore(path string, opts SQLiteOptions) (*SQLiteStore, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("sqlite path is required")
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	migrationsDir := strings.TrimSpace(opts.MigrationsDir)
	if migrationsDir == "" {
		migrationsDir = "migrations"
	}
	if err := applyMigrations(db, migrationsDir); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) ListUsers() []model.User {
	rows, err := s.db.Query(`SELECT id, first_name, last_name, phone, email, password_hash, role, skill, avatar_url FROM users`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	users := []model.User{}
	for rows.Next() {
		var u model.User
		if err := rows.Scan(&u.ID, &u.FirstName, &u.LastName, &u.Phone, &u.Email, &u.PasswordHash, &u.Role, &u.Skill, &u.AvatarURL); err != nil {
			continue
		}
		users = append(users, u)
	}
	sort.Slice(users, func(i, j int) bool { return users[i].FullName() < users[j].FullName() })
	return users
}

func (s *SQLiteStore) GetUser(id string) (model.User, bool) {
	var u model.User
	err := s.db.QueryRow(`SELECT id, first_name, last_name, phone, email, password_hash, role, skill, avatar_url FROM users WHERE id = ?`, id).
		Scan(&u.ID, &u.FirstName, &u.LastName, &u.Phone, &u.Email, &u.PasswordHash, &u.Role, &u.Skill, &u.AvatarURL)
	if err != nil {
		return model.User{}, false
	}
	return u, true
}

func (s *SQLiteStore) GetUserByEmail(email string) (model.User, bool) {
	var u model.User
	err := s.db.QueryRow(`SELECT id, first_name, last_name, phone, email, password_hash, role, skill, avatar_url FROM users WHERE lower(email) = lower(?) LIMIT 1`, email).
		Scan(&u.ID, &u.FirstName, &u.LastName, &u.Phone, &u.Email, &u.PasswordHash, &u.Role, &u.Skill, &u.AvatarURL)
	if err != nil {
		return model.User{}, false
	}
	return u, true
}

func (s *SQLiteStore) CreateUser(user model.User) (model.User, error) {
	if user.ID == "" {
		user.ID = uuid.NewString()
	}
	if strings.TrimSpace(user.Email) == "" {
		return model.User{}, errors.New("email is required")
	}
	_, err := s.db.Exec(`INSERT INTO users (id, first_name, last_name, phone, email, password_hash, role, skill, avatar_url) VALUES (?,?,?,?,?,?,?,?,?)`,
		user.ID, user.FirstName, user.LastName, user.Phone, user.Email, user.PasswordHash, string(user.Role), string(user.Skill), user.AvatarURL,
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return model.User{}, errors.New("email already exists")
		}
		return model.User{}, err
	}
	return user, nil
}

func (s *SQLiteStore) ListLeagues() []model.League {
	rows, err := s.db.Query(`SELECT id, name, description, location, owner_id, admin_roles, player_ids, sets_per_match, start_date, end_date, status, created_at FROM leagues`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	leagues := []model.League{}
	for rows.Next() {
		league, err := scanSQLiteLeagueRow(rows)
		if err != nil {
			continue
		}
		leagues = append(leagues, league)
	}
	sort.Slice(leagues, func(i, j int) bool { return leagues[i].CreatedAt.After(leagues[j].CreatedAt) })
	return leagues
}

func (s *SQLiteStore) GetLeague(id string) (model.League, bool) {
	row := s.db.QueryRow(`SELECT id, name, description, location, owner_id, admin_roles, player_ids, sets_per_match, start_date, end_date, status, created_at FROM leagues WHERE id = ?`, id)
	league, err := scanSQLiteLeagueRow(row)
	if err != nil {
		return model.League{}, false
	}
	return league, true
}

func (s *SQLiteStore) CreateLeague(league model.League) (model.League, error) {
	if league.ID == "" {
		league.ID = uuid.NewString()
	}
	if league.CreatedAt.IsZero() {
		league.CreatedAt = time.Now()
	}
	if league.AdminRoles == nil {
		league.AdminRoles = map[string]model.LeagueAdminRole{}
	}
	adminJSON := string(toJSON(league.AdminRoles))
	playerJSON := string(toJSON(league.PlayerIDs))

	_, err := s.db.Exec(`INSERT INTO leagues (id, name, description, location, owner_id, admin_roles, player_ids, sets_per_match, start_date, end_date, status, created_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		league.ID, league.Name, league.Description, league.Location, league.OwnerID, adminJSON, playerJSON, league.SetsPerMatch, timeValueString(league.StartDate), timePtrValueString(league.EndDate), string(league.Status), timeValueString(league.CreatedAt),
	)
	if err != nil {
		return model.League{}, err
	}
	return league, nil
}

func (s *SQLiteStore) UpdateLeague(league model.League) error {
	adminJSON := string(toJSON(league.AdminRoles))
	playerJSON := string(toJSON(league.PlayerIDs))

	res, err := s.db.Exec(`UPDATE leagues SET name = ?, description = ?, location = ?, owner_id = ?, admin_roles = ?, player_ids = ?, sets_per_match = ?, start_date = ?, end_date = ?, status = ?, created_at = ? WHERE id = ?`,
		league.Name, league.Description, league.Location, league.OwnerID, adminJSON, playerJSON, league.SetsPerMatch, timeValueString(league.StartDate), timePtrValueString(league.EndDate), string(league.Status), timeValueString(league.CreatedAt), league.ID,
	)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return errors.New("league not found")
	}
	return nil
}

func (s *SQLiteStore) AddPlayerToLeague(leagueID, userID string) error {
	league, ok := s.GetLeague(leagueID)
	if !ok {
		return errors.New("league not found")
	}
	for _, id := range league.PlayerIDs {
		if id == userID {
			return nil
		}
	}
	league.PlayerIDs = append(league.PlayerIDs, userID)
	return s.UpdateLeague(league)
}

func (s *SQLiteStore) AddAdminToLeague(leagueID, userID string, role model.LeagueAdminRole) error {
	league, ok := s.GetLeague(leagueID)
	if !ok {
		return errors.New("league not found")
	}
	if league.AdminRoles == nil {
		league.AdminRoles = map[string]model.LeagueAdminRole{}
	}
	league.AdminRoles[userID] = role
	if role == model.LeagueAdminPlayer {
		if !containsID(league.PlayerIDs, userID) {
			league.PlayerIDs = append(league.PlayerIDs, userID)
		}
	}
	return s.UpdateLeague(league)
}

func (s *SQLiteStore) UpdateAdminRole(leagueID, userID string, role model.LeagueAdminRole) error {
	league, ok := s.GetLeague(leagueID)
	if !ok {
		return errors.New("league not found")
	}
	if league.AdminRoles == nil {
		return errors.New("admin not found")
	}
	if _, exists := league.AdminRoles[userID]; !exists {
		return errors.New("admin not found")
	}
	league.AdminRoles[userID] = role
	if role == model.LeagueAdminPlayer {
		if !containsID(league.PlayerIDs, userID) {
			league.PlayerIDs = append(league.PlayerIDs, userID)
		}
	}
	return s.UpdateLeague(league)
}

func (s *SQLiteStore) RemoveAdminFromLeague(leagueID, userID string) error {
	league, ok := s.GetLeague(leagueID)
	if !ok {
		return errors.New("league not found")
	}
	if league.AdminRoles == nil {
		return errors.New("admin not found")
	}
	if _, exists := league.AdminRoles[userID]; !exists {
		return errors.New("admin not found")
	}
	delete(league.AdminRoles, userID)
	return s.UpdateLeague(league)
}

func (s *SQLiteStore) ListMatches(leagueID string) []model.Match {
	rows, err := s.db.Query(`SELECT id, league_id, player_a_id, player_b_id, sets_json, status, reported_by, confirmed_by, created_at FROM matches WHERE league_id = ?`, leagueID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	matches := []model.Match{}
	for rows.Next() {
		match, err := scanSQLiteMatchRow(rows)
		if err != nil {
			continue
		}
		matches = append(matches, match)
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].CreatedAt.After(matches[j].CreatedAt) })
	return matches
}

func (s *SQLiteStore) GetMatch(id string) (model.Match, bool) {
	row := s.db.QueryRow(`SELECT id, league_id, player_a_id, player_b_id, sets_json, status, reported_by, confirmed_by, created_at FROM matches WHERE id = ?`, id)
	match, err := scanSQLiteMatchRow(row)
	if err != nil {
		return model.Match{}, false
	}
	return match, true
}

func (s *SQLiteStore) CreateMatch(match model.Match) (model.Match, error) {
	if match.ID == "" {
		match.ID = uuid.NewString()
	}
	if match.CreatedAt.IsZero() {
		match.CreatedAt = time.Now()
	}
	setsJSON := string(toJSON(match.Sets))
	_, err := s.db.Exec(`INSERT INTO matches (id, league_id, player_a_id, player_b_id, sets_json, status, reported_by, confirmed_by, created_at) VALUES (?,?,?,?,?,?,?,?,?)`,
		match.ID, match.LeagueID, match.PlayerAID, match.PlayerBID, setsJSON, string(match.Status), match.ReportedBy, match.ConfirmedBy, timeValueString(match.CreatedAt),
	)
	if err != nil {
		return model.Match{}, err
	}
	return match, nil
}

func (s *SQLiteStore) UpdateMatch(match model.Match) error {
	setsJSON := string(toJSON(match.Sets))
	res, err := s.db.Exec(`UPDATE matches SET league_id = ?, player_a_id = ?, player_b_id = ?, sets_json = ?, status = ?, reported_by = ?, confirmed_by = ?, created_at = ? WHERE id = ?`,
		match.LeagueID, match.PlayerAID, match.PlayerBID, setsJSON, string(match.Status), match.ReportedBy, match.ConfirmedBy, timeValueString(match.CreatedAt), match.ID,
	)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return errors.New("match not found")
	}
	return nil
}

func (s *SQLiteStore) ListFriendlyMatches() []model.FriendlyMatch {
	rows, err := s.db.Query(`SELECT id, player_a_id, player_b_id, sets_json, status, reported_by, confirmed_by, played_at, created_at FROM friendly_matches`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	matches := []model.FriendlyMatch{}
	for rows.Next() {
		match, err := scanSQLiteFriendlyMatchRow(rows)
		if err != nil {
			continue
		}
		matches = append(matches, match)
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].PlayedAt.After(matches[j].PlayedAt) })
	return matches
}

func (s *SQLiteStore) GetFriendlyMatch(id string) (model.FriendlyMatch, bool) {
	row := s.db.QueryRow(`SELECT id, player_a_id, player_b_id, sets_json, status, reported_by, confirmed_by, played_at, created_at FROM friendly_matches WHERE id = ?`, id)
	match, err := scanSQLiteFriendlyMatchRow(row)
	if err != nil {
		return model.FriendlyMatch{}, false
	}
	return match, true
}

func (s *SQLiteStore) CreateFriendlyMatch(match model.FriendlyMatch) (model.FriendlyMatch, error) {
	if match.ID == "" {
		match.ID = uuid.NewString()
	}
	if match.CreatedAt.IsZero() {
		match.CreatedAt = time.Now()
	}
	if match.PlayedAt.IsZero() {
		match.PlayedAt = match.CreatedAt
	}
	setsJSON := string(toJSON(match.Sets))
	_, err := s.db.Exec(`INSERT INTO friendly_matches (id, player_a_id, player_b_id, sets_json, status, reported_by, confirmed_by, played_at, created_at) VALUES (?,?,?,?,?,?,?,?,?)`,
		match.ID, match.PlayerAID, match.PlayerBID, setsJSON, string(match.Status), match.ReportedBy, match.ConfirmedBy, timeValueString(match.PlayedAt), timeValueString(match.CreatedAt),
	)
	if err != nil {
		return model.FriendlyMatch{}, err
	}
	return match, nil
}

func (s *SQLiteStore) UpdateFriendlyMatch(match model.FriendlyMatch) error {
	setsJSON := string(toJSON(match.Sets))
	res, err := s.db.Exec(`UPDATE friendly_matches SET player_a_id = ?, player_b_id = ?, sets_json = ?, status = ?, reported_by = ?, confirmed_by = ?, played_at = ?, created_at = ? WHERE id = ?`,
		match.PlayerAID, match.PlayerBID, setsJSON, string(match.Status), match.ReportedBy, match.ConfirmedBy, timeValueString(match.PlayedAt), timeValueString(match.CreatedAt), match.ID,
	)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return errors.New("friendly match not found")
	}
	return nil
}

func scanSQLiteLeagueRow(scanner interface{ Scan(dest ...any) error }) (model.League, error) {
	var league model.League
	var adminJSON, playerJSON sql.NullString
	var startDate, endDate, createdAt sql.NullString
	var status string
	if err := scanner.Scan(
		&league.ID,
		&league.Name,
		&league.Description,
		&league.Location,
		&league.OwnerID,
		&adminJSON,
		&playerJSON,
		&league.SetsPerMatch,
		&startDate,
		&endDate,
		&status,
		&createdAt,
	); err != nil {
		return model.League{}, err
	}
	league.Status = model.LeagueStatus(status)
	if startDate.Valid {
		if parsed, ok := parseTimeString(startDate.String); ok {
			league.StartDate = parsed
		}
	}
	if endDate.Valid {
		if parsed, ok := parseTimeString(endDate.String); ok {
			league.EndDate = &parsed
		}
	}
	if createdAt.Valid {
		if parsed, ok := parseTimeString(createdAt.String); ok {
			league.CreatedAt = parsed
		}
	}
	if adminJSON.Valid && strings.TrimSpace(adminJSON.String) != "" {
		_ = json.Unmarshal([]byte(adminJSON.String), &league.AdminRoles)
	}
	if playerJSON.Valid && strings.TrimSpace(playerJSON.String) != "" {
		_ = json.Unmarshal([]byte(playerJSON.String), &league.PlayerIDs)
	}
	if league.AdminRoles == nil {
		league.AdminRoles = map[string]model.LeagueAdminRole{}
	}
	return league, nil
}

func scanSQLiteMatchRow(scanner interface{ Scan(dest ...any) error }) (model.Match, error) {
	var match model.Match
	var setsJSON sql.NullString
	var createdAt sql.NullString
	var status string
	if err := scanner.Scan(
		&match.ID,
		&match.LeagueID,
		&match.PlayerAID,
		&match.PlayerBID,
		&setsJSON,
		&status,
		&match.ReportedBy,
		&match.ConfirmedBy,
		&createdAt,
	); err != nil {
		return model.Match{}, err
	}
	match.Status = model.MatchStatus(status)
	if createdAt.Valid {
		if parsed, ok := parseTimeString(createdAt.String); ok {
			match.CreatedAt = parsed
		}
	}
	if setsJSON.Valid && strings.TrimSpace(setsJSON.String) != "" {
		_ = json.Unmarshal([]byte(setsJSON.String), &match.Sets)
	}
	return match, nil
}

func scanSQLiteFriendlyMatchRow(scanner interface{ Scan(dest ...any) error }) (model.FriendlyMatch, error) {
	var match model.FriendlyMatch
	var setsJSON sql.NullString
	var playedAt, createdAt sql.NullString
	var status string
	if err := scanner.Scan(
		&match.ID,
		&match.PlayerAID,
		&match.PlayerBID,
		&setsJSON,
		&status,
		&match.ReportedBy,
		&match.ConfirmedBy,
		&playedAt,
		&createdAt,
	); err != nil {
		return model.FriendlyMatch{}, err
	}
	match.Status = model.MatchStatus(status)
	if playedAt.Valid {
		if parsed, ok := parseTimeString(playedAt.String); ok {
			match.PlayedAt = parsed
		}
	}
	if createdAt.Valid {
		if parsed, ok := parseTimeString(createdAt.String); ok {
			match.CreatedAt = parsed
		}
	}
	if setsJSON.Valid && strings.TrimSpace(setsJSON.String) != "" {
		_ = json.Unmarshal([]byte(setsJSON.String), &match.Sets)
	}
	return match, nil
}

func timeValueString(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.Format(time.RFC3339Nano)
}

func timePtrValueString(t *time.Time) any {
	if t == nil || t.IsZero() {
		return nil
	}
	return t.Format(time.RFC3339Nano)
}

func parseTimeString(value string) (time.Time, bool) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, false
	}
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed, true
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed, true
	}
	return time.Time{}, false
}
