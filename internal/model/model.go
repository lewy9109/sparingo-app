package model

import (
	"strings"
	"time"
)

type SkillLevel string
type UserRole string

type MatchStatus string

const (
	SkillBeginner     SkillLevel = "beginner"
	SkillIntermediate SkillLevel = "intermediate"
	SkillPro          SkillLevel = "pro"

	RoleAdmin      UserRole = "admin"
	RoleUser       UserRole = "user"
	RoleSuperAdmin UserRole = "super_admin"

	MatchScheduled MatchStatus = "scheduled"
	MatchPending   MatchStatus = "pending"
	MatchConfirmed MatchStatus = "confirmed"
	MatchRejected  MatchStatus = "rejected"
)

type User struct {
	ID           string
	FirstName    string
	LastName     string
	Phone        string
	Email        string
	PasswordHash string
	Role         UserRole
	Skill        SkillLevel
	AvatarURL    string
}

func (u User) FullName() string {
	first := strings.TrimSpace(u.FirstName)
	last := strings.TrimSpace(u.LastName)
	if first == "" {
		return last
	}
	if last == "" {
		return first
	}
	return first + " " + last
}

type League struct {
	ID           string
	Name         string
	Description  string
	Location     string
	OwnerID      string
	AdminRoles   map[string]LeagueAdminRole
	PlayerIDs    []string
	SetsPerMatch int
	StartDate    time.Time
	EndDate      *time.Time
	Status       LeagueStatus
	CreatedAt    time.Time
}

type LeagueAdminRole string

type LeagueStatus string

const (
	LeagueAdminPlayer    LeagueAdminRole = "admin-player"
	LeagueAdminModerator LeagueAdminRole = "moderator"

	LeagueStatusActive   LeagueStatus = "active"
	LeagueStatusFinished LeagueStatus = "finished"
	LeagueStatusUpcoming LeagueStatus = "upcoming"
)

type SetScore struct {
	A int
	B int
}

type Match struct {
	ID          string
	LeagueID    string
	PlayerAID   string
	PlayerBID   string
	Sets        []SetScore
	Status      MatchStatus
	ReportedBy  string
	ConfirmedBy string
	CreatedAt   time.Time
}

type FriendlyMatch struct {
	ID          string
	PlayerAID   string
	PlayerBID   string
	Sets        []SetScore
	Status      MatchStatus
	ReportedBy  string
	ConfirmedBy string
	PlayedAt    time.Time
	CreatedAt   time.Time
}

type ReportType string
type ReportStatus string

const (
	ReportBug     ReportType = "bug"
	ReportFeature ReportType = "feature"

	ReportOpen   ReportStatus = "open"
	ReportClosed ReportStatus = "closed"
)

type Report struct {
	ID          string
	UserID      string
	Type        ReportType
	Title       string
	Description string
	Status      ReportStatus
	CreatedAt   time.Time
}
