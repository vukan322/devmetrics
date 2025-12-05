package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/vukan322/devmetrics/internal/core"
	bitbucketprovider "github.com/vukan322/devmetrics/internal/providers/bitbucket"
	githubprovider "github.com/vukan322/devmetrics/internal/providers/github"
	"github.com/vukan322/devmetrics/internal/render"
)

func main() {
	_ = godotenv.Load()

	var (
		user   string
		output string
	)

	flag.StringVar(&user, "user", "", "primary username/handle (e.g. GitHub username)")
	flag.StringVar(&output, "out", "devmetrics.svg", "output SVG file path")
	flag.Parse()

	if user == "" {
		log.Fatal("missing required flag: -user")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	token := os.Getenv("DEV_METRICS_TOKEN")
	if token == "" {
		log.Println("warning: DEV_METRICS_TOKEN not set, using unauthenticated GitHub API (rate limited)")
	}

	githubProvider := githubprovider.New(token)

	githubStats, err := githubProvider.Fetch(ctx, user)
	if err != nil {
		log.Fatalf("provider %s failed: %v", githubProvider.Name(), err)
	}

	bbEmail := os.Getenv("DEV_METRICS_BITBUCKET_EMAIL")
	bbToken := os.Getenv("DEV_METRICS_BITBUCKET_TOKEN")
	bbWorkspace := os.Getenv("DEV_METRICS_BITBUCKET_WORKSPACE")
	bbUserHandle := os.Getenv("DEV_METRICS_BITBUCKET_USER")

	stats := githubStats

	if bbEmail != "" && bbToken != "" && bbWorkspace != "" {
		bitbucketProvider := bitbucketprovider.New(bbEmail, bbToken, bbWorkspace)

		displayHandle := bbUserHandle
		if displayHandle == "" {
			displayHandle = bbWorkspace
		}

		bbStats, err := bitbucketProvider.Fetch(ctx, displayHandle)
		if err != nil {
			log.Printf("warning: provider %s failed: %v", bitbucketProvider.Name(), err)
		} else {
			stats = core.MergeStats(githubStats, bbStats)
		}
	} else {
		log.Printf("info: Bitbucket env vars not set or incomplete; skipping Bitbucket provider")
	}

	svg, err := render.RenderSVG(stats)
	if err != nil {
		log.Fatalf("failed to render SVG: %v", err)
	}

	if err := os.WriteFile(output, svg, 0o644); err != nil {
		log.Fatalf("failed to write SVG to %s: %v", output, err)
	}

	fmt.Printf("devmetrics: generated %s for user %q via %s provider\n", output, user, githubProvider.Name())
}
