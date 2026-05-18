package cli

import (
	"fmt"
	"strings"
)

const HelpText = `Usage: pgincident [options]

Options:
  --config PATH   Config file path (default: .pgincident.toml in current dir,
                  then ~/.pgincident.toml)
  --init          Create ~/.pgincident.toml with default values and exit
  -v, --version   Print version and exit
  -h, --help      Show this help
`

var ErrHelp = fmt.Errorf("help")
var ErrVersion = fmt.Errorf("version")
var ErrInit = fmt.Errorf("init")

func ParseFlags(args []string) (cfgPath string, err error) {
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
			if cfgPath == "" {
				return "", fmt.Errorf("--config requires an argument")
			}
		case arg == "--init":
			return "", ErrInit
		case arg == "-v" || arg == "--version":
			return "", ErrVersion
		case arg == "-h" || arg == "--help":
			return "", ErrHelp
		default:
			return "", fmt.Errorf("unknown flag: %s\n\n%s", arg, HelpText)
		}
	}
	return cfgPath, nil
}
