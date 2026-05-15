package cli_test

import (
	"testing"

	"github.com/shinagawa-web/pgincident/internal/cli"
)

func TestParseFlagsConfig(t *testing.T) {
	path, err := cli.ParseFlags([]string{"--config", "/tmp/my.toml"})
	if err != nil {
		t.Fatal(err)
	}
	if path != "/tmp/my.toml" {
		t.Errorf("cfgPath = %q, want /tmp/my.toml", path)
	}
}

func TestParseFlagsConfigEquals(t *testing.T) {
	path, err := cli.ParseFlags([]string{"--config=/tmp/my.toml"})
	if err != nil {
		t.Fatal(err)
	}
	if path != "/tmp/my.toml" {
		t.Errorf("cfgPath = %q, want /tmp/my.toml", path)
	}
}

func TestParseFlagsConfigMissingArg(t *testing.T) {
	_, err := cli.ParseFlags([]string{"--config"})
	if err == nil {
		t.Error("expected error for missing --config argument, got nil")
	}
}

func TestParseFlagsConfigEqualsEmpty(t *testing.T) {
	_, err := cli.ParseFlags([]string{"--config="})
	if err == nil {
		t.Error("expected error for --config= with no value, got nil")
	}
}

func TestParseFlagsVersion(t *testing.T) {
	_, err := cli.ParseFlags([]string{"-v"})
	if err != cli.ErrVersion {
		t.Errorf("err = %v, want ErrVersion", err)
	}
	_, err = cli.ParseFlags([]string{"--version"})
	if err != cli.ErrVersion {
		t.Errorf("err = %v, want ErrVersion", err)
	}
}

func TestParseFlagsHelp(t *testing.T) {
	_, err := cli.ParseFlags([]string{"-h"})
	if err != cli.ErrHelp {
		t.Errorf("err = %v, want ErrHelp", err)
	}
	_, err = cli.ParseFlags([]string{"--help"})
	if err != cli.ErrHelp {
		t.Errorf("err = %v, want ErrHelp", err)
	}
}

func TestParseFlagsUnknown(t *testing.T) {
	_, err := cli.ParseFlags([]string{"--unknown"})
	if err == nil {
		t.Error("expected error for unknown flag, got nil")
	}
}

func TestParseFlagsEmpty(t *testing.T) {
	path, err := cli.ParseFlags([]string{})
	if err != nil {
		t.Fatal(err)
	}
	if path != "" {
		t.Errorf("cfgPath = %q, want empty", path)
	}
}
