package bitbucket

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sort"
	"time"

	"github.com/vukan322/devmetrics/internal/core"
)

type Provider struct {
	client  *http.Client
	baseURL string
	email   string
	token   string
	user    string
}

func New(email, token, user string) *Provider {
	return &Provider{
		client:  &http.Client{Timeout: 10 * time.Second},
		baseURL: "https://api.bitbucket.org/2.0",
		email:   email,
		token:   token,
		user:    user,
	}
}

func (p *Provider) Name() string {
	return "bitbucket"
}

type bitbucketUser struct {
	AccountID   string `json:"account_id"`
	Username    string `json:"username"`
	Nickname    string `json:"nickname"`
	DisplayName string `json:"display_name"`
	Links       struct {
		Avatar struct {
			Href string `json:"href"`
		} `json:"avatar"`
	} `json:"links"`
}

type bitbucketRepo struct {
	IsPrivate bool   `json:"is_private"`
	Language  string `json:"language"`
}

type pagedReposResponse struct {
	Values []bitbucketRepo `json:"values"`
	Next   string          `json:"next"`
}

func (p *Provider) Fetch(ctx context.Context, handle string) (core.DevStats, error) {
	user, err := p.fetchUser(ctx)
	if err != nil {
		return core.DevStats{}, fmt.Errorf("bitbucket: fetch user: %w", err)
	}

	repos, err := p.fetchRepos(ctx, p.user)
	if err != nil {
		return core.DevStats{}, fmt.Errorf("bitbucket: fetch repos: %w", err)
	}

	publicRepos := 0
	privateRepos := 0
	for _, r := range repos {
		if r.IsPrivate {
			privateRepos++
		} else {
			publicRepos++
		}
	}

	identity := core.Identity{
		Name:     user.DisplayName,
		Username: user.Nickname,
		Avatar:   "",
		Handles:  []string{"bitbucket: " + handle},
	}

	totals := core.Totals{
		PublicRepos:  publicRepos,
		PrivateRepos: privateRepos,
	}

	stats := core.DevStats{
		Identity: identity,
		Totals:   totals,
		Activity: core.Activity{},
	}

	log.Printf("bitbucket: fetched user %q repos=%d public=%d private=%d", user.Nickname, len(repos), publicRepos, privateRepos)

	return stats, nil
}

func (p *Provider) fetchUser(ctx context.Context) (*bitbucketUser, error) {
	endpoint := fmt.Sprintf("%s/user", p.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("bitbucket: new request: %w", err)
	}
	p.applyAuth(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bitbucket: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("bitbucket: unauthorized (401), check DEV_METRICS_BITBUCKET_EMAIL and DEV_METRICS_BITBUCKET_TOKEN")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("bitbucket: fetch user: unexpected status %d from %s", resp.StatusCode, endpoint)
	}

	var u bitbucketUser
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, fmt.Errorf("bitbucket: decode user response: %w", err)
	}

	return &u, nil
}

func (p *Provider) fetchRepos(ctx context.Context, handle string) ([]bitbucketRepo, error) {
	var all []bitbucketRepo

	nextURL := fmt.Sprintf("%s/repositories/%s?pagelen=100", p.baseURL, url.PathEscape(handle))

	for nextURL != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, nextURL, nil)
		if err != nil {
			return nil, fmt.Errorf("new request: %w", err)
		}
		p.applyAuth(req)

		resp, err := p.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("do request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("unexpected status %d from %s", resp.StatusCode, nextURL)
		}

		var page pagedReposResponse
		if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
			return nil, fmt.Errorf("decode repos response: %w", err)
		}

		all = append(all, page.Values...)
		nextURL = page.Next
	}

	return all, nil
}

func (p *Provider) applyAuth(req *http.Request) {
	if p.email == "" || p.token == "" {
		return
	}

	creds := p.email + ":" + p.token
	encoded := base64.StdEncoding.EncodeToString([]byte(creds))

	req.Header.Set("Authorization", "Basic "+encoded)
	req.Header.Set("Accept", "application/json")
}

func computeLanguages(repos []bitbucketRepo) ([]core.LanguageStat, int) {
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

	total := 0
	for _, c := range counts {
		total += c
	}

	langs := make([]core.LanguageStat, 0, len(counts))
	for name, c := range counts {
		pct := float64(c) / float64(total) * 100.0
		langs = append(langs, core.LanguageStat{
			Name:       name,
			Percentage: pct,
			Color:      "#586069",
		})
	}

	sort.Slice(langs, func(i, j int) bool {
		return langs[i].Percentage > langs[j].Percentage
	})

	return langs, len(langs)
}
