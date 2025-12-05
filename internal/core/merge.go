package core

import (
	"sort"
	"time"
)

func MergeStats(primary, secondary DevStats) DevStats {
	merged := primary

	merged.Identity.Handles = append(merged.Identity.Handles, secondary.Identity.Handles...)

	merged.Totals.PublicRepos += secondary.Totals.PublicRepos
	merged.Totals.PrivateRepos += secondary.Totals.PrivateRepos
	merged.Totals.Stars += secondary.Totals.Stars
	merged.Totals.Followers += secondary.Totals.Followers
	merged.Totals.Following += secondary.Totals.Following
	merged.Totals.ContributedRepos += secondary.Totals.ContributedRepos
	merged.Totals.Commits += secondary.Totals.Commits

	merged.Activity.Issues.Open += secondary.Activity.Issues.Open
	merged.Activity.Issues.Closed += secondary.Activity.Issues.Closed

	merged.Activity.PullRequests.Open += secondary.Activity.PullRequests.Open
	merged.Activity.PullRequests.Merged += secondary.Activity.PullRequests.Merged
	merged.Activity.PullRequests.Closed += secondary.Activity.PullRequests.Closed

	if secondary.Activity.ContributionsPerDay != nil {
		if merged.Activity.ContributionsPerDay == nil {
			merged.Activity.ContributionsPerDay = make(map[time.Time]int)
		}
		for day, count := range secondary.Activity.ContributionsPerDay {
			merged.Activity.ContributionsPerDay[day] += count
		}
	}

	langs, totalLangs := mergeLanguageStats(
		merged.Activity.TopLanguages,
		secondary.Activity.TopLanguages,
	)
	merged.Activity.TopLanguages = langs
	merged.Totals.TotalLanguages = totalLangs

	current, longest := ComputeStreaks(merged.Activity.ContributionsPerDay)
	merged.Totals.CurrentStreak = current
	merged.Totals.LongestStreak = longest

	return merged
}

func mergeLanguageStats(a, b []LanguageStat) ([]LanguageStat, int) {
	if len(a) == 0 && len(b) == 0 {
		return nil, 0
	}

	m := make(map[string]LanguageStat)

	for _, ls := range a {
		m[ls.Name] = ls
	}

	for _, ls := range b {
		if existing, ok := m[ls.Name]; ok {
			totalPct := existing.Percentage + ls.Percentage
			color := existing.Color
			if color == "" {
				color = ls.Color
			}
			if color == "" {
				color = "#586069"
			}
			m[ls.Name] = LanguageStat{
				Name:       ls.Name,
				Percentage: totalPct,
				Color:      color,
			}
		} else {
			color := ls.Color
			if color == "" {
				color = "#586069"
			}
			m[ls.Name] = LanguageStat{
				Name:       ls.Name,
				Percentage: ls.Percentage,
				Color:      color,
			}
		}
	}

	result := make([]LanguageStat, 0, len(m))
	var total float64
	for _, ls := range m {
		total += ls.Percentage
		result = append(result, ls)
	}

	if total > 0 {
		for i := range result {
			result[i].Percentage = result[i].Percentage / total * 100.0
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Percentage > result[j].Percentage
	})

	return result, len(result)
}

func ComputeStreaks(contribs map[time.Time]int) (int, int) {
	if len(contribs) == 0 {
		return 0, 0
	}

	normalized := make(map[time.Time]int, len(contribs))
	for t, c := range contribs {
		d := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
		normalized[d] += c
	}

	var dates []time.Time
	for d := range normalized {
		dates = append(dates, d)
	}
	sort.Slice(dates, func(i, j int) bool {
		return dates[i].Before(dates[j])
	})

	today := time.Now().UTC()
	todayDate := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)

	current := 0
	for d := todayDate; ; d = d.AddDate(0, 0, -1) {
		if normalized[d] <= 0 {
			break
		}
		current++
	}

	longest := 0
	seen := make(map[time.Time]bool)

	for _, d := range dates {
		if seen[d] {
			continue
		}
		if normalized[d] <= 0 {
			continue
		}
		prev := d.AddDate(0, 0, -1)
		if normalized[prev] > 0 {
			continue
		}

		length := 0
		for cur := d; normalized[cur] > 0; cur = cur.AddDate(0, 0, 1) {
			seen[cur] = true
			length++
		}
		if length > longest {
			longest = length
		}
	}

	return current, longest
}
