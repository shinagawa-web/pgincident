package config

import (
	"fmt"
	"testing"
)

func TestDefaultPathHomeError(t *testing.T) {
	orig := userHomeDirFn
	userHomeDirFn = func() (string, error) { return "", fmt.Errorf("no home") }
	defer func() { userHomeDirFn = orig }()

	_, err := DefaultPath()
	if err == nil {
		t.Fatal("expected error when home directory lookup fails")
	}
}
