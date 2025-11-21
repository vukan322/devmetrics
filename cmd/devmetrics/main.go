package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/vukan322/devmetrics/internal/providers/demo"
	"github.com/vukan322/devmetrics/internal/render"
)

func main() {
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

	provider := demo.New()

	stats, err := provider.Fetch(ctx, user)
	if err != nil {
		log.Fatalf("provider %s failed: %v", provider.Name(), err)
	}

	svg, err := render.RenderSVG(stats)
	if err != nil {
		log.Fatalf("failed to render SVG: %v", err)
	}

	if err := os.WriteFile(output, svg, 0o644); err != nil {
		log.Fatalf("failed to write SVG to %s: %v", output, err)
	}

	fmt.Printf("devmetrics: generated %s for user %q via %s provider\n", output, user, provider.Name())
}
