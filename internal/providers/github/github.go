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
	"strings"
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
	Login       string    `json:"login"`
	Name        string    `json:"name"`
	AvatarURL   string    `json:"avatar_url"`
	PublicRepos int       `json:"public_repos"`
	Followers   int       `json:"followers"`
	Following   int       `json:"following"`
	CreatedAt   time.Time `json:"created_at"`
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

	contributedCount, err := p.fetchContributedRepos(ctx, handle)
	if err != nil {
		contributedCount = 0
	}

	topLangs, totalLangs := computeLanguages(repos)

	totals := core.Totals{
		PublicRepos:      user.PublicRepos,
		PrivateRepos:     countPrivate(repos),
		Stars:            sumStars(repos),
		Followers:        user.Followers,
		Following:        user.Following,
		ContributedRepos: contributedCount,
		JoinedAgo:        formatJoinedAgo(user.CreatedAt),
		TotalLanguages:   totalLangs,
	}

	avatarData, err := fetchAvatar(ctx, p.client, user.AvatarURL)
	if err != nil {
		avatarData = ""
	}

	stats := core.DevStats{
		Identity: core.Identity{
			Name:     pickName(user),
			Username: user.Login,
			Avatar:   avatarData,
			Handles:  []string{"github: " + user.Login},
		},
		Totals: totals,
		Activity: core.Activity{
			ContributionsPerDay: nil,
			TopLanguages:        topLangs,
		},
	}

	return stats, nil
}

func formatJoinedAgo(created time.Time) string {
	years := time.Since(created).Hours() / 24 / 365
	if years < 1 {
		months := int(time.Since(created).Hours() / 24 / 30)
		if months < 1 {
			return "this month"
		}
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	}
	y := int(years)
	if y == 1 {
		return "1 year ago"
	}
	return fmt.Sprintf("%d years ago", y)
}
func (p *Provider) fetchContributedRepos(ctx context.Context, handle string) (int, error) {
	endpoint := fmt.Sprintf("%s/search/issues?q=author:%s+type:pr+is:merged+-user:%s&per_page=1",
		p.baseURL, url.QueryEscape(handle), url.QueryEscape(handle))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, fmt.Errorf("new request: %w", err)
	}
	p.applyHeaders(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var result struct {
		TotalCount int `json:"total_count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("decode response: %w", err)
	}

	return result.TotalCount, nil
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
	var allRepos []githubRepo
	nextURL := fmt.Sprintf("%s/users/%s/repos?per_page=100&sort=updated", p.baseURL, url.PathEscape(handle))

	for nextURL != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, nextURL, nil)
		if err != nil {
			return nil, fmt.Errorf("new request: %w", err)
		}
		p.applyHeaders(req)

		resp, err := p.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("do request: %w", err)
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			resp.Body.Close()
			return nil, fmt.Errorf("unexpected status %d from %s", resp.StatusCode, nextURL)
		}

		var repos []githubRepo
		if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("decode repos response: %w", err)
		}
		resp.Body.Close()

		allRepos = append(allRepos, repos...)

		nextURL = extractNextLink(resp.Header.Get("Link"))
	}

	return allRepos, nil
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

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("avatar fetch failed with status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read avatar body: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}

	encoded := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", contentType, encoded), nil
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

func computeLanguages(repos []githubRepo) ([]core.LanguageStat, int) {
	counts := make(map[string]int)
	for _, r := range repos {
		if r.Language == "" {
			continue
		}
		counts[r.Language]++
	}
	if len(counts) == 0 {
		return nil, 0
	}

	totalLanguages := len(counts)

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

	if len(langs) <= 9 {
		return langs, totalLanguages
	}

	top9 := langs[:9]
	var othersTotal float64
	for i := 9; i < len(langs); i++ {
		othersTotal += langs[i].Percentage
	}

	result := append(top9, core.LanguageStat{
		Name:       "Others",
		Percentage: othersTotal,
	})

	return result, totalLanguages
}

func pickName(u *githubUser) string {
	if u.Name != "" {
		return u.Name
	}
	return u.Login
}

func extractNextLink(linkHeader string) string {
	if linkHeader == "" {
		return ""
	}

	for _, link := range splitLinks(linkHeader) {
		parts := splitLinkParts(link)
		if len(parts) == 2 && contains(parts[1], `rel="next"`) {
			url := strings.Trim(strings.TrimSpace(parts[0]), "<>")
			return url
		}
	}
	return ""
}

func splitLinks(s string) []string {
	var links []string
	current := ""
	inBracket := false

	for _, ch := range s {
		if ch == '<' {
			inBracket = true
		} else if ch == '>' {
			inBracket = false
		} else if ch == ',' && !inBracket {
			links = append(links, current)
			current = ""
			continue
		}
		current += string(ch)
	}
	if current != "" {
		links = append(links, current)
	}
	return links
}

func splitLinkParts(s string) []string {
	parts := make([]string, 0, 2)
	current := ""

	for _, ch := range s {
		if ch == ';' {
			parts = append(parts, current)
			current = ""
			continue
		}
		current += string(ch)
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr) >= 0
}

func findSubstring(s, substr string) int {
	if len(substr) == 0 {
		return 0
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
