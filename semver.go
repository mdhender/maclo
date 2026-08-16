// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

// Package maclo is the root of a Go port of ML/I, a general purpose macro
// processor. The processor itself is pkg/ml1 and the two front ends are
// cmd/maclo and cmd/ml1; this package holds only what belongs to the
// distribution as a whole.
package maclo

// version is what this port reports for itself.
//
// It is a var rather than a const so that a release build can stamp a tag into
// it without editing the source:
//
//	go build -ldflags "-X github.com/mdhender/maclo.version=0.1.0" ./cmd/maclo
//
// Nothing is tagged yet, and the default says so rather than claiming a
// release that has not happened.
var version = "0.1.0-dev"

// Version returns the version of this implementation of ML/I.
//
// This is the version of the port and not of ML/I. ML/I's own version is a
// property of the LOWL source the engine runs rather than of this program —
// the source distributed at ml1.org.uk is AJB, and the golden files here were
// recorded from a CKQ build, which is the version skew documented in
// docs/explanation/running-ml1-on-the-lowl-vm.md.
func Version() string {
	return version
}
