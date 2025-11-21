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
	svgHeight = 260
)

//go:embed templates/devcard.svg.tmpl
var devcardTemplate string

var devcardTmpl = template.Must(
	template.New("devcard").
		Funcs(template.FuncMap{
			"addf": func(a, b float64) float64 { return a + b },
			"divf": func(a, b float64) float64 { return a / b },
			"mulf": func(a, b float64) float64 { return a * b },
		}).
		Parse(devcardTemplate),
)

type devcardViewModel struct {
	Width     int
	Height    int
	Title     string
	Subtitle  string
	Repos     int
	Stars     int
	Followers int
	Languages []core.LanguageStat
	Colors    []string
}

func RenderSVG(stats core.DevStats) ([]byte, error) {
	colors := []string{"#238636", "#1f6feb", "#a371f7", "#db6d28", "#8b949e"}

	title := stats.Identity.Name
	if title == "" {
		title = stats.Identity.Username
	}

	subtitle := strings.Join(stats.Identity.Handles, " · ")

	langs := stats.Activity.TopLanguages
	if len(langs) > 3 {
		langs = langs[:3]
	}

	vm := devcardViewModel{
		Width:     svgWidth,
		Height:    svgHeight,
		Title:     title,
		Subtitle:  subtitle,
		Repos:     stats.Totals.PublicRepos,
		Stars:     stats.Totals.Stars,
		Followers: stats.Totals.Followers,
		Languages: langs,
		Colors:    colors,
	}

	var buf bytes.Buffer
	if err := devcardTmpl.Execute(&buf, vm); err != nil {
		return nil, fmt.Errorf("render svg: %w", err)
	}

	return buf.Bytes(), nil
}
