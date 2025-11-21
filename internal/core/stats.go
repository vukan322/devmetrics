package core

import "time"

type Identity struct {
	Name     string
	Username string
	Avatar   string
	Handles  []string
}

type Totals struct {
	PublicRepos  int
	PrivateRepos int
	Stars        int
	Followers    int
	Following    int
}

type LanguageStat struct {
	Name       string
	Percentage float64
}

type Activity struct {
	ContributionsPerDay map[time.Time]int
	TopLanguages        []LanguageStat
}

type DevStats struct {
	Identity Identity
	Totals   Totals
	Activity Activity
}
