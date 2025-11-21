package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"time"

	"github.com/vukan322/devmetrics/internal/core"
)

const (
	defaultBaseURL   = "https://api.github.com"
	defaultUserAgent = "devmetrics/0.1"
)

type Provider struct {
	client  *http.Client
	baseURL string
	token   string
}

func New(token string) *Provider {
	return &Provider{
		client:  &http.Client{Timeout: 10 * time.Second},
		baseURL: defaultBaseURL,
		token:   token,
	}
}

func (p *Provider) Name() string {
	return "github"
}

type githubUser struct {
	Login       string `json:"login"`
	Name        string `json:"name"`
	AvatarURL   string `json:"avatar_url"`
	PublicRepos int    `json:"public_repos"`
	Followers   int    `json:"followers"`
	Following   int    `json:"following"`
}

type githubRepo struct {
	Name            string `json:"name"`
	StargazersCount int    `json:"stargazers_count"`
	Language        string `json:"language"`
	Private         bool   `json:"private"`
}

func (p *Provider) Fetch(ctx context.Context, handle string) (core.DevStats, error) {
	user, err := p.fetchUser(ctx, handle)
	if err != nil {
		return core.DevStats{}, fmt.Errorf("github: fetch user: %w", err)
	}

	repos, err := p.fetchRepos(ctx, handle)
	if err != nil {
		return core.DevStats{}, fmt.Errorf("github: fetch repos: %w", err)
	}

	totals := core.Totals{
		PublicRepos:  user.PublicRepos,
		PrivateRepos: countPrivate(repos),
		Stars:        sumStars(repos),
		Followers:    user.Followers,
		Following:    user.Following,
	}

	topLangs := computeLanguages(repos)

	avatarData, err := fetchAvatar(ctx, p.client, user.AvatarURL)
	if err != nil {
		avatarData = ""
	}

	stats := core.DevStats{
		Identity: core.Identity{
			Name:     pickName(user),
			Username: user.Login,
			Avatar:   avatarData,
			Handles:  []string{"github:" + user.Login},
		},
		Totals: totals,
		Activity: core.Activity{
			ContributionsPerDay: nil, // TODO: /repos activity graph later
			TopLanguages:        topLangs,
		},
	}

	return stats, nil
}

func (p *Provider) fetchUser(ctx context.Context, handle string) (*githubUser, error) {
	endpoint := fmt.Sprintf("%s/users/%s", p.baseURL, url.PathEscape(handle))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	p.applyHeaders(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("user %q not found", handle)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status %d from %s", resp.StatusCode, endpoint)
	}

	var u githubUser
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, fmt.Errorf("decode user response: %w", err)
	}

	return &u, nil
}

func (p *Provider) fetchRepos(ctx context.Context, handle string) ([]githubRepo, error) {
	endpoint := fmt.Sprintf("%s/users/%s/repos?per_page=100&sort=updated", p.baseURL, url.PathEscape(handle))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	p.applyHeaders(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status %d from %s", resp.StatusCode, endpoint)
	}

	var repos []githubRepo
	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		return nil, fmt.Errorf("decode repos response: %w", err)
	}

	return repos, nil
}

func fetchAvatar(ctx context.Context, client *http.Client, avatarURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, avatarURL, nil)
	if err != nil {
		return "", fmt.Errorf("new avatar request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch avatar: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read avatar body: %w", err)
	}

	encoded := base64.StdEncoding.EncodeToString(data)
	return "data:image/png;base64," + encoded, nil
}

func (p *Provider) applyHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", defaultUserAgent)
	if p.token != "" {
		req.Header.Set("Authorization", "Bearer "+p.token)
	}
}

func sumStars(repos []githubRepo) int {
	var total int
	for _, r := range repos {
		total += r.StargazersCount
	}
	return total
}

func countPrivate(repos []githubRepo) int {
	var n int
	for _, r := range repos {
		if r.Private {
			n++
		}
	}
	return n
}

func computeLanguages(repos []githubRepo) []core.LanguageStat {
	counts := make(map[string]int)
	for _, r := range repos {
		if r.Language == "" {
			continue
		}
		counts[r.Language]++
	}
	if len(counts) == 0 {
		return nil
	}

	var total int
	for _, c := range counts {
		total += c
	}

	langs := make([]core.LanguageStat, 0, len(counts))
	for name, c := range counts {
		pct := float64(c) / float64(total) * 100.0
		langs = append(langs, core.LanguageStat{
			Name:       name,
			Percentage: pct,
		})
	}

	sort.Slice(langs, func(i, j int) bool {
		return langs[i].Percentage > langs[j].Percentage
	})

	if len(langs) > 5 {
		langs = langs[:5]
	}

	return langs
}

func pickName(u *githubUser) string {
	if u.Name != "" {
		return u.Name
	}
	return u.Login
}
