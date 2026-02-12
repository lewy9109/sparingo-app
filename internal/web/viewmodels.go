package web

import "sqoush-app/internal/model"

type BaseView struct {
	Title           string
	CurrentUser     model.User
	Users           []model.User
	IsAuthenticated bool
	IsDev           bool
	FlashSuccess    string
}

type HomeView struct {
	BaseView
	Leagues               []model.League
	AvailableLeagues      []model.League
	AvailablePage         int
	AvailableTotalPages   int
	AvailablePages        []int
	AvailableHasPrev      bool
	AvailableHasNext      bool
	AvailablePrevPage     int
	AvailableNextPage     int
	JoinRequests          []JoinRequestDashboardItem
	RecentActivity        []RecentActivityItem
	RecentPending         []RecentActivityItem
	RecentFriendlyMatches []FriendlyMatchView
	FriendlyForm          FriendlyFormView
	DashboardRecent       DashboardTabView
	DashboardPending      DashboardTabView
	DashboardFriendly     DashboardTabView
	LeagueSearch          LeagueSearchView
}

type LeagueView struct {
	BaseView
	League          model.League
	Players         []model.User
	Standings       []StandingEntry
	Matches         []MatchView
	PendingOnly     bool
	PlayersPanel    LeaguePlayersPanelView
	PlayerSearch    PlayerSearchView
	SetsRange       []int
	IsAdmin         bool
	IsPlayer        bool
	PendingJoin     bool
	Admins          []LeagueAdminView
	AdminCandidates []model.User
	JoinRequests    []LeagueJoinRequestView
	Page            int
	TotalPages      int
	Pages           []int
	HasPrev         bool
	HasNext         bool
	PrevPage        int
	NextPage        int
}

type LeagueSearchView struct {
	BaseView
	Query      string
	Results    []LeagueSearchResultView
	EmptyQuery bool
	Page       int
	TotalPages int
	Pages      []int
	HasPrev    bool
	HasNext    bool
	PrevPage   int
	NextPage   int
}

type LeagueSearchResultView struct {
	League   model.League
	InLeague bool
	Pending  bool
}

type MatchView struct {
	Match      model.Match
	PlayerA    model.User
	PlayerB    model.User
	ScoreLine  string
	CanConfirm bool
	CanReject  bool
	StatusText string
}

type RecentActivityItem struct {
	Kind          string
	MatchID       string
	PlayerA       model.User
	PlayerB       model.User
	ScoreLine     string
	StatusText    string
	CanConfirm    bool
	CanReject     bool
	PlayedAtLabel string
	LeagueName    string
}

type FriendlyMatchView struct {
	Match         model.FriendlyMatch
	PlayerA       model.User
	PlayerB       model.User
	ScoreLine     string
	PlayedAtLabel string
	CanConfirm    bool
	CanReject     bool
	StatusText    string
}

type FriendlyDashboardView struct {
	Period       string
	CustomMonth  int
	CustomYear   int
	MonthOptions []MonthOption
	YearOptions  []int
	OpponentID   string
	Opponents    []model.User
	Summary      *FriendlySummaryView
	Matches      []FriendlyMatchView
	Page         int
	TotalPages   int
	Pages        []int
	HasPrev      bool
	HasNext      bool
	PrevPage     int
	NextPage     int
}

type FriendlySearchResult struct {
	User model.User
}

type FriendlySearchView struct {
	Query      string
	Results    []FriendlySearchResult
	EmptyQuery bool
	SwapOOB    bool
}

type FriendlySearchInputView struct {
	Query   string
	SwapOOB bool
}

type FriendlyFormView struct {
	LeagueUsers []model.User
	Search      FriendlySearchView
	SearchInput FriendlySearchInputView
	Selected    *model.User
	PlayedAt    string
	Error       string
}

type AuthView struct {
	BaseView
	Error            string
	RecaptchaSiteKey string
}

type MonthOption struct {
	Value int
	Label string
}

type FriendlySummaryView struct {
	OpponentName string
	Matches      int
	Wins         int
	Losses       int
	SetsWon      int
	SetsLost     int
	PointsWon    int
	PointsLost   int
}

type DashboardTabView struct {
	Items     []RecentActivityItem
	HasMore   bool
	MoreURL   string
	EmptyText string
}

type ReportFormView struct {
	Type        string
	Title       string
	Description string
	Error       string
}

type ReportListItem struct {
	Report   model.Report
	Reporter model.User
}

type ReportsView struct {
	BaseView
	Items []ReportListItem
}

type JoinRequestDashboardItem struct {
	League  model.League
	User    model.User
	Request model.LeagueJoinRequest
}

type MatchesListView struct {
	BaseView
	Items      []RecentActivityItem
	Page       int
	TotalPages int
	Pages      []int
	HasPrev    bool
	HasNext    bool
	PrevPage   int
	NextPage   int
	BasePath   string
}

type StandingEntry struct {
	Player     model.User
	Points     int
	Matches    int
	SetsWon    int
	SetsLost   int
	PointsWon  int
	PointsLost int
}

type LeaguePlayersPanelView struct {
	League  model.League
	Players []model.User
	SwapOOB bool
}

type PlayerSearchResult struct {
	User     model.User
	InLeague bool
}

type PlayerSearchView struct {
	LeagueID     string
	Query        string
	Results      []PlayerSearchResult
	EmptyQuery   bool
	IncludePanel bool
	PlayersPanel LeaguePlayersPanelView
	CanManage    bool
}

type LeagueAdminView struct {
	User model.User
	Role model.LeagueAdminRole
}

type LeagueJoinRequestView struct {
	Request model.LeagueJoinRequest
	User    model.User
}
