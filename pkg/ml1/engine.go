// ml_i - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package ml1

import (
	"os"
	"path/filepath"
	"runtime"
)

// Where the engine lives.
//
// ML/I is distributed as source written in LOWL and this package runs that
// source rather than a translation of it, so the processor is a file that has
// to be on the machine before anything can be processed. Its licence forbids
// redistributing a machine readable copy, which means it cannot be committed
// here and cannot be embedded in the binary either: it is a runtime
// dependency, and where it is found is part of this package's contract rather
// than a detail of the command.
//
// That is also what lets a newer version be dropped in without a code change.

const (
	// EngineFile is the name of the LOWL source of ML/I, as the archive from
	// ml1.org.uk contains it.
	EngineFile = "ml1ajb.lwl"

	// EngineEnv names a LOWL source directly, overriding every search.
	EngineEnv = "ML1_LOWL_SOURCE"

	// HomeEnv overrides the per-user directory the engine is kept in.
	HomeEnv = "ML1_HOME"
)

// EngineDir returns the per-user directory the engine is kept in, which is
// where an installed binary looks for it and where a bootstrap writes it.
//
// $ML1_HOME wins outright. Otherwise this follows the platform: macOS and
// Windows keep application data where os.UserConfigDir points, and elsewhere
// the engine is data rather than configuration, so it goes under
// $XDG_DATA_HOME — ~/.local/share by default — as the base directory
// specification asks.
//
// It returns "" if there is no home directory to work from, which is a machine
// where the engine has to be named explicitly instead.
func EngineDir() string {
	if dir := os.Getenv(HomeEnv); dir != "" {
		return dir
	}
	if runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
			return filepath.Join(dir, "ml1")
		}
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, ".local", "share", "ml1")
		}
		return ""
	}
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "ml1")
	}
	return ""
}

// EnginePaths returns every place a LOWL source is looked for when a Job does
// not name one, in the order they are tried.
//
// The first two are for an installed binary and the last two for a checkout,
// which is why both are here: the same call has to work from a developer's
// working directory and from wherever a user happens to be standing.
func EnginePaths() []string {
	var paths []string
	if p := os.Getenv(EngineEnv); p != "" {
		paths = append(paths, p)
	}
	if dir := EngineDir(); dir != "" {
		paths = append(paths, filepath.Join(dir, EngineFile))
	}
	// the repository layout, relative to the working directory: where
	// cmd/fetchtestdata puts the engine for the tests
	paths = append(paths,
		filepath.Join(".downloads", "lowlml1", EngineFile),
		filepath.Join(".references", "ml1aih.lwl"),
	)
	return paths
}
