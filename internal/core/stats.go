package core

import "time"

type Identity struct {
	Name     string
	Username string
	Avatar   string
	Handles  []string
}

type Totals struct {
	PublicRepos      int
	PrivateRepos     int
	Stars            int
	Followers        int
	Following        int
	ContributedRepos int
	JoinedAgo        string
	TotalLanguages   int
}

type LanguageStat struct {
	Name       string
	Percentage float64
}

type IssueStats struct {
	Open   int
	Closed int
}

type PRStats struct {
	Open   int
	Merged int
	Closed int
}

type Activity struct {
	ContributionsPerDay map[time.Time]int
	TopLanguages        []LanguageStat
	Issues              IssueStats
	PullRequests        PRStats
}

type DevStats struct {
	Identity Identity
	Totals   Totals
	Activity Activity
}
