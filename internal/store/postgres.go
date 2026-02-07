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
	_ "github.com/jackc/pgx/v5/stdlib"
)

type PostgresStore struct {
	db *sql.DB
}

type PostgresOptions struct {
	MigrationsDir string
}

func NewPostgresStore(dsn string, opts PostgresOptions) (*PostgresStore, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, errors.New("postgres dsn is required")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	migrationsDir := strings.TrimSpace(opts.MigrationsDir)
	if migrationsDir == "" {
		migrationsDir = "migrations/postgres"
	}
	if err := applyMigrations(db, migrationsDir); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &PostgresStore{db: db}, nil
}

func (s *PostgresStore) ListUsers() []model.User {
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

func (s *PostgresStore) GetUser(id string) (model.User, bool) {
	var u model.User
	err := s.db.QueryRow(`SELECT id, first_name, last_name, phone, email, password_hash, role, skill, avatar_url FROM users WHERE id = $1`, id).
		Scan(&u.ID, &u.FirstName, &u.LastName, &u.Phone, &u.Email, &u.PasswordHash, &u.Role, &u.Skill, &u.AvatarURL)
	if err != nil {
		return model.User{}, false
	}
	return u, true
}

func (s *PostgresStore) GetUserByEmail(email string) (model.User, bool) {
	var u model.User
	err := s.db.QueryRow(`SELECT id, first_name, last_name, phone, email, password_hash, role, skill, avatar_url FROM users WHERE lower(email) = lower($1) LIMIT 1`, email).
		Scan(&u.ID, &u.FirstName, &u.LastName, &u.Phone, &u.Email, &u.PasswordHash, &u.Role, &u.Skill, &u.AvatarURL)
	if err != nil {
		return model.User{}, false
	}
	return u, true
}

func (s *PostgresStore) CreateUser(user model.User) (model.User, error) {
	if user.ID == "" {
		user.ID = uuid.NewString()
	}
	if strings.TrimSpace(user.Email) == "" {
		return model.User{}, errors.New("email is required")
	}
	_, err := s.db.Exec(`INSERT INTO users (id, first_name, last_name, phone, email, password_hash, role, skill, avatar_url) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
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

func (s *PostgresStore) ListLeagues() []model.League {
	rows, err := s.db.Query(`SELECT id, name, description, location, owner_id, admin_roles, player_ids, sets_per_match, start_date, end_date, status, created_at FROM leagues`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	leagues := []model.League{}
	for rows.Next() {
		league, err := scanLeagueRow(rows)
		if err != nil {
			continue
		}
		leagues = append(leagues, league)
	}
	sort.Slice(leagues, func(i, j int) bool { return leagues[i].CreatedAt.After(leagues[j].CreatedAt) })
	return leagues
}

func (s *PostgresStore) GetLeague(id string) (model.League, bool) {
	row := s.db.QueryRow(`SELECT id, name, description, location, owner_id, admin_roles, player_ids, sets_per_match, start_date, end_date, status, created_at FROM leagues WHERE id = $1`, id)
	league, err := scanLeagueRow(row)
	if err != nil {
		return model.League{}, false
	}
	return league, true
}

func (s *PostgresStore) CreateLeague(league model.League) (model.League, error) {
	if league.ID == "" {
		league.ID = uuid.NewString()
	}
	if league.CreatedAt.IsZero() {
		league.CreatedAt = time.Now()
	}
	if league.AdminRoles == nil {
		league.AdminRoles = map[string]model.LeagueAdminRole{}
	}
	adminJSON := toJSON(league.AdminRoles)
	playerJSON := toJSON(league.PlayerIDs)

	_, err := s.db.Exec(`INSERT INTO leagues (id, name, description, location, owner_id, admin_roles, player_ids, sets_per_match, start_date, end_date, status, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		league.ID, league.Name, league.Description, league.Location, league.OwnerID, adminJSON, playerJSON, league.SetsPerMatch, timeValuePtr(league.StartDate), timePtrValue(league.EndDate), string(league.Status), timeValuePtr(league.CreatedAt),
	)
	if err != nil {
		return model.League{}, err
	}
	return league, nil
}

func (s *PostgresStore) UpdateLeague(league model.League) error {
	adminJSON := toJSON(league.AdminRoles)
	playerJSON := toJSON(league.PlayerIDs)

	res, err := s.db.Exec(`UPDATE leagues SET name = $1, description = $2, location = $3, owner_id = $4, admin_roles = $5, player_ids = $6, sets_per_match = $7, start_date = $8, end_date = $9, status = $10, created_at = $11 WHERE id = $12`,
		league.Name, league.Description, league.Location, league.OwnerID, adminJSON, playerJSON, league.SetsPerMatch, timeValuePtr(league.StartDate), timePtrValue(league.EndDate), string(league.Status), timeValuePtr(league.CreatedAt), league.ID,
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

func (s *PostgresStore) AddPlayerToLeague(leagueID, userID string) error {
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

func (s *PostgresStore) AddAdminToLeague(leagueID, userID string, role model.LeagueAdminRole) error {
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

func (s *PostgresStore) UpdateAdminRole(leagueID, userID string, role model.LeagueAdminRole) error {
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

func (s *PostgresStore) RemoveAdminFromLeague(leagueID, userID string) error {
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

func (s *PostgresStore) ListMatches(leagueID string) []model.Match {
	rows, err := s.db.Query(`SELECT id, league_id, player_a_id, player_b_id, sets_json, status, reported_by, confirmed_by, created_at FROM matches WHERE league_id = $1`, leagueID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	matches := []model.Match{}
	for rows.Next() {
		match, err := scanMatchRow(rows)
		if err != nil {
			continue
		}
		matches = append(matches, match)
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].CreatedAt.After(matches[j].CreatedAt) })
	return matches
}

func (s *PostgresStore) GetMatch(id string) (model.Match, bool) {
	row := s.db.QueryRow(`SELECT id, league_id, player_a_id, player_b_id, sets_json, status, reported_by, confirmed_by, created_at FROM matches WHERE id = $1`, id)
	match, err := scanMatchRow(row)
	if err != nil {
		return model.Match{}, false
	}
	return match, true
}

func (s *PostgresStore) CreateMatch(match model.Match) (model.Match, error) {
	if match.ID == "" {
		match.ID = uuid.NewString()
	}
	if match.CreatedAt.IsZero() {
		match.CreatedAt = time.Now()
	}
	setsJSON := toJSON(match.Sets)
	_, err := s.db.Exec(`INSERT INTO matches (id, league_id, player_a_id, player_b_id, sets_json, status, reported_by, confirmed_by, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		match.ID, match.LeagueID, match.PlayerAID, match.PlayerBID, setsJSON, string(match.Status), match.ReportedBy, match.ConfirmedBy, timeValuePtr(match.CreatedAt),
	)
	if err != nil {
		return model.Match{}, err
	}
	return match, nil
}

func (s *PostgresStore) UpdateMatch(match model.Match) error {
	setsJSON := toJSON(match.Sets)
	res, err := s.db.Exec(`UPDATE matches SET league_id = $1, player_a_id = $2, player_b_id = $3, sets_json = $4, status = $5, reported_by = $6, confirmed_by = $7, created_at = $8 WHERE id = $9`,
		match.LeagueID, match.PlayerAID, match.PlayerBID, setsJSON, string(match.Status), match.ReportedBy, match.ConfirmedBy, timeValuePtr(match.CreatedAt), match.ID,
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

func (s *PostgresStore) ListFriendlyMatches() []model.FriendlyMatch {
	rows, err := s.db.Query(`SELECT id, player_a_id, player_b_id, sets_json, status, reported_by, confirmed_by, played_at, created_at FROM friendly_matches`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	matches := []model.FriendlyMatch{}
	for rows.Next() {
		match, err := scanFriendlyMatchRow(rows)
		if err != nil {
			continue
		}
		matches = append(matches, match)
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].PlayedAt.After(matches[j].PlayedAt) })
	return matches
}

func (s *PostgresStore) GetFriendlyMatch(id string) (model.FriendlyMatch, bool) {
	row := s.db.QueryRow(`SELECT id, player_a_id, player_b_id, sets_json, status, reported_by, confirmed_by, played_at, created_at FROM friendly_matches WHERE id = $1`, id)
	match, err := scanFriendlyMatchRow(row)
	if err != nil {
		return model.FriendlyMatch{}, false
	}
	return match, true
}

func (s *PostgresStore) CreateFriendlyMatch(match model.FriendlyMatch) (model.FriendlyMatch, error) {
	if match.ID == "" {
		match.ID = uuid.NewString()
	}
	if match.CreatedAt.IsZero() {
		match.CreatedAt = time.Now()
	}
	if match.PlayedAt.IsZero() {
		match.PlayedAt = match.CreatedAt
	}
	setsJSON := toJSON(match.Sets)
	_, err := s.db.Exec(`INSERT INTO friendly_matches (id, player_a_id, player_b_id, sets_json, status, reported_by, confirmed_by, played_at, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		match.ID, match.PlayerAID, match.PlayerBID, setsJSON, string(match.Status), match.ReportedBy, match.ConfirmedBy, timeValuePtr(match.PlayedAt), timeValuePtr(match.CreatedAt),
	)
	if err != nil {
		return model.FriendlyMatch{}, err
	}
	return match, nil
}

func (s *PostgresStore) UpdateFriendlyMatch(match model.FriendlyMatch) error {
	setsJSON := toJSON(match.Sets)
	res, err := s.db.Exec(`UPDATE friendly_matches SET player_a_id = $1, player_b_id = $2, sets_json = $3, status = $4, reported_by = $5, confirmed_by = $6, played_at = $7, created_at = $8 WHERE id = $9`,
		match.PlayerAID, match.PlayerBID, setsJSON, string(match.Status), match.ReportedBy, match.ConfirmedBy, timeValuePtr(match.PlayedAt), timeValuePtr(match.CreatedAt), match.ID,
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

func scanLeagueRow(scanner interface{ Scan(dest ...any) error }) (model.League, error) {
	var league model.League
	var adminJSON, playerJSON []byte
	var startDate, endDate, createdAt sql.NullTime
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
		league.StartDate = startDate.Time
	}
	if endDate.Valid {
		parsed := endDate.Time
		league.EndDate = &parsed
	}
	if createdAt.Valid {
		league.CreatedAt = createdAt.Time
	}
	if len(adminJSON) > 0 {
		_ = json.Unmarshal(adminJSON, &league.AdminRoles)
	}
	if len(playerJSON) > 0 {
		_ = json.Unmarshal(playerJSON, &league.PlayerIDs)
	}
	if league.AdminRoles == nil {
		league.AdminRoles = map[string]model.LeagueAdminRole{}
	}
	return league, nil
}

func scanMatchRow(scanner interface{ Scan(dest ...any) error }) (model.Match, error) {
	var match model.Match
	var setsJSON []byte
	var createdAt sql.NullTime
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
		match.CreatedAt = createdAt.Time
	}
	if len(setsJSON) > 0 {
		_ = json.Unmarshal(setsJSON, &match.Sets)
	}
	return match, nil
}

func scanFriendlyMatchRow(scanner interface{ Scan(dest ...any) error }) (model.FriendlyMatch, error) {
	var match model.FriendlyMatch
	var setsJSON []byte
	var playedAt, createdAt sql.NullTime
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
		match.PlayedAt = playedAt.Time
	}
	if createdAt.Valid {
		match.CreatedAt = createdAt.Time
	}
	if len(setsJSON) > 0 {
		_ = json.Unmarshal(setsJSON, &match.Sets)
	}
	return match, nil
}

func timeValuePtr(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}

func timePtrValue(t *time.Time) any {
	if t == nil || t.IsZero() {
		return nil
	}
	return *t
}

func toJSON(v any) []byte {
	if v == nil {
		return []byte("null")
	}
	data, err := json.Marshal(v)
	if err != nil {
		return []byte("null")
	}
	return data
}
