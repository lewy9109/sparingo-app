package store

import (
	"errors"
	"math/rand"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"sqoush-app/internal/model"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type MemoryStore struct {
	mu         sync.RWMutex
	users      map[string]model.User
	leagues    map[string]model.League
	matches    map[string]model.Match
	friendlies map[string]model.FriendlyMatch
	reports    map[string]model.Report
}

func NewMemoryStore() *MemoryStore {
	s := &MemoryStore{
		users:      make(map[string]model.User),
		leagues:    make(map[string]model.League),
		matches:    make(map[string]model.Match),
		friendlies: make(map[string]model.FriendlyMatch),
		reports:    make(map[string]model.Report),
	}
	if strings.ToLower(strings.TrimSpace(os.Getenv("APP"))) != "prod" {
		seedData(s)
	}

	return s
}

func (s *MemoryStore) ListUsers() []model.User {
	s.mu.RLock()
	defer s.mu.RUnlock()

	users := make([]model.User, 0, len(s.users))
	for _, u := range s.users {
		users = append(users, u)
	}
	sort.Slice(users, func(i, j int) bool { return users[i].FullName() < users[j].FullName() })
	return users
}

func (s *MemoryStore) GetUser(id string) (model.User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	u, ok := s.users[id]
	return u, ok
}

func (s *MemoryStore) GetUserByEmail(email string) (model.User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, u := range s.users {
		if strings.EqualFold(u.Email, email) {
			return u, true
		}
	}
	return model.User{}, false
}

func (s *MemoryStore) CreateUser(user model.User) (model.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if user.ID == "" {
		user.ID = uuid.NewString()
	}
	if user.Email == "" {
		return model.User{}, errors.New("email is required")
	}
	if user.Role == "" {
		user.Role = model.RoleUser
	}
	for _, u := range s.users {
		if strings.EqualFold(u.Email, user.Email) {
			return model.User{}, errors.New("email already exists")
		}
	}
	s.users[user.ID] = user
	return user, nil
}

func (s *MemoryStore) ListLeagues() []model.League {
	s.mu.RLock()
	defer s.mu.RUnlock()

	leagues := make([]model.League, 0, len(s.leagues))
	for _, l := range s.leagues {
		leagues = append(leagues, l)
	}
	sort.Slice(leagues, func(i, j int) bool { return leagues[i].CreatedAt.After(leagues[j].CreatedAt) })
	return leagues
}

func (s *MemoryStore) GetLeague(id string) (model.League, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	l, ok := s.leagues[id]
	return l, ok
}

func (s *MemoryStore) CreateLeague(league model.League) (model.League, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if league.ID == "" {
		league.ID = uuid.NewString()
	}
	if league.CreatedAt.IsZero() {
		league.CreatedAt = time.Now()
	}
	s.leagues[league.ID] = league
	return league, nil
}

func (s *MemoryStore) UpdateLeague(league model.League) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.leagues[league.ID]; !ok {
		return errors.New("league not found")
	}
	s.leagues[league.ID] = league
	return nil
}

func (s *MemoryStore) AddPlayerToLeague(leagueID, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	league, ok := s.leagues[leagueID]
	if !ok {
		return errors.New("league not found")
	}
	for _, id := range league.PlayerIDs {
		if id == userID {
			return nil
		}
	}
	league.PlayerIDs = append(league.PlayerIDs, userID)
	s.leagues[leagueID] = league
	return nil
}

func (s *MemoryStore) AddAdminToLeague(leagueID, userID string, role model.LeagueAdminRole) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	league, ok := s.leagues[leagueID]
	if !ok {
		return errors.New("league not found")
	}
	if league.AdminRoles == nil {
		league.AdminRoles = map[string]model.LeagueAdminRole{}
	}
	league.AdminRoles[userID] = role
	if role == model.LeagueAdminPlayer {
		alreadyPlayer := false
		for _, id := range league.PlayerIDs {
			if id == userID {
				alreadyPlayer = true
				break
			}
		}
		if !alreadyPlayer {
			league.PlayerIDs = append(league.PlayerIDs, userID)
		}
	}
	s.leagues[leagueID] = league
	return nil
}

func (s *MemoryStore) UpdateAdminRole(leagueID, userID string, role model.LeagueAdminRole) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	league, ok := s.leagues[leagueID]
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
	s.leagues[leagueID] = league
	return nil
}

func (s *MemoryStore) RemoveAdminFromLeague(leagueID, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	league, ok := s.leagues[leagueID]
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
	s.leagues[leagueID] = league
	return nil
}

func (s *MemoryStore) ListMatches(leagueID string) []model.Match {
	s.mu.RLock()
	defer s.mu.RUnlock()

	matches := make([]model.Match, 0)
	for _, m := range s.matches {
		if m.LeagueID == leagueID {
			matches = append(matches, m)
		}
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].CreatedAt.After(matches[j].CreatedAt) })
	return matches
}

func (s *MemoryStore) GetMatch(id string) (model.Match, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	m, ok := s.matches[id]
	return m, ok
}

func (s *MemoryStore) CreateMatch(match model.Match) (model.Match, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if match.ID == "" {
		match.ID = uuid.NewString()
	}
	if match.CreatedAt.IsZero() {
		match.CreatedAt = time.Now()
	}
	s.matches[match.ID] = match
	return match, nil
}

func (s *MemoryStore) UpdateMatch(match model.Match) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.matches[match.ID]; !ok {
		return errors.New("match not found")
	}
	s.matches[match.ID] = match
	return nil
}

func (s *MemoryStore) ListFriendlyMatches() []model.FriendlyMatch {
	s.mu.RLock()
	defer s.mu.RUnlock()

	matches := make([]model.FriendlyMatch, 0, len(s.friendlies))
	for _, m := range s.friendlies {
		matches = append(matches, m)
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].PlayedAt.After(matches[j].PlayedAt) })
	return matches
}

func (s *MemoryStore) GetFriendlyMatch(id string) (model.FriendlyMatch, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	m, ok := s.friendlies[id]
	return m, ok
}

func (s *MemoryStore) CreateFriendlyMatch(match model.FriendlyMatch) (model.FriendlyMatch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if match.ID == "" {
		match.ID = uuid.NewString()
	}
	if match.CreatedAt.IsZero() {
		match.CreatedAt = time.Now()
	}
	if match.PlayedAt.IsZero() {
		match.PlayedAt = match.CreatedAt
	}
	s.friendlies[match.ID] = match
	return match, nil
}

func (s *MemoryStore) UpdateFriendlyMatch(match model.FriendlyMatch) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.friendlies[match.ID]; !ok {
		return errors.New("friendly match not found")
	}
	s.friendlies[match.ID] = match
	return nil
}

func (s *MemoryStore) ListReports() []model.Report {
	s.mu.RLock()
	defer s.mu.RUnlock()

	reports := make([]model.Report, 0, len(s.reports))
	for _, r := range s.reports {
		reports = append(reports, r)
	}
	sort.Slice(reports, func(i, j int) bool { return reports[i].CreatedAt.After(reports[j].CreatedAt) })
	return reports
}

func (s *MemoryStore) CreateReport(report model.Report) (model.Report, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if report.ID == "" {
		report.ID = uuid.NewString()
	}
	if report.CreatedAt.IsZero() {
		report.CreatedAt = time.Now()
	}
	if report.Status == "" {
		report.Status = model.ReportOpen
	}
	s.reports[report.ID] = report
	return report, nil
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

func seedData(s *MemoryStore) {
	rng := rand.New(rand.NewSource(42))
	defaultHash := hashPassword("password123")

	seedUsers := []model.User{
		{ID: uuid.NewString(), FirstName: "Krystian", LastName: "Lewandowski", Email: "lewy9109@gmail.com", Skill: model.SkillIntermediate, AvatarURL: "https://i.pravatar.cc/100?img=12"},
		{ID: uuid.NewString(), FirstName: "Lewy", LastName: "Nowy", Email: "lewy9109+1@gmail.com", Skill: model.SkillBeginner, AvatarURL: "https://i.pravatar.cc/100?img=61"},
		{ID: uuid.NewString(), FirstName: "Paweł", LastName: "Góra", Email: "pawel.gora@example.com", Skill: model.SkillBeginner, AvatarURL: "https://i.pravatar.cc/100?img=32"},
		{ID: uuid.NewString(), FirstName: "Jacek", LastName: "Nowak", Email: "jacek.nowak@example.com", Skill: model.SkillBeginner, AvatarURL: "https://i.pravatar.cc/100?img=33"},
		{ID: uuid.NewString(), FirstName: "Tomek", LastName: "Zieliński", Email: "tomek.zielinski@example.com", Skill: model.SkillBeginner, AvatarURL: "https://i.pravatar.cc/100?img=34"},
		{ID: uuid.NewString(), FirstName: "Władek", LastName: "Kowal", Email: "wladek.kowal@example.com", Skill: model.SkillBeginner, AvatarURL: "https://i.pravatar.cc/100?img=35"},
		{ID: uuid.NewString(), FirstName: "Damian", LastName: "Lis", Email: "damian.lis@example.com", Skill: model.SkillBeginner, AvatarURL: "https://i.pravatar.cc/100?img=36"},
		{ID: uuid.NewString(), FirstName: "Aneta", LastName: "Zalewska", Email: "aneta.zalewska@example.com", Skill: model.SkillIntermediate, AvatarURL: "https://i.pravatar.cc/100?img=47"},
		{ID: uuid.NewString(), FirstName: "Marek", LastName: "Król", Email: "marek.krol@example.com", Skill: model.SkillIntermediate, AvatarURL: "https://i.pravatar.cc/100?img=48"},
		{ID: uuid.NewString(), FirstName: "Kasia", LastName: "Wrona", Email: "kasia.wrona@example.com", Skill: model.SkillBeginner, AvatarURL: "https://i.pravatar.cc/100?img=49"},
		{ID: uuid.NewString(), FirstName: "Ola", LastName: "Chmiel", Email: "ola.chmiel@example.com", Skill: model.SkillPro, AvatarURL: "https://i.pravatar.cc/100?img=50"},
		{ID: uuid.NewString(), FirstName: "Piotr", LastName: "Maj", Email: "piotr.maj@example.com", Skill: model.SkillIntermediate, AvatarURL: "https://i.pravatar.cc/100?img=51"},
		{ID: uuid.NewString(), FirstName: "Lena", LastName: "Jankowska", Email: "lena.jankowska@example.com", Skill: model.SkillBeginner, AvatarURL: "https://i.pravatar.cc/100?img=52"},
		{ID: uuid.NewString(), FirstName: "Bartek", LastName: "Nowicki", Email: "bartek.nowicki@example.com", Skill: model.SkillBeginner, AvatarURL: "https://i.pravatar.cc/100?img=53"},
		{ID: uuid.NewString(), FirstName: "Rafał", LastName: "Olszewski", Email: "rafal.olszewski@example.com", Skill: model.SkillIntermediate, AvatarURL: "https://i.pravatar.cc/100?img=54"},
		{ID: uuid.NewString(), FirstName: "Ewa", LastName: "Kania", Email: "ewa.kania@example.com", Skill: model.SkillIntermediate, AvatarURL: "https://i.pravatar.cc/100?img=55"},
		{ID: uuid.NewString(), FirstName: "Krzysztof", LastName: "Małek", Email: "krzysztof.malek@example.com", Skill: model.SkillBeginner, AvatarURL: "https://i.pravatar.cc/100?img=56"},
		{ID: uuid.NewString(), FirstName: "Monika", LastName: "Woźniak", Email: "monika.wozniak@example.com", Skill: model.SkillBeginner, AvatarURL: "https://i.pravatar.cc/100?img=57"},
		{ID: uuid.NewString(), FirstName: "Tomasz", LastName: "Jura", Email: "tomasz.jura@example.com", Skill: model.SkillIntermediate, AvatarURL: "https://i.pravatar.cc/100?img=58"},
		{ID: uuid.NewString(), FirstName: "Natalia", LastName: "Kruk", Email: "natalia.kruk@example.com", Skill: model.SkillPro, AvatarURL: "https://i.pravatar.cc/100?img=59"},
	}

	for i := range seedUsers {
		seedUsers[i].PasswordHash = defaultHash
		if strings.EqualFold(seedUsers[i].Email, "lewy9109@gmail.com") {
			seedUsers[i].Role = model.RoleSuperAdmin
		} else {
			seedUsers[i].Role = model.RoleUser
		}
		s.users[seedUsers[i].ID] = seedUsers[i]
	}

	seedUsersForActivity := filterUsersByEmail(seedUsers, "lewy9109+1@gmail.com")

	leagueDefs := []struct {
		Name, Description, Location string
		Sets                        int
	}{
		{"Liga Warszawska", "Liga miejska dla graczy z całej Warszawy.", "Warsaw Squash Center", 5},
		{"Liga Klubowa", "Rozgrywki klubowe dla stałych bywalców.", "Squash Arena", 5},
		{"Liga Weekendowa", "Spotkania weekendowe dla znajomych.", "City Squash Hub", 3},
		{"Liga Pro", "Mecze dla zaawansowanych zawodników.", "ProSquash Hall", 5},
	}

	leagues := make([]model.League, 0, len(leagueDefs))
	for i, ln := range leagueDefs {
		owner := seedUsersForActivity[i%len(seedUsersForActivity)]
		playerIDs := pickUserIDs(seedUsersForActivity, rng, 10+rng.Intn(6))
		startDate := time.Now().AddDate(0, 0, -30+rng.Intn(90))
		var endDate *time.Time
		if rng.Intn(4) == 0 {
			end := startDate.AddDate(0, 0, 30+rng.Intn(120))
			endDate = &end
		}
		league := model.League{
			ID:           uuid.NewString(),
			Name:         ln.Name,
			Description:  ln.Description,
			Location:     ln.Location,
			OwnerID:      owner.ID,
			AdminRoles:   map[string]model.LeagueAdminRole{owner.ID: model.LeagueAdminPlayer},
			PlayerIDs:    playerIDs,
			SetsPerMatch: ln.Sets,
			StartDate:    startDate,
			EndDate:      endDate,
			Status:       leagueStatusForSeed(startDate, endDate, time.Now()),
			CreatedAt:    time.Now().AddDate(0, 0, -rng.Intn(120)),
		}
		if !containsID(playerIDs, owner.ID) {
			league.PlayerIDs = append(league.PlayerIDs, owner.ID)
		}
		s.leagues[league.ID] = league
		leagues = append(leagues, league)
	}

	currentYear := time.Now().Year()
	previousYear := currentYear - 1

	if lewy, ok := findUserByEmail(seedUsers, "lewy9109@gmail.com"); ok {
		statuses := []model.MatchStatus{
			model.MatchConfirmed,
			model.MatchPending,
			model.MatchRejected,
		}
		for i := range leagues {
			league := &leagues[i]
			ensureUserInLeague(s, league, lewy.ID)
			matchCount := 3 + rng.Intn(4)
			for j := 0; j < matchCount; j++ {
				opponentID := pickOpponentExcluding(league.PlayerIDs, lewy.ID, rng)
				if opponentID == "" {
					continue
				}
				playedAt := time.Now().AddDate(0, 0, -rng.Intn(60))
				status := statuses[rng.Intn(len(statuses))]
				sets := randomSets(rng, league.SetsPerMatch)
				match := model.Match{
					ID:         uuid.NewString(),
					LeagueID:   league.ID,
					PlayerAID:  lewy.ID,
					PlayerBID:  opponentID,
					Sets:       sets,
					Status:     status,
					ReportedBy: lewy.ID,
					CreatedAt:  playedAt,
				}
				if status == model.MatchConfirmed || status == model.MatchRejected {
					match.ConfirmedBy = opponentID
				}
				s.matches[match.ID] = match
			}
		}
	}

	seedLeagueMatches(s, leagues, seedUsersForActivity, rng, currentYear, 200)
	seedLeagueMatches(s, leagues, seedUsersForActivity, rng, previousYear, 200)
	seedFriendlyMatches(s, seedUsersForActivity, rng, currentYear, 200)
	seedFriendlyMatches(s, seedUsersForActivity, rng, previousYear, 200)
}

func seedLeagueMatches(s *MemoryStore, leagues []model.League, users []model.User, rng *rand.Rand, year int, count int) {
	if len(leagues) == 0 || len(users) < 2 {
		return
	}
	for i := 0; i < count; i++ {
		league := leagues[rng.Intn(len(leagues))]
		playerAID, playerBID := pickTwoPlayers(league.PlayerIDs, rng)
		playedAt := randomTimeInYearMonth(rng, year, (i%12)+1)
		sets := randomSets(rng, league.SetsPerMatch)
		match := model.Match{
			ID:          uuid.NewString(),
			LeagueID:    league.ID,
			PlayerAID:   playerAID,
			PlayerBID:   playerBID,
			Sets:        sets,
			Status:      model.MatchConfirmed,
			ReportedBy:  playerAID,
			ConfirmedBy: playerBID,
			CreatedAt:   playedAt,
		}
		s.matches[match.ID] = match
	}
}

func seedFriendlyMatches(s *MemoryStore, users []model.User, rng *rand.Rand, year int, count int) {
	if len(users) < 2 {
		return
	}
	userIDs := userIDs(users)
	for i := 0; i < count; i++ {
		playerAID, playerBID := pickTwoPlayers(userIDs, rng)
		playedAt := randomTimeInYearMonth(rng, year, (i%12)+1)
		sets := randomSets(rng, 5)
		match := model.FriendlyMatch{
			ID:          uuid.NewString(),
			PlayerAID:   playerAID,
			PlayerBID:   playerBID,
			Sets:        sets,
			Status:      model.MatchConfirmed,
			ReportedBy:  playerAID,
			ConfirmedBy: playerBID,
			PlayedAt:    playedAt,
			CreatedAt:   playedAt,
		}
		s.friendlies[match.ID] = match
	}
}

func findUserByEmail(users []model.User, email string) (model.User, bool) {
	for _, u := range users {
		if strings.EqualFold(u.Email, email) {
			return u, true
		}
	}
	return model.User{}, false
}

func filterUsersByEmail(users []model.User, excludeEmail string) []model.User {
	filtered := make([]model.User, 0, len(users))
	for _, u := range users {
		if strings.EqualFold(u.Email, excludeEmail) {
			continue
		}
		filtered = append(filtered, u)
	}
	return filtered
}

func ensureUserInLeague(s *MemoryStore, league *model.League, userID string) {
	for _, id := range league.PlayerIDs {
		if id == userID {
			return
		}
	}
	league.PlayerIDs = append(league.PlayerIDs, userID)
	s.leagues[league.ID] = *league
}

func pickOpponentExcluding(playerIDs []string, excludeID string, rng *rand.Rand) string {
	candidates := make([]string, 0, len(playerIDs))
	for _, id := range playerIDs {
		if id == excludeID {
			continue
		}
		candidates = append(candidates, id)
	}
	if len(candidates) == 0 {
		return ""
	}
	return candidates[rng.Intn(len(candidates))]
}

func containsID(ids []string, needle string) bool {
	for _, id := range ids {
		if id == needle {
			return true
		}
	}
	return false
}

func leagueStatusForSeed(start time.Time, end *time.Time, now time.Time) model.LeagueStatus {
	if end != nil && end.Before(now) {
		return model.LeagueStatusFinished
	}
	if start.After(now) {
		return model.LeagueStatusUpcoming
	}
	return model.LeagueStatusActive
}

func pickUserIDs(users []model.User, rng *rand.Rand, count int) []string {
	if count > len(users) {
		count = len(users)
	}
	ids := userIDs(users)
	rng.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
	return append([]string{}, ids[:count]...)
}

func userIDs(users []model.User) []string {
	ids := make([]string, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.ID)
	}
	return ids
}

func pickTwoPlayers(ids []string, rng *rand.Rand) (string, string) {
	if len(ids) < 2 {
		return "", ""
	}
	a := ids[rng.Intn(len(ids))]
	b := ids[rng.Intn(len(ids))]
	for b == a {
		b = ids[rng.Intn(len(ids))]
	}
	return a, b
}

func randomSets(rng *rand.Rand, maxSets int) []model.SetScore {
	if maxSets < 3 {
		maxSets = 3
	}
	totalSets := 3 + rng.Intn(maxSets-2)
	sets := make([]model.SetScore, 0, totalSets)
	for i := 0; i < totalSets; i++ {
		a := 11 + rng.Intn(6)
		b := 11 + rng.Intn(6)
		if a == b {
			b += 2
		}
		sets = append(sets, model.SetScore{A: a, B: b})
	}
	return sets
}

func randomTimeInYearMonth(rng *rand.Rand, year int, month int) time.Time {
	if month < 1 || month > 12 {
		month = 1
	}
	day := 1 + rng.Intn(27)
	hour := rng.Intn(22)
	min := rng.Intn(60)
	return time.Date(year, time.Month(month), day, hour, min, 0, 0, time.Local)
}
