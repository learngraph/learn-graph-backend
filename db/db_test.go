package db

import (
	"os"
	"testing"
)

func TestGetEnvConfig(t *testing.T) {
	os.Setenv("DB_ARANGO_USER", "ab")
	os.Setenv("DB_ARANGO_PASSWORD", "cd")
	conf := GetEnvConfig()
	if conf.User != "ab" {
		t.Errorf("want 'ab', got '%s'", conf.User)
	}
	if conf.Password != "cd" {
		t.Errorf("want 'cd', got '%s'", conf.Password)
	}
}
