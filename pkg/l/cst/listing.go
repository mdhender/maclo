// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package cst

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/mdhender/maclo/pkg/l/stmt"
	"github.com/mdhender/maclo/pkg/l/token"
)

// WriteListing dumps one line per source line: what the parser made of it, and
// what it could not. It takes an io.Writer rather than naming a file, so no
// artifact name exists under pkg/ and debug_artifacts_test.go has nothing new
// to declare.
func WriteListing(w io.Writer, f *File) error {
	bw := bufio.NewWriter(w)
	for _, line := range f.Lines {
		for _, t := range line.Lead {
			fmt.Fprintf(bw, "%5d | %s\n", t.Pos.Line, triviaText(t))
		}
		fmt.Fprintf(bw, "%5d | %s\n", line.Pos.Line, line.String())
		for _, e := range line.Errs {
			fmt.Fprintf(bw, "      ! %s %s\n", e.Pos, e.Msg)
		}
	}
	for _, t := range f.Tail {
		fmt.Fprintf(bw, "%5d | %s\n", t.Pos.Line, triviaText(t))
	}
	return bw.Flush()
}

// String renders the line the way it was written, near enough to read against
// the source.
func (l *Line) String() string {
	var sb strings.Builder
	if l.Label != nil {
		sb.WriteString("[" + l.Label.Text + "] ")
	}
	for _, p := range l.Prefixes {
		sb.WriteString("/-" + p.Text + "-/ ")
	}
	if l.Head == stmt.Unknown {
		sb.WriteString("?")
	} else {
		sb.WriteString(l.Head.String())
	}
	if len(l.Args) > 0 {
		if k := l.Args[0].Kind; k != ArgGroup && k != ArgText {
			sb.WriteByte(' ')
		}
		sb.WriteString(ArgsText(l.Args))
	}
	return sb.String()
}

// ArgsText renders an argument list. Adjacent items are separated by a space
// except where the source could not have had one, which keeps OF(LCH) from
// coming back as OF ( LCH ).
func ArgsText(args []*Arg) string {
	var sb strings.Builder
	for i, a := range args {
		if i > 0 && needsSpace(args[i-1], a) {
			sb.WriteByte(' ')
		}
		sb.WriteString(a.String())
	}
	return sb.String()
}

// String renders one argument node.
func (a *Arg) String() string {
	switch a.Kind {
	case ArgQuote:
		return "'" + a.Raw + "'"
	case ArgText:
		return "[" + a.Raw + "]"
	case ArgComment:
		return "//" + a.Raw + "//"
	case ArgPrefix:
		return "/-" + a.Text + "-/"
	case ArgGroup:
		return "(" + ArgsText(a.Children) + ")"
	}
	return a.Raw
}

// needsSpace decides how to separate two adjacent arguments.
//
// This listing is a debug dump, not the canonical form - that is the ast's
// job - so the rule is chosen to be unambiguous rather than to reproduce the
// source byte for byte. Groups and bracket text bind tight to what precedes
// them, arithmetic binds tight on both sides, and everything else is spaced.
//
// One consequence is worth keeping: IND(SPT)PT comes back as "IND(SPT) PT",
// because at this stage PT is just the next word and nothing has yet decided
// it is a type suffix. Seeing that in the dump is the point.
func needsSpace(prev, cur *Arg) bool {
	if cur.Kind == ArgGroup || cur.Kind == ArgText {
		return false
	}
	if cur.Kind == ArgPunct && cur.Punct == token.Comma {
		return false // a comma takes no space before it, and one after
	}
	if isTightPunct(prev) || isTightPunct(cur) {
		return false
	}
	return true
}

// isTightPunct reports whether an argument is punctuation written without
// surrounding spaces: the arithmetic operators, and a comma, which takes no
// space before it.
func isTightPunct(a *Arg) bool {
	if a.Kind != ArgPunct {
		return false
	}
	switch a.Punct {
	case token.Plus, token.Minus, token.Star, token.Slash:
		return true
	}
	return false
}

func triviaText(t token.Token) string {
	if t.Kind == token.Heading {
		return "/+" + t.Raw + "+/"
	}
	return "//" + t.Raw + "//"
}
