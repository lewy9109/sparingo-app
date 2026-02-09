package web

import (
	"fmt"
	"net/http"
	"strconv"

	"sqoush-app/internal/model"
)

func parseSets(r *http.Request, maxSets int) []model.SetScore {
	sets := []model.SetScore{}
	if maxSets < 1 {
		maxSets = 5
	}
	for i := 1; i <= maxSets; i++ {
		aStr := r.FormValue(fmt.Sprintf("set_%d_a", i))
		bStr := r.FormValue(fmt.Sprintf("set_%d_b", i))
		if aStr == "" || bStr == "" {
			continue
		}
		a, errA := strconv.Atoi(aStr)
		b, errB := strconv.Atoi(bStr)
		if errA != nil || errB != nil {
			continue
		}
		sets = append(sets, model.SetScore{A: a, B: b})
	}
	return sets
}

func buildSetsRange(maxSets int) []int {
	if maxSets < 1 {
		maxSets = 5
	}
	sets := make([]int, 0, maxSets)
	for i := 1; i <= maxSets; i++ {
		sets = append(sets, i)
	}
	return sets
}

func parseSetsPerMatch(value string) int {
	if value == "" {
		return 5
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return 5
	}
	if parsed > 9 {
		return 9
	}
	return parsed
}

func parseSetsCount(value string, max int) int {
	if value == "" {
		return 5
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return 5
	}
	if max > 0 && parsed > max {
		return max
	}
	return parsed
}
