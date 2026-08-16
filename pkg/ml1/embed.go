// ml_i - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package ml1

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// The engines built into this binary.
//
// ML/I is distributed as LOWL source and this package runs that source rather
// than a translation of it. The licence on it permits building it into a
// program and forbids redistributing either the source or the program, so the
// source can be embedded but cannot be committed here — engines/.gitignore
// denies everything except itself and a README, and those two are what make
// the directory exist for //go:embed to find.
//
// The consequence worth stating plainly: how many engines a binary has is a
// property of the machine it was built on. A clone has none, and so does
// anything `go install` builds from the proxy. Engines() is the only honest
// answer to what a given binary can run, which is why maclo prints it.

//go:embed engines
var engineFS embed.FS

// engineDir is the directory inside engineFS. //go:embed keeps the path.
const engineDir = "engines"

// Engine is one LOWL source built into this binary.
type Engine struct {
	// Name identifies the engine to a user and on a command line: the file
	// name with its extension removed, such as "ml1ajb".
	Name string

	// Version is the three letter version ML/I calls itself by, uppercased
	// from the tail of Name — "AJB" — or empty when Name does not follow the
	// convention.
	Version string

	// Size is the length of the source in bytes.
	Size int64
}

// Engines returns the engines built into this binary, newest first.
//
// "Newest" is decided by the file name, because that is where ML/I's version
// lives: ml1aih precedes ml1ajb. A name that does not follow the convention
// still sorts, just not meaningfully against the ones that do, so a binary
// carrying an oddly named source is odd rather than broken.
//
// The result is empty, not nil-and-an-error, when nothing was embedded. That
// is a legitimate build.
func Engines() []Engine {
	entries, err := fs.ReadDir(engineFS, engineDir)
	if err != nil {
		return nil
	}
	var found []Engine
	for _, e := range entries {
		if e.IsDir() || path.Ext(e.Name()) != ".lwl" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".lwl")
		found = append(found, Engine{
			Name:    name,
			Version: engineVersion(name),
			Size:    info.Size(),
		})
	}
	sort.Slice(found, func(i, j int) bool { return found[i].Name > found[j].Name })
	return found
}

// engineVersion pulls ML/I's version letters off the end of a source name.
// The convention is ml1 followed by three letters; anything else gets no
// version rather than a guess at one.
func engineVersion(name string) string {
	rest, ok := strings.CutPrefix(strings.ToLower(name), "ml1")
	if !ok || len(rest) != 3 {
		return ""
	}
	for _, r := range rest {
		if r < 'a' || r > 'z' {
			return ""
		}
	}
	return strings.ToUpper(rest)
}

// DefaultEngine returns the name of the engine that runs when none is chosen,
// which is the newest embedded one. It returns "" when none was embedded.
func DefaultEngine() string {
	if all := Engines(); len(all) != 0 {
		return all[0].Name
	}
	return ""
}

// HasEngine reports whether the named engine is built into this binary.
//
// It is what separates "run the AJB I was built with" from "run the file at
// this path": a caller that accepts either checks here first and treats
// anything else as a path.
func HasEngine(name string) bool {
	for _, e := range Engines() {
		if e.Name == name {
			return true
		}
	}
	return false
}

// engineSource returns the embedded source of the named engine.
func engineSource(name string) ([]byte, error) {
	if name == "" {
		return nil, fmt.Errorf("no engine named: %w", ErrNoEngineSource)
	}
	b, err := fs.ReadFile(engineFS, path.Join(engineDir, name+".lwl"))
	if err != nil {
		return nil, fmt.Errorf("%s is not built into this binary: %w", name, ErrNoEngineSource)
	}
	return b, nil
}
