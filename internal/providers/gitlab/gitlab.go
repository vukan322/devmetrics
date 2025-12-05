package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/vukan322/devmetrics/internal/core"
)

type Provider struct {
	client  *http.Client
	baseURL string
	token   string
	user    string
}

func New(token, user string) *Provider {
	return &Provider{
		client:  &http.Client{Timeout: 10 * time.Second},
		baseURL: "https://gitlab.com/api/v4",
		token:   token,
		user:    user,
	}
}

func (p *Provider) Name() string {
	return "gitlab"
}

type gitlabUser struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
	Avatar   string `json:"avatar_url"`
}

type gitlabProject struct {
	ID                int    `json:"id"`
	Name              string `json:"name"`
	PathWithNamespace string `json:"path_with_namespace"`
	Visibility        string `json:"visibility"`
	StarCount         int    `json:"star_count"`
}

type gitlabLanguages map[string]float64

func (p *Provider) Fetch(ctx context.Context, handle string) (core.DevStats, error) {
	user, err := p.fetchUser(ctx, handle)
	if err != nil {
		return core.DevStats{}, fmt.Errorf("gitlab: fetch user: %w", err)
	}

	projects, err := p.fetchProjects(ctx, user.ID)
	if err != nil {
		return core.DevStats{}, fmt.Errorf("gitlab: fetch projects: %w", err)
	}

	publicCount := 0
	privateCount := 0
	totalStars := 0

	for _, pr := range projects {
		switch pr.Visibility {
		case "public":
			publicCount++
		case "private", "internal":
			privateCount++
		}
		totalStars += pr.StarCount
	}

	topLangs, _ := p.computeLanguages(ctx, projects)

	identity := core.Identity{
		Name:     pickName(user),
		Username: user.Username,
		Avatar:   "",
		Handles:  []string{"gitlab: " + handle},
	}

	totals := core.Totals{
		PublicRepos:  publicCount,
		PrivateRepos: privateCount,
		Stars:        totalStars,
	}

	stats := core.DevStats{
		Identity: identity,
		Totals:   totals,
		Activity: core.Activity{
			TopLanguages: topLangs,
		},
	}

	return stats, nil
}

func (p *Provider) fetchUser(ctx context.Context, handle string) (*gitlabUser, error) {
	endpoint := fmt.Sprintf("%s/users?username=%s", p.baseURL, url.QueryEscape(handle))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("gitlab: new user request: %w", err)
	}
	p.applyAuth(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gitlab: do user request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("gitlab: fetch user: unexpected status %d from %s", resp.StatusCode, endpoint)
	}

	var users []gitlabUser
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return nil, fmt.Errorf("gitlab: decode user response: %w", err)
	}

	if len(users) == 0 {
		return nil, fmt.Errorf("gitlab: user %q not found", handle)
	}

	return &users[0], nil
}

func (p *Provider) fetchProjects(ctx context.Context, userID int) ([]gitlabProject, error) {
	var all []gitlabProject
	page := 1

	for {
		endpoint := fmt.Sprintf(
			"%s/users/%d/projects?per_page=100&page=%d&simple=true&order_by=last_activity_at&sort=desc",
			p.baseURL,
			userID,
			page,
		)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("gitlab: new projects request: %w", err)
		}
		p.applyAuth(req)

		resp, err := p.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("gitlab: do projects request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("gitlab: fetch projects: unexpected status %d from %s", resp.StatusCode, endpoint)
		}

		var pageProjects []gitlabProject
		if err := json.NewDecoder(resp.Body).Decode(&pageProjects); err != nil {
			return nil, fmt.Errorf("gitlab: decode projects response: %w", err)
		}

		if len(pageProjects) == 0 {
			break
		}

		all = append(all, pageProjects...)
		page++
	}

	return all, nil
}

func (p *Provider) computeLanguages(ctx context.Context, projects []gitlabProject) ([]core.LanguageStat, int) {
	counts := map[string]float64{}

	for _, pr := range projects {
		langs, err := p.fetchProjectLanguages(ctx, pr.ID)
		if err != nil {
			log.Printf("gitlab: fetch languages failed for project %d (%s): %v", pr.ID, pr.PathWithNamespace, err)
			continue
		}

		for name, val := range langs {
			counts[name] += val
		}
	}

	if len(counts) == 0 {
		return nil, 0
	}

	totalLanguages := len(counts)
	var total float64
	for _, v := range counts {
		total += v
	}

	langStats := make([]core.LanguageStat, 0, len(counts))
	for name, v := range counts {
		pct := (v / total) * 100.0
		langStats = append(langStats, core.LanguageStat{
			Name:       name,
			Percentage: pct,
		})
	}

	for i := 0; i < len(langStats); i++ {
		for j := i + 1; j < len(langStats); j++ {
			if langStats[j].Percentage > langStats[i].Percentage {
				langStats[i], langStats[j] = langStats[j], langStats[i]
			}
		}
	}

	if len(langStats) <= 9 {
		return langStats, totalLanguages
	}

	top9 := langStats[:9]
	var othersTotal float64
	for i := 9; i < len(langStats); i++ {
		othersTotal += langStats[i].Percentage
	}

	result := append(top9, core.LanguageStat{
		Name:       "Others",
		Percentage: othersTotal,
	})

	return result, totalLanguages
}

func (p *Provider) fetchProjectLanguages(ctx context.Context, projectID int) (gitlabLanguages, error) {
	endpoint := fmt.Sprintf("%s/projects/%d/languages", p.baseURL, projectID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("gitlab: new languages request: %w", err)
	}
	p.applyAuth(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gitlab: do languages request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return gitlabLanguages{}, nil
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("gitlab: fetch languages: unexpected status %d from %s", resp.StatusCode, endpoint)
	}

	var langs gitlabLanguages
	if err := json.NewDecoder(resp.Body).Decode(&langs); err != nil {
		return nil, fmt.Errorf("gitlab: decode languages response: %w", err)
	}

	return langs, nil
}

func (p *Provider) applyAuth(req *http.Request) {
	if p.token == "" {
		return
	}
	req.Header.Set("PRIVATE-TOKEN", p.token)
	req.Header.Set("Accept", "application/json")
}

func pickName(u *gitlabUser) string {
	if u.Name != "" {
		return u.Name
	}
	return u.Username
}
