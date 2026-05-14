package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shinagawa-web/pgincident/internal/config"
	"github.com/shinagawa-web/pgincident/internal/core"
	"github.com/shinagawa-web/pgincident/internal/tui"
	"github.com/shinagawa-web/pgincident/internal/version"
)

const helpText = `Usage: pgincident [options]

Options:
  --config PATH   Config file path (default: ~/.pgincident.toml)
  -v, --version   Print version and exit
  -h, --help      Show this help
`

// errHelp and errVersion signal clean exits without triggering the error path.
var errHelp = fmt.Errorf("help")
var errVersion = fmt.Errorf("version")

// parseFlags returns the config path extracted from args, or errHelp/errVersion
// for -h/--help and -v/--version respectively.
func parseFlags(args []string) (cfgPath string, err error) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--config":
			i++
			if i >= len(args) {
				return "", fmt.Errorf("--config requires an argument")
			}
			cfgPath = args[i]
		case strings.HasPrefix(arg, "--config="):
			cfgPath = strings.TrimPrefix(arg, "--config=")
		case arg == "-v" || arg == "--version":
			return "", errVersion
		case arg == "-h" || arg == "--help":
			return "", errHelp
		default:
			return "", fmt.Errorf("unknown flag: %s\n\n%s", arg, helpText)
		}
	}
	return cfgPath, nil
}

func main() {
	cfgPath, err := parseFlags(os.Args[1:])
	if err == errVersion {
		fmt.Printf("pgincident v%s\n", version.Version)
		os.Exit(0)
	}
	if err == errHelp {
		fmt.Print(helpText)
		os.Exit(0)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if cfgPath == "" {
		cfgPath = config.DefaultPath()
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if cfg.DSN == "" {
		fmt.Fprintf(os.Stderr, "error: no DSN configured — set 'dsn' in %s\n", cfgPath)
		os.Exit(1)
	}

	ctx := context.Background()

	client, err := core.Connect(ctx, cfg.DSN)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer client.Close(ctx)

	poller := core.NewPoller(client, 5*time.Second)
	poller.LongRunningThreshold = cfg.Thresholds.LongRunning.TimeDuration()
	poller.IdleInTxThreshold = cfg.Thresholds.IdleInTx.TimeDuration()

	app := tui.New(poller)

	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
