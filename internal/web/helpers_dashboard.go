package web

import (
	"math"
	"time"

	"sqoush-app/internal/model"
)

type activityEntry struct {
	When time.Time
	Item RecentActivityItem
}

func (s *Server) leagueActivityEntries(currentUser model.User, statuses map[model.MatchStatus]bool) []activityEntry {
	entries := []activityEntry{}
	for _, league := range s.leaguesForUser(currentUser.ID) {
		matches := s.store.ListMatches(league.ID)
		for _, match := range matches {
			if match.PlayerAID != currentUser.ID && match.PlayerBID != currentUser.ID {
				continue
			}
			if !statuses[match.Status] {
				continue
			}
			view := s.matchView(match, currentUser)
			item := RecentActivityItem{
				Kind:          "league",
				MatchID:       match.ID,
				PlayerA:       view.PlayerA,
				PlayerB:       view.PlayerB,
				ScoreLine:     view.ScoreLine,
				StatusText:    view.StatusText,
				CanConfirm:    view.CanConfirm,
				CanReject:     view.CanReject,
				PlayedAtLabel: match.CreatedAt.Format("02 Jan 2006 15:04"),
				LeagueName:    league.Name,
			}
			entries = append(entries, activityEntry{
				When: match.CreatedAt,
				Item: item,
			})
		}
	}
	return entries
}

func (s *Server) friendlyActivityEntries(currentUser model.User, statuses map[model.MatchStatus]bool) []activityEntry {
	entries := []activityEntry{}
	for _, match := range s.store.ListFriendlyMatches() {
		if match.PlayerAID != currentUser.ID && match.PlayerBID != currentUser.ID {
			continue
		}
		if !statuses[match.Status] {
			continue
		}
		view := s.friendlyMatchView(match, currentUser)
		item := RecentActivityItem{
			Kind:          "friendly",
			MatchID:       match.ID,
			PlayerA:       view.PlayerA,
			PlayerB:       view.PlayerB,
			ScoreLine:     view.ScoreLine,
			StatusText:    view.StatusText,
			CanConfirm:    view.CanConfirm,
			CanReject:     view.CanReject,
			PlayedAtLabel: view.PlayedAtLabel,
		}
		entries = append(entries, activityEntry{
			When: match.PlayedAt,
			Item: item,
		})
	}
	return entries
}

func buildDashboardTabView(entries []activityEntry, maxTotal int, maxDisplay int, moreURL string, emptyText string) DashboardTabView {
	total := len(entries)
	if maxTotal > 0 && total > maxTotal {
		entries = entries[:maxTotal]
	}
	items := make([]RecentActivityItem, 0, len(entries))
	for _, entry := range entries {
		items = append(items, entry.Item)
	}
	view := DashboardTabView{
		Items:     items,
		HasMore:   total > maxDisplay,
		MoreURL:   moreURL,
		EmptyText: emptyText,
	}
	if len(view.Items) > maxDisplay {
		view.Items = view.Items[:maxDisplay]
	}
	return view
}

func (s *Server) buildMatchesListView(entries []activityEntry, page int, pageSize int, basePath string, currentUser model.User, title string) MatchesListView {
	if page < 1 {
		page = 1
	}
	total := len(entries)
	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))
	if totalPages > 0 && page > totalPages {
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
	items := make([]RecentActivityItem, 0, pageSize)
	for _, entry := range entries[start:end] {
		items = append(items, entry.Item)
	}

	view := MatchesListView{
		BaseView: BaseView{
			Title:           title,
			CurrentUser:     currentUser,
			Users:           s.store.ListUsers(),
			IsAuthenticated: currentUser.ID != "",
			IsDev:           isDevMode(),
		},
		Items:      items,
		Page:       page,
		TotalPages: totalPages,
		BasePath:   basePath,
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
