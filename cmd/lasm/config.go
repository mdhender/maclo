// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2023 Michael D Henderson.
// All rights reserved.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// The command line, and the two places a flag may come from instead.
//
// This used to be peterbourgon/ff, which is a good library and was the only
// dependency in the module. It bought three behaviours — flags, a LASM_
// environment prefix, and a JSON config file — of which the first is in the
// standard library and the other two are the forty lines below. Dropping it
// makes the module standard-library-only, which is worth more to a port of a
// 1960s macro processor than the forty lines cost.
//
// Precedence is the one ff used and the one people expect: an explicit flag
// beats the environment, which beats the config file. That order is why the
// command line is parsed first — it is also what names the config file.

// envPrefix turns a flag name into the environment variable that can supply
// it: -test-scanner is LASM_TEST_SCANNER.
const envPrefix = "LASM"

type config struct {
	debug      bool
	sourcefile string
	test       struct {
		astParser bool
		cstParser bool
		scanner   bool
	}
}

func getConfig(args []string) (*config, error) {
	cfg := &config{}

	fs := flag.NewFlagSet("lasm", flag.ContinueOnError)
	configFile := fs.String("config", "", "config file (optional, json)")
	fs.StringVar(&cfg.sourcefile, "source", cfg.sourcefile, "assembly source file (required)")
	fs.BoolVar(&cfg.test.scanner, "test-scanner", cfg.test.scanner, "test scanner, then exit")
	fs.BoolVar(&cfg.test.cstParser, "test-cst-parser", cfg.test.cstParser, "test cst parser, then exit")
	fs.BoolVar(&cfg.test.astParser, "test-ast-parser", cfg.test.astParser, "test ast parser, then exit")
	fs.BoolVar(&cfg.debug, "debug", cfg.debug, "log debug information (optional)")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	// Which flags are already answered for. Visit walks only the ones Parse
	// set, which is what makes "an explicit flag wins" possible at all — a
	// flag left at its default is otherwise indistinguishable from one given
	// that value on the command line.
	//
	// It then accumulates rather than being rebuilt, because a flag the
	// environment supplied has to be off limits to the config file too, and
	// fs.Visit cannot say so: it reports what Parse set, and a later fs.Set
	// looks identical to it whoever called it.
	settled := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { settled[f.Name] = true })

	if err := applyEnv(fs, settled); err != nil {
		return nil, err
	}
	if err := applyConfigFile(fs, settled, *configFile); err != nil {
		return nil, err
	}

	if cfg.sourcefile == "" {
		return nil, fmt.Errorf("--source is required")
	}
	return cfg, nil
}

// applyEnv fills in flags the command line did not set from LASM_ variables,
// and records what it supplied so the config file cannot undo it.
func applyEnv(fs *flag.FlagSet, settled map[string]bool) error {
	var err error
	fs.VisitAll(func(f *flag.Flag) {
		if err != nil || settled[f.Name] {
			return
		}
		name := envPrefix + "_" + strings.ToUpper(strings.ReplaceAll(f.Name, "-", "_"))
		value, ok := os.LookupEnv(name)
		if !ok {
			return
		}
		if setErr := fs.Set(f.Name, value); setErr != nil {
			err = fmt.Errorf("%s: %w", name, setErr)
			return
		}
		settled[f.Name] = true
	})
	return err
}

// applyConfigFile fills in whatever is still unset from a JSON object.
//
// A key that is not a flag is an error rather than something ignored, which is
// what ff.WithIgnoreUndefined(false) asked for: a typo in a config file that
// silently does nothing is worse than one that stops the program.
func applyConfigFile(fs *flag.FlagSet, settled map[string]bool, path string) error {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	for key, value := range raw {
		if fs.Lookup(key) == nil {
			return fmt.Errorf("%s: %q is not a flag", path, key)
		}
		if settled[key] {
			continue // the command line or the environment said otherwise
		}
		text, err := literal(value)
		if err != nil {
			return fmt.Errorf("%s: %s: %w", path, key, err)
		}
		if err := fs.Set(key, text); err != nil {
			return fmt.Errorf("%s: %s: %w", path, key, err)
		}
	}
	return nil
}

// literal renders a JSON scalar the way flag.Value.Set expects it. Objects and
// arrays have no meaning for any flag here, so they are refused rather than
// stringified into something surprising.
func literal(value any) (string, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	case bool:
		return strconv.FormatBool(v), nil
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case nil:
		return "", fmt.Errorf("null has no value")
	}
	return "", fmt.Errorf("want a string, number, or boolean, got %T", value)
}
