package web

import (
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"sqoush-app/internal/model"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type friendlyDashboardParams struct {
	Period     string
	Month      int
	Year       int
	OpponentID string
	Page       int
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
		{Value: 1, Label: "Styczeń"},
		{Value: 2, Label: "Luty"},
		{Value: 3, Label: "Marzec"},
		{Value: 4, Label: "Kwiecień"},
		{Value: 5, Label: "Maj"},
		{Value: 6, Label: "Czerwiec"},
		{Value: 7, Label: "Lipiec"},
		{Value: 8, Label: "Sierpień"},
		{Value: 9, Label: "Wrzesień"},
		{Value: 10, Label: "Październik"},
		{Value: 11, Label: "Listopad"},
		{Value: 12, Label: "Grudzień"},
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
