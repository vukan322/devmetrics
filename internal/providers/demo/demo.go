package demo

import (
	"context"
	"time"

	"github.com/vukan322/devmetrics/internal/core"
)

type DemoProvider struct{}

func New() *DemoProvider {
	return &DemoProvider{}
}

func (d *DemoProvider) Name() string {
	return "demo"
}

func (d *DemoProvider) Fetch(ctx context.Context, handle string) (core.DevStats, error) {
	now := time.Now()
	contribs := make(map[time.Time]int, 7)

	for i := range 7 {
		day := now.AddDate(0, 0, -i)
		contribs[time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())] = 3 + i
	}

	return core.DevStats{
		Identity: core.Identity{
			Name:     "Demo Developer",
			Username: handle,
			Avatar:   "",
			Handles:  []string{"demo:" + handle},
		},
		Totals: core.Totals{
			PublicRepos:  12,
			PrivateRepos: 3,
			Stars:        32,
			Followers:    10,
			Following:    5,
		},
		Activity: core.Activity{
			ContributionsPerDay: contribs,
			TopLanguages: []core.LanguageStat{
				{Name: "Go", Percentage: 70},
				{Name: "TypeScript", Percentage: 20},
				{Name: "Lua", Percentage: 10},
			},
		},
	}, nil
}
