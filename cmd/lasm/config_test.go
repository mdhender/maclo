// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The precedence rules used to belong to peterbourgon/ff, and were covered by
// that library's tests rather than by any here. Taking the dependency out
// moved the rules into this package, so the tests come with them: a flag beats
// the environment, which beats the config file, and nothing quietly disagrees.

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "lasm.json")
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

// TestConfigFlagsOnly is the ordinary case, and the one that has to keep
// working whatever the rest of this does.
func TestConfigFlagsOnly(t *testing.T) {
	cfg, err := getConfig([]string{"-source", "a.lowl", "-test-scanner"})
	if err != nil {
		t.Fatalf("getConfig: %v", err)
	}
	if cfg.sourcefile != "a.lowl" {
		t.Errorf("source: want a.lowl, got %q", cfg.sourcefile)
	}
	if !cfg.test.scanner {
		t.Errorf("test-scanner: want true")
	}
	if cfg.debug {
		t.Errorf("debug: want false, nothing set it")
	}

	// -source is the one required flag
	if _, err := getConfig([]string{"-debug"}); err == nil {
		t.Errorf("no -source: want an error, got nil")
	}
}

// TestConfigFromEnvironment covers the LASM_ prefix, including the name
// mangling that a hyphenated flag needs.
func TestConfigFromEnvironment(t *testing.T) {
	t.Setenv("LASM_SOURCE", "from-env.lowl")
	t.Setenv("LASM_TEST_SCANNER", "true")

	cfg, err := getConfig(nil)
	if err != nil {
		t.Fatalf("getConfig: %v", err)
	}
	if cfg.sourcefile != "from-env.lowl" {
		t.Errorf("source: want from-env.lowl, got %q", cfg.sourcefile)
	}
	if !cfg.test.scanner {
		t.Errorf("test-scanner: want true from LASM_TEST_SCANNER")
	}

	// a value the flag cannot parse is an error that names the variable,
	// rather than a silently ignored setting
	t.Setenv("LASM_DEBUG", "yes-please")
	if _, err := getConfig(nil); err == nil {
		t.Errorf("unparseable env value: want an error, got nil")
	} else if !strings.Contains(err.Error(), "LASM_DEBUG") {
		t.Errorf("want the error to name the variable, got %v", err)
	}
}

// TestConfigFromFile covers the JSON file, and that a key which is not a flag
// stops the program. A typo that silently does nothing is worse than one that
// fails.
func TestConfigFromFile(t *testing.T) {
	path := writeConfig(t, `{"source": "from-file.lowl", "debug": true}`)

	cfg, err := getConfig([]string{"-config", path})
	if err != nil {
		t.Fatalf("getConfig: %v", err)
	}
	if cfg.sourcefile != "from-file.lowl" {
		t.Errorf("source: want from-file.lowl, got %q", cfg.sourcefile)
	}
	if !cfg.debug {
		t.Errorf("debug: want true from the config file")
	}

	bad := writeConfig(t, `{"source": "a.lowl", "sorce": "typo"}`)
	if _, err := getConfig([]string{"-config", bad}); err == nil {
		t.Errorf("unknown key: want an error, got nil")
	} else if !strings.Contains(err.Error(), "sorce") {
		t.Errorf("want the error to name the key, got %v", err)
	}

	// a value with no sensible flag rendering is refused rather than
	// stringified into something surprising
	odd := writeConfig(t, `{"source": {"nested": 1}}`)
	if _, err := getConfig([]string{"-config", odd}); err == nil {
		t.Errorf("object as a flag value: want an error, got nil")
	}

	if _, err := getConfig([]string{"-config", filepath.Join(t.TempDir(), "nope.json")}); err == nil {
		t.Errorf("missing config file: want an error, got nil")
	}
}

// TestConfigPrecedence is the whole point of keeping all three sources: the
// order between them. Each pair is checked with the losing source set to
// something the winner is not, so a rule that silently reversed would show up
// as a wrong value rather than as an equal one.
func TestConfigPrecedence(t *testing.T) {
	path := writeConfig(t, `{"source": "from-file.lowl", "debug": true}`)

	// flag beats config file
	cfg, err := getConfig([]string{"-config", path, "-source", "from-flag.lowl"})
	if err != nil {
		t.Fatalf("getConfig: %v", err)
	}
	if cfg.sourcefile != "from-flag.lowl" {
		t.Errorf("flag over file: want from-flag.lowl, got %q", cfg.sourcefile)
	}

	// environment beats config file
	t.Setenv("LASM_SOURCE", "from-env.lowl")
	cfg, err = getConfig([]string{"-config", path})
	if err != nil {
		t.Fatalf("getConfig: %v", err)
	}
	if cfg.sourcefile != "from-env.lowl" {
		t.Errorf("env over file: want from-env.lowl, got %q", cfg.sourcefile)
	}
	// and the file still supplies what the environment did not
	if !cfg.debug {
		t.Errorf("env over file: debug should still come from the file")
	}

	// flag beats environment
	cfg, err = getConfig([]string{"-source", "from-flag.lowl"})
	if err != nil {
		t.Fatalf("getConfig: %v", err)
	}
	if cfg.sourcefile != "from-flag.lowl" {
		t.Errorf("flag over env: want from-flag.lowl, got %q", cfg.sourcefile)
	}

	// all three at once, which is the case the accumulating `settled` map
	// exists for: a flag the environment supplied must be off limits to the
	// file as well
	cfg, err = getConfig([]string{"-config", path, "-test-scanner"})
	if err != nil {
		t.Fatalf("getConfig: %v", err)
	}
	if cfg.sourcefile != "from-env.lowl" || !cfg.debug || !cfg.test.scanner {
		t.Errorf("all three: want env source, file debug, flag scanner; got %q %v %v",
			cfg.sourcefile, cfg.debug, cfg.test.scanner)
	}
}
