package web

import (
	"sort"

	"sqoush-app/internal/model"
)

func BuildStandings(players []model.User, matches []model.Match) []StandingEntry {
	index := make(map[string]*StandingEntry)
	for _, p := range players {
		entry := &StandingEntry{Player: p}
		index[p.ID] = entry
	}

	for _, match := range matches {
		if match.Status != model.MatchConfirmed {
			continue
		}
		entryA := index[match.PlayerAID]
		entryB := index[match.PlayerBID]
		if entryA == nil || entryB == nil {
			continue
		}
		entryA.Matches++
		entryB.Matches++

		setsWonA := 0
		setsWonB := 0
		for _, set := range match.Sets {
			entryA.PointsWon += set.A
			entryA.PointsLost += set.B
			entryB.PointsWon += set.B
			entryB.PointsLost += set.A
			if set.A > set.B {
				setsWonA++
			} else {
				setsWonB++
			}
		}
		entryA.SetsWon += setsWonA
		entryA.SetsLost += setsWonB
		entryB.SetsWon += setsWonB
		entryB.SetsLost += setsWonA

		if setsWonA > setsWonB {
			entryA.Points += 3
			entryB.Points += 1
		} else {
			entryB.Points += 3
			entryA.Points += 1
		}
	}

	standings := make([]StandingEntry, 0, len(players))
	for _, entry := range index {
		standings = append(standings, *entry)
	}
	sort.Slice(standings, func(i, j int) bool {
		if standings[i].Points == standings[j].Points {
			if standings[i].SetsWon-standings[i].SetsLost == standings[j].SetsWon-standings[j].SetsLost {
				return standings[i].PointsWon-standings[i].PointsLost > standings[j].PointsWon-standings[j].PointsLost
			}
			return standings[i].SetsWon-standings[i].SetsLost > standings[j].SetsWon-standings[j].SetsLost
		}
		return standings[i].Points > standings[j].Points
	})
	return standings
}
