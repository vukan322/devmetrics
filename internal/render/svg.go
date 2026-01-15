package render

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"
	"text/template"

	"github.com/vukan322/devmetrics/internal/core"
)

const (
	svgWidth  = 800
	svgHeight = 390
)

//go:embed templates/devcard.svg.tmpl
var devcardTemplate string

var devcardTmpl = template.Must(
	template.New("devcard").
		Funcs(template.FuncMap{
			"addf":    func(a, b float64) float64 { return a + b },
			"subf":    func(a, b float64) float64 { return a - b },
			"divf":    func(a, b float64) float64 { return a / b },
			"mulf":    func(a, b float64) float64 { return a * b },
			"float64": func(i int) float64 { return float64(i) },
			"divInt":  func(a, b int) int { return a / b },
			"modInt":  func(a, b int) int { return a % b },
		}).
		Parse(devcardTemplate),
)

type devcardViewModel struct {
	Width  int
	Height int

	Title     string
	Subtitle  string
	AvatarURL string

	Repos            int
	PrivateRepos     int
	Stars            int
	Followers        int
	ContributedRepos int
	JoinedAgo        string
	Languages        []core.LanguageStat
	TotalLanguages   int

	IssuesOpen   int
	IssuesClosed int

	PROpen   int
	PRMerged int
	PRClosed int

	Commits         int
	CurrentStreak   int
	LongestStreak   int
	CommitsThisWeek int
}

func RenderSVG(stats core.DevStats) ([]byte, error) {
	title := stats.Identity.Name
	if title == "" {
		title = stats.Identity.Username
	}
	subtitle := strings.Join(stats.Identity.Handles, " · ")

	langs := stats.Activity.TopLanguages

	vm := devcardViewModel{
		Width:            svgWidth,
		Height:           svgHeight,
		Title:            title,
		Subtitle:         subtitle,
		AvatarURL:        stats.Identity.Avatar,
		Repos:            stats.Totals.PublicRepos,
		PrivateRepos:     stats.Totals.PrivateRepos,
		Stars:            stats.Totals.Stars,
		Followers:        stats.Totals.Followers,
		ContributedRepos: stats.Totals.ContributedRepos,
		JoinedAgo:        stats.Totals.JoinedAgo,
		TotalLanguages:   stats.Totals.TotalLanguages,
		Languages:        langs,
		IssuesOpen:       stats.Activity.Issues.Open,
		IssuesClosed:     stats.Activity.Issues.Closed,
		PROpen:           stats.Activity.PullRequests.Open,
		PRMerged:         stats.Activity.PullRequests.Merged,
		PRClosed:         stats.Activity.PullRequests.Closed,
		Commits:          stats.Totals.Commits,
		CurrentStreak:    stats.Totals.CurrentStreak,
		LongestStreak:    stats.Totals.LongestStreak,
		CommitsThisWeek:  stats.Totals.CommitsThisWeek,
	}

	var buf bytes.Buffer
	if err := devcardTmpl.Execute(&buf, vm); err != nil {
		return nil, fmt.Errorf("render svg: %w", err)
	}
	return buf.Bytes(), nil
}
