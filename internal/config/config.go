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

// Config is the top-level structure for ~/.pgincident.toml.
type Config struct {
	DSN        string     `toml:"dsn"`
	Thresholds Thresholds `toml:"thresholds"`
}

// DefaultPath returns the default config file path (~/.pgincident.toml).
func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".pgincident.toml")
}

// Load reads and parses the TOML config file at path.
// Threshold fields not present in the file keep their default values (5s / 30s).
func Load(path string) (*Config, error) {
	cfg := &Config{
		Thresholds: Thresholds{
			LongRunning: Duration(5 * time.Second),
			IdleInTx:    Duration(30 * time.Second),
		},
	}
	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	return cfg, nil
}
