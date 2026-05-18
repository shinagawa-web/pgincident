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

// Config is the top-level structure for ~/.pgincident.toml.
type Config struct {
	DSN        string     `toml:"dsn"`
	Thresholds Thresholds `toml:"thresholds"`
}

var defaultThresholds = Thresholds{
	LongRunning: Duration(5 * time.Second),
	IdleInTx:    Duration(30 * time.Second),
}

// DefaultTOML returns the TOML template written by --init.
func DefaultTOML() string {
	return fmt.Sprintf(
		"dsn = \"postgres://user:password@localhost:5432/mydb\"\n\n[thresholds]\nlong_running        = \"%s\"\nidle_in_transaction = \"%s\"\n",
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
		return nil, fmt.Errorf("config: unknown keys: %v", keys)
	}
	return cfg, nil
}
