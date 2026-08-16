// ml_i - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package maclo_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The debug artifacts, and the rule that keeps them out of a library call.
//
// Every stage of the LOWL pipeline can write a listing, and each one goes to
// the *current directory* because the program they were written for is
// cmd/lasm. pkg/ml1/silence_test.go checks the consequence — that ml1.Run
// leaves an empty directory empty — by running the engine and looking. That is
// the assertion that matters, and it has one blind spot: it only sees a write
// on a path it exercises. A stage that wrote its listing from somewhere the
// four jobs there do not reach would still be a library call writing files into
// a directory it does not own.
//
// So this reads the source instead. It is at the top of the repository rather
// than inside a package because what it says is about the whole of it: which
// functions may create a file, which file names the tooling chooses for itself,
// and the fact that every one of those names is gitignored. None of that
// belongs to one package.
//
// The three rules, and why each is worth a test rather than a habit:
//
//  1. Only the functions in writeSites may create a file. Anything under pkg
//     or internal is a library, and a library that writes a file nobody asked
//     for is the defect this whole file exists for. internal is held to the
//     same rule as pkg on purpose: a package that only had to move one
//     directory to escape the check would not be checked at all.
//  2. A file name written as a literal in the source is a debug artifact by
//     definition — a real output file is named by the caller — so every one of
//     them is listed here, with what writes it and what turns it on.
//  3. Every artifact is in .gitignore. That rule earned itself: until d417033
//     assembler.Assemble took no Options and wrote its listings on every call,
//     so running its tests dropped asm_listing.txt and asm_symtab.txt into
//     pkg/lowl/assembler — where both were still sitting when this was written,
//     invisible to git status because someone had named them. scanner_buffer.txt
//     had not been named, and the same accident would have offered it for
//     commit.

// debugArtifacts is every file name the tooling writes for a developer to
// read, mapped to what writes it and what asks for it.
//
// A name here is expected to appear as a literal in the source and to be
// matched by .gitignore. Both directions are checked: a new artifact that is
// not listed fails, and a listed one that has disappeared fails too, so the
// table cannot rot into a description of what used to happen.
var debugArtifacts = map[string]string{
	"scanner_buffer.txt": "scanner.TestBuffer, via cst.Parse for lasm --test-buffer",
	"scanner_tokens.txt": "scanner.TestScanner, via cst.Parse for lasm --test-scanner",
	"ast_listing.txt":    "ast.Nodes.Listing, named by lasm",
	"asm_listing.txt":    "assembler.Listing, named by assembler.Assemble under Options.Listings",
	"asm_symtab.txt":     "assembler.Assemble, under Options.Listings",
	"vm_stdout.txt":      "lasm itself, holding what the machine wrote to the results stream",
	"vm_stdmsg.txt":      "lasm itself, holding what the machine wrote to the messages stream",
}

// writeSites are the functions under pkg and internal that may create a file,
// mapped to the reason they are allowed to.
//
// The pkg/lowl entries are all reached only from cmd/lasm. The library entry
// points beside them — cst.ParseBuffer, assembler.Options{} with Listings
// unset, a nil vm.Streams.Trace — are what ml1.Run uses instead, and pkg/ml1 is
// absent from this table on purpose: the processor writes to the io.Writers its
// Job names and to nothing else.
//
// internal/fetch is the one entry of a different kind, and it is worth being
// explicit about why it is not a violation. The rule this file enforces is
// against a library writing a file *nobody asked for*; installing the engine is
// the whole of what its caller asked for, and the directory is named by that
// caller rather than chosen here. What still applies to it, and is checked
// below like anything else, is that it must not print: cmd/ml1 runs the
// processor on the standard output, so a fetcher reaching for os.Stdout would
// corrupt the very stream the program exists to write.
var writeSites = map[string]string{
	"pkg/lowl/scanner.(*Scanner).TestBuffer":  "scanner_buffer.txt, for lasm --test-buffer",
	"pkg/lowl/scanner.(*Scanner).TestScanner": "scanner_tokens.txt, for lasm --test-scanner",
	"pkg/lowl/ast.Nodes.Listing":              "the file lasm names, ast_listing.txt",
	"pkg/lowl/assembler.Listing":              "the file Assemble names, asm_listing.txt",
	"pkg/lowl/assembler.Assemble":             "asm_symtab.txt, under Options.Listings",
	"pkg/lowl/vm.(*VM).Disassemble":           "the file its caller names",
	"internal/fetch.(*Archive).Install":       "the engine and the test suite, into the directory its caller names",
	"internal/fetch.(*Archive).bytes":         "the archive cache, at Options.Cache",
	"internal/fetch.InstallEngines":           "the LOWL sources //go:embed compiles in, into the directory its caller names",
}

// fileCreators are the calls that can put a file on the disk. os.Open is not
// among them: reading is not the problem.
var fileCreators = map[string]bool{
	"os.Create":    true,
	"os.WriteFile": true,
	"os.OpenFile":  true,
	"os.Mkdir":     true,
	"os.MkdirAll":  true,
}

// processStreams are the writers a library must not reach for. Code under pkg
// and internal takes an io.Writer instead; there is no allowlist for these
// because nothing outside cmd has ever needed one.
var processStreams = map[string]bool{
	"os.Stdout": true,
	"os.Stderr": true,
	"os.Stdin":  true,
}

// artifactName matches a file name the source chose for itself. A name a
// caller supplies never appears as a literal, which is what makes the pattern
// a sound way to find the artifacts rather than a guess at them.
var artifactName = regexp.MustCompile(`^[a-z0-9_]+\.(txt|lst|log)$`)

// TestDebugArtifactsAreDeclared reads every non-test source file under pkg,
// cmd and internal and holds it to the three rules above.
func TestDebugArtifactsAreDeclared(t *testing.T) {
	fset := token.NewFileSet()
	found := map[string]bool{} // artifact names seen in the source

	for _, root := range []string{"pkg", "cmd", "internal"} {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatalf("%s: %v\n", path, err)
			}
			pkgPath := filepath.ToSlash(filepath.Dir(path))
			// cmd is where a program is allowed to be a program; everything
			// else in the module is something another package calls
			isLibrary := root != "cmd"

			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok {
					continue
				}
				site := pkgPath + "." + funcName(fn)
				ast.Inspect(fn, func(n ast.Node) bool {
					switch node := n.(type) {
					case *ast.CallExpr:
						name := qualified(node.Fun)
						if fileCreators[name] && isLibrary && writeSites[site] == "" {
							t.Errorf("%s: %s calls %s\n"+
								"\tanything outside cmd is a library: if this is reachable from ml1.Run it must\n"+
								"\tnot exist, and if the file is one its caller asked for or an opt-in artifact\n"+
								"\tfor lasm, add %s to writeSites with the reason\n",
								position(fset, node.Pos()), site, name, site)
						}
					case *ast.SelectorExpr:
						if name := qualified(node); processStreams[name] && isLibrary {
							t.Errorf("%s: %s uses %s\n"+
								"\ta library must write to the io.Writer it was given, not to the process's streams\n",
								position(fset, node.Pos()), site, name)
						}
					case *ast.BasicLit:
						if node.Kind != token.STRING {
							return true
						}
						value, err := strconv.Unquote(node.Value)
						if err != nil || !artifactName.MatchString(value) {
							return true
						}
						found[value] = true
						if debugArtifacts[value] == "" {
							t.Errorf("%s: %s writes %q\n"+
								"\tthat is a file name the tooling chose for itself, so it is a debug artifact:\n"+
								"\tadd it to debugArtifacts here and to the block in .gitignore\n",
								position(fset, node.Pos()), site, value)
						}
					}
					return true
				})
			}
			return nil
		})
		if err != nil {
			t.Fatalf("%s: %v\n", root, err)
		}
	}

	for _, name := range sorted(debugArtifacts) {
		if !found[name] {
			t.Errorf("%s: nothing writes it any more (%s)\n"+
				"\tdrop it from debugArtifacts, and from .gitignore if nothing else claims the name\n",
				name, debugArtifacts[name])
		}
	}
	checkGitignore(t, debugArtifacts)
}

// checkGitignore requires every artifact to be ignored by name.
//
// The patterns are read rather than run through git, so that the test says the
// same thing in a checkout that has no git available. They are plain names,
// which is the whole of the syntax this repository uses for them.
func checkGitignore(t *testing.T, artifacts map[string]string) {
	t.Helper()
	data, err := os.ReadFile(".gitignore")
	if err != nil {
		t.Fatalf(".gitignore: %v\n", err)
	}
	ignored := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			ignored[line] = true
		}
	}
	for _, name := range sorted(artifacts) {
		if !ignored[name] {
			t.Errorf(".gitignore: %s is not ignored (%s)\n"+
				"\tit lands in whatever directory the tooling was run from, which can be a package\n"+
				"\tdirectory during a test; add it to the debug artifacts block\n", name, artifacts[name])
		}
	}
}

// funcName renders a declaration the way a reader would name it, with the
// receiver when it has one.
func funcName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	return typeName(fn.Recv.List[0].Type) + "." + fn.Name.Name
}

func typeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return "(*" + typeName(t.X) + ")"
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr: // a generic receiver, Type[T]
		return typeName(t.X)
	}
	return "?"
}

// qualified renders a package qualified name such as os.WriteFile, and returns
// "" for anything else. It is a syntactic match: a local variable called os
// would fool it, and there is none.
func qualified(expr ast.Expr) string {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return ""
	}
	return ident.Name + "." + sel.Sel.Name
}

func position(fset *token.FileSet, pos token.Pos) string {
	p := fset.Position(pos)
	return p.Filename + ":" + strconv.Itoa(p.Line)
}

func sorted(m map[string]string) []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
