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
	svgHeight = 360
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

	Title            string
	Subtitle         string
	AvatarURL        string
	Repos            int
	Stars            int
	Followers        int
	ContributedRepos int
	JoinedAgo        string
	Languages        []core.LanguageStat

	TotalLanguages int
	Colors         []string

	IssuesOpen   int
	IssuesClosed int

	PROpen   int
	PRMerged int
	PRClosed int
}

func RenderSVG(stats core.DevStats) ([]byte, error) {
	title := stats.Identity.Name
	if title == "" {
		title = stats.Identity.Username
	}
	subtitle := strings.Join(stats.Identity.Handles, " · ")
	langs := stats.Activity.TopLanguages
	colors := []string{
		"#2563EB", // Blue
		"#22C55E", // Green
		"#F97316", // Orange
		"#A855F7", // Purple
		"#EF4444", // Red
		"#06B6D4", // Teal
		"#FACC15", // Yellow
		"#EC4899", // Pink
		"#4B5563", // Slate
		"#9CA3AF", // Gray (Others)
	}
	vm := devcardViewModel{
		Width:            svgWidth,
		Height:           svgHeight,
		Title:            title,
		Subtitle:         subtitle,
		AvatarURL:        stats.Identity.Avatar,
		Repos:            stats.Totals.PublicRepos,
		Stars:            stats.Totals.Stars,
		Followers:        stats.Totals.Followers,
		ContributedRepos: stats.Totals.ContributedRepos,
		JoinedAgo:        stats.Totals.JoinedAgo,
		TotalLanguages:   stats.Totals.TotalLanguages,
		Languages:        langs,
		Colors:           colors,
		IssuesOpen:       stats.Activity.Issues.Open,
		IssuesClosed:     stats.Activity.Issues.Closed,
		PROpen:           stats.Activity.PullRequests.Open,
		PRMerged:         stats.Activity.PullRequests.Merged,
		PRClosed:         stats.Activity.PullRequests.Closed,
	}
	var buf bytes.Buffer
	if err := devcardTmpl.Execute(&buf, vm); err != nil {
		return nil, fmt.Errorf("render svg: %w", err)
	}
	return buf.Bytes(), nil
}
