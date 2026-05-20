package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

// Duration wraps time.Duration to support TOML string parsing (e.g. "5s", "1m30s").
type Duration time.Duration

func (d *Duration) UnmarshalText(text []byte) error {
	dd, err := time.ParseDuration(string(text))
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", string(text), err)
	}
	if dd <= 0 {
		return fmt.Errorf("duration %q must be positive", string(text))
	}
	*d = Duration(dd)
	return nil
}

// TimeDuration returns the underlying time.Duration value.
func (d Duration) TimeDuration() time.Duration {
	return time.Duration(d)
}

// Thresholds holds query duration thresholds for incident detection.
type Thresholds struct {
	LongRunning Duration `toml:"long_running"`
	IdleInTx    Duration `toml:"idle_in_transaction"`
}

// Connection holds the DSN for a single named connection preset.
type Connection struct {
	DSN string `toml:"dsn"`
}

// Config is the top-level structure for ~/.pgincident.toml.
type Config struct {
	Connections     map[string]Connection `toml:"connections"`
	ConnectionOrder []string              `toml:"-"`
	Thresholds      Thresholds            `toml:"thresholds"`
}

var defaultThresholds = Thresholds{
	LongRunning: Duration(5 * time.Second),
	IdleInTx:    Duration(30 * time.Second),
}

// DefaultTOML returns the TOML template written by --init.
func DefaultTOML() string {
	return fmt.Sprintf(
		"[connections.default]\ndsn = \"postgres://user:password@localhost:5432/mydb\"\n\n[thresholds]\nlong_running        = \"%s\"\nidle_in_transaction = \"%s\"\n",
		defaultThresholds.LongRunning.TimeDuration(),
		defaultThresholds.IdleInTx.TimeDuration(),
	)
}

var userHomeDirFn = os.UserHomeDir

// DefaultPath returns the default config file path (~/.pgincident.toml).
func DefaultPath() (string, error) {
	home, err := userHomeDirFn()
	if err != nil {
		return "", fmt.Errorf("cannot locate home directory: %w", err)
	}
	return filepath.Join(home, ".pgincident.toml"), nil
}

// ResolvePath returns cfgPath if non-empty. Otherwise it returns
// .pgincident.toml in the current directory if the file exists, falling back
// to DefaultPath.
func ResolvePath(cfgPath string) (string, error) {
	if cfgPath != "" {
		return cfgPath, nil
	}
	if _, err := os.Stat(".pgincident.toml"); err == nil {
		return ".pgincident.toml", nil
	}
	return DefaultPath()
}

// Load reads and parses the TOML config file at path.
// Threshold fields not present in the file keep their default values (5s / 30s).
// Unknown keys and non-positive threshold durations are rejected.
func Load(path string) (*Config, error) {
	cfg := &Config{
		Thresholds: defaultThresholds,
	}
	md, err := toml.DecodeFile(path, cfg)
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	if keys := md.Undecoded(); len(keys) > 0 {
		for _, k := range keys {
			if len(k) == 1 && k[0] == "dsn" {
				return nil, fmt.Errorf("config: top-level 'dsn' is no longer supported; use [connections.<name>] instead (see .pgincident.example.toml)")
			}
		}
		return nil, fmt.Errorf("config: unknown keys: %v", keys)
	}
	if len(cfg.Connections) == 0 {
		return nil, fmt.Errorf("config: no connections defined — add a [connections.<name>] section")
	}
	for name, conn := range cfg.Connections {
		if conn.DSN == "" {
			return nil, fmt.Errorf("config: connection %q has no dsn", name)
		}
	}
	// Populate ConnectionOrder from key parse order (md.Keys preserves file order).
	seen := make(map[string]bool, len(cfg.Connections))
	for _, k := range md.Keys() {
		if len(k) == 2 && k[0] == "connections" {
			name := k[1]
			if !seen[name] {
				cfg.ConnectionOrder = append(cfg.ConnectionOrder, name)
				seen[name] = true
			}
		}
	}
	return cfg, nil
}
