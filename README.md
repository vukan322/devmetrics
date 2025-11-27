# DevMetrics

Multi-provider developer activity card generator that aggregates your coding stats from GitHub, GitLab, and Bitbucket into a single SVG card.

## Installation
```bash
git clone https://github.com/vukan322/devmetrics.git
cd devmetrics
```

## Usage

Set your GitHub token:
```bash
export GITHUB_TOKEN=your_token_here
```

Generate your dev card:
```bash
go run cmd/devmetrics/main.go -user yourusername -out devmetrics.svg
```

### Options

- `-user` - Your username (required)
- `-out` - Output file path (default: `devmetrics.svg`)

## License

MIT License - see LICENSE file for details

## Contributing

Contributions welcome! Feel free to open issues or submit pull requests.
