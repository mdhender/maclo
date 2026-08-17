// maclo - an ML/I macro processor ported to Go
// Copyright (c) 2026 Michael D Henderson.
// All rights reserved.

package scanner

import (
	"strings"
	"testing"

	"github.com/mdhender/maclo/pkg/l/token"
)

// dump renders a token stream compactly so a test case can state what it
// expects on one line. Newline is ";" and EOF is dropped.
func dump(toks []token.Token) string {
	var sb strings.Builder
	for _, t := range toks {
		if t.Kind == token.EOF {
			continue
		}
		if sb.Len() > 0 {
			sb.WriteByte(' ')
		}
		switch t.Kind {
		case token.Word:
			sb.WriteString("W(" + t.Text + ")")
		case token.Number:
			sb.WriteString("N(" + t.Raw + ")")
		case token.Quote:
			sb.WriteString("Q(" + t.Raw + ")")
		case token.BracketText:
			sb.WriteString("T(" + t.Raw + ")")
		case token.LabelName:
			sb.WriteString("L(" + t.Text + ")")
		case token.Heading:
			sb.WriteString("H(" + t.Text + ")")
		case token.Comment:
			sb.WriteString("C(" + t.Text + ")")
		case token.Prefix:
			sb.WriteString("P(" + t.Text + ")")
		case token.Newline:
			sb.WriteString(";")
		case token.Comma:
			sb.WriteString(",")
		case token.LParen:
			sb.WriteString("(")
		case token.RParen:
			sb.WriteString(")")
		case token.Equals:
			sb.WriteString("=")
		case token.Amp:
			sb.WriteString("&")
		case token.Bar:
			sb.WriteString("|")
		case token.Plus:
			sb.WriteString("+")
		case token.Minus:
			sb.WriteString("-")
		case token.Star:
			sb.WriteString("*")
		case token.Slash:
			sb.WriteString("/")
		default:
			sb.WriteString("?" + t.Kind.String())
		}
	}
	return sb.String()
}

func TestScan(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		want string
	}{{
		name: "an assignment",
		src:  "SET SPT = IDPT-OF(LCH)",
		want: "W(SET) W(SPT) = W(IDPT) - W(OF) ( W(LCH) ) ;",
	}, {
		name: "a two word head is still two words",
		src:  "GO TO MBEGIN",
		want: "W(GO) W(TO) W(MBEGIN) ;",
	}, {
		name: "a label on the same line as its statement",
		src:  "[BSLOOP]        CALL GTATOM",
		want: "L(BSLOOP) W(CALL) W(GTATOM) ;",
	}, {
		name: "nested parentheses",
		src:  "CALL MDTEST(FFPT-OF(LNM+LCH))PT EXIT GTLOOP",
		want: "W(CALL) W(MDTEST) ( W(FFPT) - W(OF) ( W(LNM) + W(LCH) ) ) W(PT) W(EXIT) W(GTLOOP) ;",
	}, {
		name: "an indirect address keeps its type suffix as a word",
		src:  "IF IND(IDPT)CH = STOPCODE THEN GO TO MNSTOP",
		want: "W(IF) W(IND) ( W(IDPT) ) W(CH) = W(STOPCODE) W(THEN) W(GO) W(TO) W(MNSTOP) ;",
	}, {
		name: "a minus before a digit is punctuation, not part of the number",
		src:  "SET TEMP = -6+ARGNO",
		want: "W(SET) W(TEMP) = - N(6) + W(ARGNO) ;",
	}, {
		name: "multi digit numbers",
		src:  "OPMAC 'X',ENDCHN,OPMK,14",
		want: "W(OPMAC) Q(X) , W(ENDCHN) , W(OPMK) , N(14) ;",
	}, {
		name: "blank lines and indentation are dropped",
		src:  "\n\t  SET A = 0\n\n",
		want: "; W(SET) W(A) = N(0) ; ;",
	}, {
		name: "a file with no final newline still ends a statement",
		src:  "PRGEND",
		want: "W(PRGEND) ;",
	}, {
		name: "carriage returns are folded",
		src:  "SET A = 0\r\nSET B = 1\r\n",
		want: "W(SET) W(A) = N(0) ; W(SET) W(B) = N(1) ;",
	}} {
		t.Run(tc.name, func(t *testing.T) {
			toks, errs := Scan([]byte(tc.src))
			if len(errs) != 0 {
				t.Fatalf("unexpected diagnostics:\n%v", errs)
			}
			if got := dump(toks); got != tc.want {
				t.Errorf("\n got %s\nwant %s", got, tc.want)
			}
		})
	}
}

// TestQuotesWinOverComments is trap 3. The L source of ML/I has a CHARMATCH
// whose arms include '*' and '/' on one line; a scanner that looked for a
// comment before it looked for a quote would swallow the rest of the line.
func TestQuotesWinOverComments(t *testing.T) {
	src := "CHARMATCH IDPT,'+' GOING GMP,'-' GOING GMM,'*' GOING GMT,'/' GOING GMD"
	want := "W(CHARMATCH) W(IDPT) , Q(+) W(GOING) W(GMP) , Q(-) W(GOING) W(GMM) , " +
		"Q(*) W(GOING) W(GMT) , Q(/) W(GOING) W(GMD) ;"
	toks, errs := Scan([]byte(src))
	if len(errs) != 0 {
		t.Fatalf("unexpected diagnostics:\n%v", errs)
	}
	if got := dump(toks); got != want {
		t.Errorf("\n got %s\nwant %s", got, want)
	}
}

// TestCommentTerminatorsAreTwoCharacters is trap 1. A comment may contain the
// other half of its own terminator, and the second line of the L source of
// ML/I does. Scanning to the next bare '/' breaks on it.
func TestCommentTerminatorsAreTwoCharacters(t *testing.T) {
	for _, tc := range []struct {
		name, src, want string
	}{
		{"a comment containing a slash", "//A/B AUG 30 1971//", "C(A/B AUG 30 1971) ;"},
		{"a heading containing a slash", "/+SLASH / INSIDE+/", "H(SLASH / INSIDE) ;"},
		{"a comment containing a plus", "//ONE + TWO//", "C(ONE + TWO) ;"},
		{"a heading containing a minus", "/+ONE - TWO+/", "H(ONE - TWO) ;"},
		{"an empty comment", "////", "C() ;"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			toks, errs := Scan([]byte(tc.src))
			if len(errs) != 0 {
				t.Fatalf("unexpected diagnostics:\n%v", errs)
			}
			if got := dump(toks); got != tc.want {
				t.Errorf("\n got %s\nwant %s", got, tc.want)
			}
		})
	}
}

// TestCommentsAsArguments covers the three places a // comment is not trivia
// (lmap.txt 2.4, 2.5, 4.1.1.1). Note the shape difference: SECTION and
// BLOCKDEC take a comma before the comment and SUBROUTINE does not, and on
// SUBROUTINE the comment follows EXIT rather than the name.
func TestCommentsAsArguments(t *testing.T) {
	for _, tc := range []struct {
		name, src, want string
	}{
		{"section", "SECTION VARS,//DECLARATIONS//", "W(SECTION) W(VARS) , C(DECLARATIONS) ;"},
		{"blockdec", "BLOCKDEC SDB,//STATE OF SCAN//", "W(BLOCKDEC) W(SDB) , C(STATE OF SCAN) ;"},
		{"subroutine exit", "SUBROUTINE ADVNCE EXIT //END OF TEXT//", "W(SUBROUTINE) W(ADVNCE) W(EXIT) C(END OF TEXT) ;"},
		{"standalone and indented", "\t//JUST A NOTE//", "C(JUST A NOTE) ;"},
		{"a heading on its own line", "/+MAIN SCANNING ROUTINE+/", "H(MAIN SCANNING ROUTINE) ;"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			toks, errs := Scan([]byte(tc.src))
			if len(errs) != 0 {
				t.Fatalf("unexpected diagnostics:\n%v", errs)
			}
			if got := dump(toks); got != tc.want {
				t.Errorf("\n got %s\nwant %s", got, tc.want)
			}
		})
	}
}

// TestPrefixesSitOnEitherSideOfTheLabel is trap 6. The manual only ever shows
// "/- OVP -/ SET" with spaces and no label (lmap.txt appendix A); the L source
// of ML/I writes the prefix after the label in the INVALS section and before
// it on every CSS-marked label, always with no interior spaces.
func TestPrefixesSitOnEitherSideOfTheLabel(t *testing.T) {
	for _, tc := range []struct {
		name, src, want string
	}{
		{"label then prefix", "[BEGIN] /-IN-/SET ARGPT = NULLPT", "L(BEGIN) P(IN) W(SET) W(ARGPT) = W(NULLPT) ;"},
		{"prefix then label", "/-CSS-/[FNCTEX] CALL OPEXIT", "P(CSS) L(FNCTEX) W(CALL) W(OPEXIT) ;"},
		{"prefix alone, with the manual's spaces", "/- OVP -/ SET A = A+1", "P(OVP) W(SET) W(A) = W(A) + N(1) ;"},
		{"two prefixes", "/-IN-//-OVP-/SET A = 0", "P(IN) P(OVP) W(SET) W(A) = N(0) ;"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			toks, errs := Scan([]byte(tc.src))
			if len(errs) != 0 {
				t.Fatalf("unexpected diagnostics:\n%v", errs)
			}
			if got := dump(toks); got != tc.want {
				t.Errorf("\n got %s\nwant %s", got, tc.want)
			}
		})
	}
}

// TestBracketIsContextSensitive is trap 4, and the only lexical decision this
// scanner makes. "[SKIP] SETSW SQSW = 0" is a label and "PRTEXT[SKIP]" is a
// string; the bytes of the bracket are identical and only what precedes them
// differs. Both forms occur in the L source of ML/I with the same name.
func TestBracketIsContextSensitive(t *testing.T) {
	for _, tc := range []struct {
		name, src, want string
	}{
		{"a label", "[SKIP]  SETSW SQSW = 0", "L(SKIP) W(SETSW) W(SQSW) = N(0) ;"},
		{"the same name as PRTEXT text", "[PRT4]  PRTEXT[SKIP]", "L(PRT4) W(PRTEXT) T(SKIP) ;"},
		{"LID text", "DC LID[SPACES]", "W(DC) W(LID) T(SPACES) ;"},
		{"trailing spaces inside the brackets are kept", "PRTEXT[)    ]", "W(PRTEXT) T()    ) ;"},
		{"dollars and punctuation inside the brackets", "PRTEXT[$$$ERROR(S)$]", "W(PRTEXT) T($$$ERROR(S)$) ;"},
		{"a label on the line before does not leak", "[A1]\nPRTEXT[B]", "L(A1) ; W(PRTEXT) T(B) ;"},
		{"a bracket after a non-word opens a label", "SET A = 0\n[B1] READ", "W(SET) W(A) = N(0) ; L(B1) W(READ) ;"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			toks, errs := Scan([]byte(tc.src))
			if len(errs) != 0 {
				t.Fatalf("unexpected diagnostics:\n%v", errs)
			}
			if got := dump(toks); got != tc.want {
				t.Errorf("\n got %s\nwant %s", got, tc.want)
			}
		})
	}
}

// TestDollarDecoding checks that a token keeps both what was written and what
// it means. The listing round-trips Raw; a back end wants Text.
func TestDollarDecoding(t *testing.T) {
	toks, errs := Scan([]byte("PRTEXT[A$B]\nSET C = '$'"))
	if len(errs) != 0 {
		t.Fatalf("unexpected diagnostics:\n%v", errs)
	}
	var text, quote token.Token
	for _, tk := range toks {
		switch tk.Kind {
		case token.BracketText:
			text = tk
		case token.Quote:
			quote = tk
		}
	}
	if text.Raw != "A$B" || text.Text != "A\nB" {
		t.Errorf("bracket text: raw %q text %q, want %q and %q", text.Raw, text.Text, "A$B", "A\nB")
	}
	if quote.Raw != "$" || quote.Text != "\n" {
		t.Errorf("quote: raw %q text %q, want %q and %q", quote.Raw, quote.Text, "$", "\n")
	}
}

// TestSlashIsDivisionWhenItIsNotAComment. Nothing in the corpus divides, but
// LICH is defined as 1/LCH (lmap.txt 3.3.1), so the operator exists. Whether a
// slash is legal where it appears is a question about OF that the scanner
// cannot answer, so it emits the token and sema decides.
func TestSlashIsDivisionWhenItIsNotAComment(t *testing.T) {
	toks, errs := Scan([]byte("DC OF(1/LCH)"))
	if len(errs) != 0 {
		t.Fatalf("unexpected diagnostics:\n%v", errs)
	}
	if got, want := dump(toks), "W(DC) W(OF) ( N(1) / W(LCH) ) ;"; got != want {
		t.Errorf("\n got %s\nwant %s", got, want)
	}
}

func TestScanErrors(t *testing.T) {
	for _, tc := range []struct {
		name, src, want string
	}{
		{"unterminated quote", "SET A = 'X", "unterminated quote"},
		{"quote running to the next line", "SET A = 'X\nSET B = 0", "unterminated quote"},
		{"unterminated comment", "//A NOTE\nSET A = 0", "unterminated comment: no closing //"},
		{"unterminated heading", "/+A HEADING\n", "unterminated comment: no closing +/"},
		{"unterminated prefix", "/-OVP\n", "unterminated comment: no closing -/"},
		{"unterminated bracket text", "PRTEXT[ABC\n", "unterminated text"},
		{"unterminated label", "[ABC\n", "unterminated label"},
		{"empty label", "[]\n", "empty label"},
		{"lower case", "set A = 0", "not in capitals"},
		{"a stray dollar", "SET A = $", "only allowed inside a quote"},
		{"a stray semicolon", "SET A = 0;", "only allowed inside a quote"},
		{"a stray close bracket", "SET A = 0]", "no [ opened here"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, errs := Scan([]byte(tc.src))
			if len(errs) == 0 {
				t.Fatalf("no diagnostic; want one mentioning %q", tc.want)
			}
			if !strings.Contains(errs[0].Msg, tc.want) {
				t.Errorf("first diagnostic is %q, want one mentioning %q", errs[0].Msg, tc.want)
			}
		})
	}
}

// TestScanAccumulates is the difference from pkg/lowl/scanner, which stops
// appending at the first bad token. Over a 2,500 line file, one diagnostic per
// run means one typo fixed per run.
func TestScanAccumulates(t *testing.T) {
	src := "set A = 0\nSET B = 'X\nSET C = 0\nPRTEXT[ABC\n"
	_, errs := Scan([]byte(src))
	if len(errs) != 3 {
		t.Fatalf("got %d diagnostics, want 3:\n%v", len(errs), errs)
	}
	for i, wantLine := range []int{1, 2, 4} {
		if errs[i].Pos.Line != wantLine {
			t.Errorf("diagnostic %d is at line %d, want %d: %s", i, errs[i].Pos.Line, wantLine, errs[i].Msg)
		}
	}
}

// TestPositions checks that a token knows where it came from, because every
// diagnostic downstream of here is only as good as this.
func TestPositions(t *testing.T) {
	toks, errs := Scan([]byte("[BEGIN] SET A = 0\n  CALL X\n"))
	if len(errs) != 0 {
		t.Fatalf("unexpected diagnostics:\n%v", errs)
	}
	for _, want := range []struct {
		index     int
		kind      token.Kind
		line, col int
	}{
		{0, token.LabelName, 1, 1},
		{1, token.Word, 1, 9},
		{2, token.Word, 1, 13},
		{3, token.Equals, 1, 15},
		{4, token.Number, 1, 17},
		{5, token.Newline, 1, 18},
		{6, token.Word, 2, 3},
		{7, token.Word, 2, 8},
	} {
		got := toks[want.index]
		if got.Kind != want.kind || got.Pos.Line != want.line || got.Pos.Col != want.col {
			t.Errorf("token %d is %s at %s, want %s at %d:%d",
				want.index, got.Kind, got.Pos, want.kind, want.line, want.col)
		}
	}
}
