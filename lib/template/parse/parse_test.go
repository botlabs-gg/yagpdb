// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package parse

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var debug = flag.Bool("debug", false, "show the errors produced by the main tests")

type numberTest struct {
	text      string
	isInt     bool
	isUint    bool
	isFloat   bool
	isComplex bool
	int64
	uint64
	float64
	complex128
}

var numberTests = []numberTest{
	// basics
	{"0", true, true, true, false, 0, 0, 0, 0},
	{"-0", true, true, true, false, 0, 0, 0, 0}, // check that -0 is a uint.
	{"73", true, true, true, false, 73, 73, 73, 0},
	{"7_3", true, true, true, false, 73, 73, 73, 0},
	{"0b10_010_01", true, true, true, false, 73, 73, 73, 0},
	{"0B10_010_01", true, true, true, false, 73, 73, 73, 0},
	{"073", true, true, true, false, 073, 073, 073, 0},
	{"0o73", true, true, true, false, 073, 073, 073, 0},
	{"0O73", true, true, true, false, 073, 073, 073, 0},
	{"0x73", true, true, true, false, 0x73, 0x73, 0x73, 0},
	{"0X73", true, true, true, false, 0x73, 0x73, 0x73, 0},
	{"0x7_3", true, true, true, false, 0x73, 0x73, 0x73, 0},
	{"-73", true, false, true, false, -73, 0, -73, 0},
	{"+73", true, false, true, false, 73, 0, 73, 0},
	{"100", true, true, true, false, 100, 100, 100, 0},
	{"1e9", true, true, true, false, 1e9, 1e9, 1e9, 0},
	{"-1e9", true, false, true, false, -1e9, 0, -1e9, 0},
	{"-1.2", false, false, true, false, 0, 0, -1.2, 0},
	{"1e19", false, true, true, false, 0, 1e19, 1e19, 0},
	{"1e1_9", false, true, true, false, 0, 1e19, 1e19, 0},
	{"1E19", false, true, true, false, 0, 1e19, 1e19, 0},
	{"-1e19", false, false, true, false, 0, 0, -1e19, 0},
	{"0x_1p4", true, true, true, false, 16, 16, 16, 0},
	{"0X_1P4", true, true, true, false, 16, 16, 16, 0},
	{"0x_1p-4", false, false, true, false, 0, 0, 1 / 16., 0},
	{"4i", false, false, false, true, 0, 0, 0, 4i},
	{"-1.2+4.2i", false, false, false, true, 0, 0, 0, -1.2 + 4.2i},
	{"073i", false, false, false, true, 0, 0, 0, 73i}, // not octal!
	// complex with 0 imaginary are float (and maybe integer)
	{"0i", true, true, true, true, 0, 0, 0, 0},
	{"-1.2+0i", false, false, true, true, 0, 0, -1.2, -1.2},
	{"-12+0i", true, false, true, true, -12, 0, -12, -12},
	{"13+0i", true, true, true, true, 13, 13, 13, 13},
	// funny bases
	{"0123", true, true, true, false, 0123, 0123, 0123, 0},
	{"-0x0", true, true, true, false, 0, 0, 0, 0},
	{"0xdeadbeef", true, true, true, false, 0xdeadbeef, 0xdeadbeef, 0xdeadbeef, 0},
	// character constants
	{`'a'`, true, true, true, false, 'a', 'a', 'a', 0},
	{`'\n'`, true, true, true, false, '\n', '\n', '\n', 0},
	{`'\\'`, true, true, true, false, '\\', '\\', '\\', 0},
	{`'\''`, true, true, true, false, '\'', '\'', '\'', 0},
	{`'\xFF'`, true, true, true, false, 0xFF, 0xFF, 0xFF, 0},
	{`'パ'`, true, true, true, false, 0x30d1, 0x30d1, 0x30d1, 0},
	{`'\u30d1'`, true, true, true, false, 0x30d1, 0x30d1, 0x30d1, 0},
	{`'\U000030d1'`, true, true, true, false, 0x30d1, 0x30d1, 0x30d1, 0},
	// some broken syntax
	{text: "+-2"},
	{text: "0x123."},
	{text: "1e."},
	{text: "0xi."},
	{text: "1+2."},
	{text: "'x"},
	{text: "'xx'"},
	{text: "'433937734937734969526500969526500'"}, // Integer too large - issue 10634.
	// Issue 8622 - 0xe parsed as floating point. Very embarrassing.
	{"0xef", true, true, true, false, 0xef, 0xef, 0xef, 0},
}

func TestNumberParse(t *testing.T) {
	for _, test := range numberTests {
		// If fmt.Sscan thinks it's complex, it's complex. We can't trust the output
		// because imaginary comes out as a number.
		var c complex128
		typ := itemNumber
		var tree *Tree
		if test.text[0] == '\'' {
			typ = itemCharConstant
		} else {
			_, err := fmt.Sscan(test.text, &c)
			if err == nil {
				typ = itemComplex
			}
		}
		n, err := tree.newNumber(0, test.text, typ)
		ok := test.isInt || test.isUint || test.isFloat || test.isComplex
		if ok && err != nil {
			t.Errorf("unexpected error for %q: %s", test.text, err)
			continue
		}
		if !ok && err == nil {
			t.Errorf("expected error for %q", test.text)
			continue
		}
		if !ok {
			if *debug {
				fmt.Printf("%s\n\t%s\n", test.text, err)
			}
			continue
		}
		if n.IsComplex != test.isComplex {
			t.Errorf("complex incorrect for %q; should be %t", test.text, test.isComplex)
		}
		if test.isInt {
			if !n.IsInt {
				t.Errorf("expected integer for %q", test.text)
			}
			if n.Int64 != test.int64 {
				t.Errorf("int64 for %q should be %d Is %d", test.text, test.int64, n.Int64)
			}
		} else if n.IsInt {
			t.Errorf("did not expect integer for %q", test.text)
		}
		if test.isUint {
			if !n.IsUint {
				t.Errorf("expected unsigned integer for %q", test.text)
			}
			if n.Uint64 != test.uint64 {
				t.Errorf("uint64 for %q should be %d Is %d", test.text, test.uint64, n.Uint64)
			}
		} else if n.IsUint {
			t.Errorf("did not expect unsigned integer for %q", test.text)
		}
		if test.isFloat {
			if !n.IsFloat {
				t.Errorf("expected float for %q", test.text)
			}
			if n.Float64 != test.float64 {
				t.Errorf("float64 for %q should be %g Is %g", test.text, test.float64, n.Float64)
			}
		} else if n.IsFloat {
			t.Errorf("did not expect float for %q", test.text)
		}
		if test.isComplex {
			if !n.IsComplex {
				t.Errorf("expected complex for %q", test.text)
			}
			if n.Complex128 != test.complex128 {
				t.Errorf("complex128 for %q should be %g Is %g", test.text, test.complex128, n.Complex128)
			}
		} else if n.IsComplex {
			t.Errorf("did not expect complex for %q", test.text)
		}
	}
}

type parseTest struct {
	name   string
	input  string
	ok     bool
	result string // what the user would see in an error message.
}

const (
	noError  = true
	hasError = false
)

var parseTests = []parseTest{
	{"empty", "", noError,
		``},
	{"comment", "{{/*\n\n\n*/}}", noError,
		``},
	{"spaces", " \t\n", noError,
		`" \t\n"`},
	{"text", "some text", noError,
		`"some text"`},
	{"emptyAction", "{{}}", hasError,
		`{{}}`},
	{"field", "{{.X}}", noError,
		`{{.X}}`},
	{"simple command", "{{printf}}", noError,
		`{{printf}}`},
	{"$ invocation", "{{$}}", noError,
		"{{$}}"},
	{"variable invocation", "{{with $x := 3}}{{$x 23}}{{end}}", noError,
		"{{with $x := 3}}{{$x 23}}{{end}}"},
	{"variable with fields", "{{$.I}}", noError,
		"{{$.I}}"},
	{"multi-word command", "{{printf `%d` 23}}", noError,
		"{{printf `%d` 23}}"},
	{"pipeline", "{{.X|.Y}}", noError,
		`{{.X | .Y}}`},
	{"pipeline with decl", "{{$x := .X|.Y}}", noError,
		`{{$x := .X | .Y}}`},
	{"nested pipeline", "{{.X (.Y .Z) (.A | .B .C) (.E)}}", noError,
		`{{.X (.Y .Z) (.A | .B .C) (.E)}}`},
	{"field applied to parentheses", "{{(.Y .Z).Field}}", noError,
		`{{(.Y .Z).Field}}`},
	{"simple if", "{{if .X}}hello{{end}}", noError,
		`{{if .X}}"hello"{{end}}`},
	{"if with else", "{{if .X}}true{{else}}false{{end}}", noError,
		`{{if .X}}"true"{{else}}"false"{{end}}`},
	{"if with else if", "{{if .X}}true{{else if .Y}}false{{end}}", noError,
		`{{if .X}}"true"{{else}}{{if .Y}}"false"{{end}}{{end}}`},
	{"if else chain", "+{{if .X}}X{{else if .Y}}Y{{else if .Z}}Z{{end}}+", noError,
		`"+"{{if .X}}"X"{{else}}{{if .Y}}"Y"{{else}}{{if .Z}}"Z"{{end}}{{end}}{{end}}"+"`},
	{"try-catch", "{{try}}abc{{catch}}xyz{{end}}", noError,
		`{{try}}"abc"{{catch}}"xyz"{{end}}`},
	{"try with empty catch", "{{try}}abc{{catch}}{{end}}", noError,
		`{{try}}"abc"{{catch}}{{end}}`},
	{"simple range", "{{range .X}}hello{{end}}", noError,
		`{{range .X}}"hello"{{end}}`},
	{"chained field range", "{{range .X.Y.Z}}hello{{end}}", noError,
		`{{range .X.Y.Z}}"hello"{{end}}`},
	{"nested range", "{{range .X}}hello{{range .Y}}goodbye{{end}}{{end}}", noError,
		`{{range .X}}"hello"{{range .Y}}"goodbye"{{end}}{{end}}`},
	{"range with else", "{{range .X}}true{{else}}false{{end}}", noError,
		`{{range .X}}"true"{{else}}"false"{{end}}`},
	{"range over pipeline", "{{range .X|.M}}true{{else}}false{{end}}", noError,
		`{{range .X | .M}}"true"{{else}}"false"{{end}}`},
	{"range []int", "{{range .SI}}{{.}}{{end}}", noError,
		`{{range .SI}}{{.}}{{end}}`},
	{"range 1 var", "{{range $x := .SI}}{{.}}{{end}}", noError,
		`{{range $x := .SI}}{{.}}{{end}}`},
	{"range 2 vars", "{{range $x, $y := .SI}}{{.}}{{end}}", noError,
		`{{range $x, $y := .SI}}{{.}}{{end}}`},
	{"range with break", "{{range .SI}}{{.}}{{break}}{{end}}", noError,
		`{{range .SI}}{{.}}{{break}}{{end}}`},
	{"range with continue", "{{range .SI}}{{.}}{{continue}}{{end}}", noError,
		`{{range .SI}}{{.}}{{continue}}{{end}}`},
	{"simple while", "{{$i := 0}}{{while lt $i 5}}hello{{$i := add $i 1}}{{end}}", noError,
		`{{$i := 0}}{{while lt $i 5}}"hello"{{$i := add $i 1}}{{end}}`},
	{"while declaration", "{{$i := 0}}{{while $truth := lt $i 5}}hello{{$i := add $i 1}}{{end}}", noError,
		`{{$i := 0}}{{while $truth := lt $i 5}}"hello"{{$i := add $i 1}}{{end}}`},
	{"while with else", "{{while true}}hello{{else}}goodbye{{end}}", noError,
		`{{while true}}"hello"{{else}}"goodbye"{{end}}`},
	{"while with break", "{{while true}}{{break}}{{end}}", noError,
		`{{while true}}{{break}}{{end}}`},
	{"while with continue", "{{while true}}{{continue}}{{end}}", noError,
		`{{while true}}{{continue}}{{end}}`},
	{"constants", "{{range .SI 1 -3.2i true false 'a' nil}}{{end}}", noError,
		`{{range .SI 1 -3.2i true false 'a' nil}}{{end}}`},
	{"template", "{{template `x`}}", noError,
		`{{template "x"}}`},
	{"template with arg", "{{template `x` .Y}}", noError,
		`{{template "x" .Y}}`},
	{"return", `{{return}}`, noError,
		`{{return}}`},
	{"return with arg", "{{return .Y}}", noError,
		`{{return .Y}}`},
	{"with", "{{with .X}}hello{{end}}", noError,
		`{{with .X}}"hello"{{end}}`},
	{"with with else", "{{with .X}}hello{{else}}goodbye{{end}}", noError,
		`{{with .X}}"hello"{{else}}"goodbye"{{end}}`},
	{"with with else if", "{{with .X}}true{{else if .Y}}false{{end}}", noError, `{{with .X}}"true"{{else}}{{if .Y}}"false"{{end}}{{end}}`},
	{"with else chain", "+{{with .X}}X{{else if .Y}}Y{{else if .Z}}Z{{end}}+", noError, `"+"{{with .X}}"X"{{else}}{{if .Y}}"Y"{{else}}{{if .Z}}"Z"{{end}}{{end}}{{end}}"+"`},
	// Trimming spaces.
	{"trim left", "x \r\n\t{{- 3}}", noError, `"x"{{3}}`},
	{"trim right", "{{3 -}}\n\n\ty", noError, `{{3}}"y"`},
	{"trim left and right", "x \r\n\t{{- 3 -}}\n\n\ty", noError, `"x"{{3}}"y"`},
	{"comment trim left", "x \r\n\t{{- /* hi */}}", noError, `"x"`},
	{"comment trim right", "{{/* hi */ -}}\n\n\ty", noError, `"y"`},
	{"comment trim left and right", "x \r\n\t{{- /* */ -}}\n\n\ty", noError, `"x""y"`},
	{"block definition", `{{block "foo" .}}hello{{end}}`, noError,
		`{{template "foo" .}}`},
	// Errors.
	{"unclosed action", "hello{{range", hasError, ""},
	{"unmatched end", "{{end}}", hasError, ""},
	{"unmatched else", "{{else}}", hasError, ""},
	{"unmatched else after if", "{{if .X}}hello{{end}}{{else}}", hasError, ""},
	{"multiple else", "{{if .X}}1{{else}}2{{else}}3{{end}}", hasError, ""},
	{"missing end", "hello{{range .x}}", hasError, ""},
	{"missing end after else", "hello{{range .x}}{{else}}", hasError, ""},
	{"undefined function", "hello{{undefined}}", hasError, ""},
	{"undefined variable", "{{$x}}", hasError, ""},
	{"variable undefined after end", "{{with $x := 4}}{{end}}{{$x}}", hasError, ""},
	{"variable undefined in template", "{{template $v}}", hasError, ""},
	{"declare with field", "{{with $x.Y := 4}}{{end}}", hasError, ""},
	{"template with field ref", "{{template .X}}", hasError, ""},
	{"template with var", "{{template $v}}", hasError, ""},
	{"invalid punctuation", "{{printf 3, 4}}", hasError, ""},
	{"multidecl outside range", "{{with $v, $u := 3}}{{end}}", hasError, ""},
	{"too many decls in range", "{{range $u, $v, $w := 3}}{{end}}", hasError, ""},
	{"dot applied to parentheses", "{{printf (printf .).}}", hasError, ""},
	{"adjacent args", "{{printf 3`x`}}", hasError, ""},
	{"adjacent args with .", "{{printf `x`.}}", hasError, ""},
	{"extra end after if", "{{if .X}}a{{else if .Y}}b{{end}}{{end}}", hasError, ""},
	{"if-catch", "{{if $x.Y}}abc{{catch}}xyz{{end}}", hasError, ""},
	{"try with no catch", "{{try}}abc{{end}}", hasError, ""},
	{"try with else", "{{try}}abc{{else}}{{end}}", hasError, ""},
	{"try-catch-else chain", "{{try}}abc{{catch}}xyz{{else}}dxy{{end}}", hasError, ""},
	{"top level catch", "{{catch}}", hasError, ""},
	{"break outside range/while", "{{range .}}{{end}} {{break}}", hasError, ""},
	{"continue outside range/while", "{{while .}}{{end}} {{continue}}", hasError, ""},
	{"break in range else", "{{range .}}{{else}}{{break}}{{end}}", hasError, ""},
	{"continue in range else", "{{range .}}{{else}}{{continue}}{{end}}", hasError, ""},
	{"break in while else", "{{while true}}{{else}}{{break}}{{end}}", hasError, ""},
	{"continue in while else", "{{while true}}{{else}}{{continue}}{{end}}", hasError, ""},
	// Other kinds of assignments and operators aren't available yet.
	{"bug0a", "{{$x := 0}}{{$x}}", noError, "{{$x := 0}}{{$x}}"},
	{"bug0b", "{{$x += 1}}{{$x}}", hasError, ""},
	{"bug0c", "{{$x ! 2}}{{$x}}", hasError, ""},
	{"bug0d", "{{$x % 3}}{{$x}}", hasError, ""},
	// Check the parse fails for := rather than comma.
	{"bug0e", "{{range $x := $y := 3}}{{end}}", hasError, ""},
	// Another bug: variable read must ignore following punctuation.
	{"bug1a", "{{$x:=.}}{{$x!2}}", hasError, ""},                     // ! is just illegal here.
	{"bug1b", "{{$x:=.}}{{$x+2}}", hasError, ""},                     // $x+2 should not parse as ($x) (+2).
	{"bug1c", "{{$x:=.}}{{$x +2}}", noError, "{{$x := .}}{{$x +2}}"}, // It's OK with a space.
	// dot following a literal value
	{"dot after integer", "{{1.E}}", hasError, ""},
	{"dot after float", "{{0.1.E}}", hasError, ""},
	{"dot after boolean", "{{true.E}}", hasError, ""},
	{"dot after char", "{{'a'.any}}", hasError, ""},
	{"dot after string", `{{"hello".guys}}`, hasError, ""},
	{"dot after dot", "{{..E}}", hasError, ""},
	{"dot after nil", "{{nil.E}}", hasError, ""},
	// Wrong pipeline
	{"wrong pipeline dot", "{{12|.}}", hasError, ""},
	{"wrong pipeline number", "{{.|12|printf}}", hasError, ""},
	{"wrong pipeline string", "{{.|printf|\"error\"}}", hasError, ""},
	{"wrong pipeline char", "{{12|printf|'e'}}", hasError, ""},
	{"wrong pipeline boolean", "{{.|true}}", hasError, ""},
	{"wrong pipeline nil", "{{'c'|nil}}", hasError, ""},
	{"empty pipeline", `{{printf "%d" ( ) }}`, hasError, ""},
	// Missing pipeline in block
	{"block definition", `{{block "foo"}}hello{{end}}`, hasError, ""},
	// Invalid loop control
	{"break outside of loop", `{{break}}`, hasError, ""},
	{"break in range else, outside of range", `{{range .}}{{.}}{{else}}{{break}{{end}}`, hasError, ""},
	{"break in while else, outside of while", `{{while $i := .I}}{{.}}{{else}}{{break}}{{end}}`, hasError, ""},
	{"continue outside of loop", `{{continue}}`, hasError, ""},
	{"continue in range else, outside of range", `{{range .}}{{.}}{{else}}{{continue}}{{end}}`, hasError, ""},
	{"continue in while else, outside of while", `{{while true}}{{.}}{{else}}{{continue}}{{end}}`, hasError, ""},
	{"additional break data", `{{range .}}{{break label}}{{end}}`, hasError, ""},
	{"additional continue data", `{{range .}}{{continue label}}{{end}}`, hasError, ""},
}

var builtins = map[string]interface{}{
	"printf": fmt.Sprintf,
	"add":    func(a, b int) int { return a + b },
	"lt":     func(a, b int) bool { return a < b },
}

func testParse(doCopy bool, t *testing.T) {
	textFormat = "%q"
	defer func() { textFormat = "%s" }()
	for _, test := range parseTests {
		tmpl, err := New(test.name).Parse(test.input, "", "", make(map[string]*Tree), builtins)
		switch {
		case err == nil && !test.ok:
			t.Errorf("%q: expected error; got none", test.name)
			continue
		case err != nil && test.ok:
			t.Errorf("%q: unexpected error: %v", test.name, err)
			continue
		case err != nil && !test.ok:
			// expected error, got one
			if *debug {
				fmt.Printf("%s: %s\n\t%s\n", test.name, test.input, err)
			}
			continue
		}
		var result string
		if doCopy {
			result = tmpl.Root.Copy().String()
		} else {
			result = tmpl.Root.String()
		}
		if result != test.result {
			t.Errorf("%s=(%q): got\n\t%v\nexpected\n\t%v", test.name, test.input, result, test.result)
		}
	}
}

func TestParse(t *testing.T) {
	testParse(false, t)
}

// Same as TestParse, but we copy the node first
func TestParseCopy(t *testing.T) {
	testParse(true, t)
}

type isEmptyTest struct {
	name  string
	input string
	empty bool
}

var isEmptyTests = []isEmptyTest{
	{"empty", ``, true},
	{"nonempty", `hello`, false},
	{"spaces only", " \t\n \t\n", true},
	{"definition", `{{define "x"}}something{{end}}`, true},
	{"definitions and space", "{{define `x`}}something{{end}}\n\n{{define `y`}}something{{end}}\n\n", true},
	{"definitions and text", "{{define `x`}}something{{end}}\nx\n{{define `y`}}something{{end}}\ny\n", false},
	{"definition and action", "{{define `x`}}something{{end}}{{if 3}}foo{{end}}", false},
}

func TestIsEmpty(t *testing.T) {
	if !IsEmptyTree(nil) {
		t.Errorf("nil tree is not empty")
	}
	for _, test := range isEmptyTests {
		tree, err := New("root").Parse(test.input, "", "", make(map[string]*Tree), nil)
		if err != nil {
			t.Errorf("%q: unexpected error: %v", test.name, err)
			continue
		}
		if empty := IsEmptyTree(tree.Root); empty != test.empty {
			t.Errorf("%q: expected %t got %t", test.name, test.empty, empty)
		}
	}
}

func TestErrorContextWithTreeCopy(t *testing.T) {
	tree, err := New("root").Parse("{{if true}}{{end}}", "", "", make(map[string]*Tree), nil)
	if err != nil {
		t.Fatalf("unexpected tree parse failure: %v", err)
	}
	treeCopy := tree.Copy()
	wantLocation, wantContext := tree.ErrorContext(tree.Root.Nodes[0])
	gotLocation, gotContext := treeCopy.ErrorContext(treeCopy.Root.Nodes[0])
	if wantLocation != gotLocation {
		t.Errorf("wrong error location want %q got %q", wantLocation, gotLocation)
	}
	if wantContext != gotContext {
		t.Errorf("wrong error location want %q got %q", wantContext, gotContext)
	}
}

// All failures, and the result is a string that must appear in the error message.
var errorTests = []parseTest{
	// Check line numbers are accurate.
	{"unclosed1",
		"line1\n{{",
		hasError, `unclosed1:2: unexpected unclosed action in command`},
	{"unclosed2",
		"line1\n{{define `x`}}line2\n{{",
		hasError, `unclosed2:3: unexpected unclosed action in command`},
	// Specific errors.
	{"function",
		"{{foo}}",
		hasError, `function "foo" not defined`},
	{"comment",
		"{{/*}}",
		hasError, `unclosed comment`},
	{"lparen",
		"{{.X (1 2 3}}",
		hasError, `unclosed left paren`},
	{"rparen",
		"{{.X 1 2 3)}}",
		hasError, `unexpected right paren`},
	{"space",
		"{{`x`3}}",
		hasError, `in operand`},
	{"idchar",
		"{{a#}}",
		hasError, `'#'`},
	{"charconst",
		"{{'a}}",
		hasError, `unterminated character constant`},
	{"stringconst",
		`{{"a}}`,
		hasError, `unterminated quoted string`},
	{"rawstringconst",
		"{{`a}}",
		hasError, `unterminated raw quoted string`},
	{"number",
		"{{0xi}}",
		hasError, `number syntax`},
	{"multidefine",
		"{{define `a`}}a{{end}}{{define `a`}}b{{end}}",
		hasError, `multiple definition of template`},
	{"eof",
		"{{range .X}}",
		hasError, `unexpected EOF`},
	{"variable",
		// Declare $x so it's defined, to avoid that error, and then check we don't parse a declaration.
		"{{$x := 23}}{{with $x.y := 3}}{{$x 23}}{{end}}",
		hasError, `unexpected ":="`},
	{"multidecl",
		"{{$a,$b,$c := 23}}",
		hasError, `too many declarations`},
	{"undefvar",
		"{{$a}}",
		hasError, `undefined variable`},
	{"wrongdot",
		"{{true.any}}",
		hasError, `unexpected . after term`},
	{"wrongpipeline",
		"{{12|false}}",
		hasError, `non executable command in pipeline`},
	{"emptypipeline",
		`{{ ( ) }}`,
		hasError, `missing value for parenthesized pipeline`},
	{"multilinerawstring",
		"{{ $v := `\n` }} {{",
		hasError, `multilinerawstring:2: unexpected unclosed action`},
	{"rangeundefvar",
		"{{range $k}}{{end}}",
		hasError, `undefined variable`},
	{"rangeundefvars",
		"{{range $k, $v}}{{end}}",
		hasError, `undefined variable`},
	{"rangemissingvalue1",
		"{{range $k,}}{{end}}",
		hasError, `missing value for range`},
	{"rangemissingvalue2",
		"{{range $k, $v := }}{{end}}",
		hasError, `missing value for range`},
	{"rangenotvariable1",
		"{{range $k, .}}{{end}}",
		hasError, `range can only initialize variables`},
	{"rangenotvariable2",
		"{{range $k, 123 := .}}{{end}}",
		hasError, `range can only initialize variables`},
}

func TestErrors(t *testing.T) {
	for _, test := range errorTests {
		t.Run(test.name, func(t *testing.T) {
			_, err := New(test.name).Parse(test.input, "", "", make(map[string]*Tree))
			if err == nil {
				t.Fatalf("expected error %q, got nil", test.result)
			}
			if !strings.Contains(err.Error(), test.result) {
				t.Fatalf("error %q does not contain %q", err, test.result)
			}
		})
	}
}

func TestBlock(t *testing.T) {
	const (
		input = `a{{block "inner" .}}bar{{.}}baz{{end}}b`
		outer = `a{{template "inner" .}}b`
		inner = `bar{{.}}baz`
	)
	treeSet := make(map[string]*Tree)
	tmpl, err := New("outer").Parse(input, "", "", treeSet, nil)
	if err != nil {
		t.Fatal(err)
	}
	if g, w := tmpl.Root.String(), outer; g != w {
		t.Errorf("outer template = %q, want %q", g, w)
	}
	inTmpl := treeSet["inner"]
	if inTmpl == nil {
		t.Fatal("block did not define template")
	}
	if g, w := inTmpl.Root.String(), inner; g != w {
		t.Errorf("inner template = %q, want %q", g, w)
	}
}

func TestLineNum(t *testing.T) {
	const count = 100
	text := strings.Repeat("{{printf 1234}}\n", count)
	tree, err := New("bench").Parse(text, "", "", make(map[string]*Tree), builtins)
	if err != nil {
		t.Fatal(err)
	}
	// Check the line numbers. Each line is an action containing a template, followed by text.
	// That's two nodes per line.
	nodes := tree.Root.Nodes
	for i := 0; i < len(nodes); i += 2 {
		line := 1 + i/2
		// Action first.
		action := nodes[i].(*ActionNode)
		if action.Line != line {
			t.Fatalf("line %d: action is line %d", line, action.Line)
		}
		pipe := action.Pipe
		if pipe.Line != line {
			t.Fatalf("line %d: pipe is line %d", line, pipe.Line)
		}
	}
}

func BenchmarkParse(b *testing.B) {
	benchmarks := []struct {
		name, file string
	}{
		{"lorem ipsum", "lorem.tmpl"},
		{"short", "short.tmpl"},
		{"medium", "medium.tmpl"},
		{"long", "long.tmpl"},
		{"very-long", "very_long.tmpl"},
	}
	for _, bm := range benchmarks {
		f, err := os.ReadFile(filepath.Join("..", "testdata", bm.file))
		if err != nil {
			b.Fatal(err)
		}

		in := string(f)
		b.Run(bm.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := Parse(bm.name, in, "{{", "}}", funcs)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

var funcs = map[string]interface{}{
	"add":                       func() interface{} { return nil },
	"addMessageReactions":       func() interface{} { return nil },
	"addReactions":              func() interface{} { return nil },
	"addResponseReactions":      func() interface{} { return nil },
	"addRoleID":                 func() interface{} { return nil },
	"addRoleName":               func() interface{} { return nil },
	"adjective":                 func() interface{} { return nil },
	"and":                       func() interface{} { return nil },
	"bitwiseAnd":                func() interface{} { return nil },
	"bitwiseAndNot":             func() interface{} { return nil },
	"bitwiseLeftShift":          func() interface{} { return nil },
	"bitwiseNot":                func() interface{} { return nil },
	"bitwiseOr":                 func() interface{} { return nil },
	"bitwiseRightShift":         func() interface{} { return nil },
	"bitwiseXor":                func() interface{} { return nil },
	"call":                      func() interface{} { return nil },
	"cancelScheduledUniqueCC":   func() interface{} { return nil },
	"carg":                      func() interface{} { return nil },
	"cbrt":                      func() interface{} { return nil },
	"cembed":                    func() interface{} { return nil },
	"complexMessage":            func() interface{} { return nil },
	"complexMessageEdit":        func() interface{} { return nil },
	"createTicket":              func() interface{} { return nil },
	"cslice":                    func() interface{} { return nil },
	"currentTime":               func() interface{} { return nil },
	"currentUserAgeHuman":       func() interface{} { return nil },
	"currentUserAgeMinutes":     func() interface{} { return nil },
	"currentUserCreated":        func() interface{} { return nil },
	"dbBottomEntries":           func() interface{} { return nil },
	"dbCount":                   func() interface{} { return nil },
	"dbDel":                     func() interface{} { return nil },
	"dbDelByID":                 func() interface{} { return nil },
	"dbDelById":                 func() interface{} { return nil },
	"dbDelMultiple":             func() interface{} { return nil },
	"dbGet":                     func() interface{} { return nil },
	"dbGetPattern":              func() interface{} { return nil },
	"dbGetPatternReverse":       func() interface{} { return nil },
	"dbIncr":                    func() interface{} { return nil },
	"dbRank":                    func() interface{} { return nil },
	"dbSet":                     func() interface{} { return nil },
	"dbSetExpire":               func() interface{} { return nil },
	"dbTopEntries":              func() interface{} { return nil },
	"deleteAllMessageReactions": func() interface{} { return nil },
	"deleteMessage":             func() interface{} { return nil },
	"deleteMessageReaction":     func() interface{} { return nil },
	"deleteResponse":            func() interface{} { return nil },
	"deleteTrigger":             func() interface{} { return nil },
	"dict":                      func() interface{} { return nil },
	"div":                       func() interface{} { return nil },
	"editChannelName":           func() interface{} { return nil },
	"editChannelTopic":          func() interface{} { return nil },
	"editMessage":               func() interface{} { return nil },
	"editMessageNoEscape":       func() interface{} { return nil },
	"editNickname":              func() interface{} { return nil },
	"eq":                        func() interface{} { return nil },
	"exec":                      func() interface{} { return nil },
	"execAdmin":                 func() interface{} { return nil },
	"execCC":                    func() interface{} { return nil },
	"execTemplate":              func() interface{} { return nil },
	"fdiv":                      func() interface{} { return nil },
	"formatTime":                func() interface{} { return nil },
	"ge":                        func() interface{} { return nil },
	"getChannel":                func() interface{} { return nil },
	"getChannelOrThread":        func() interface{} { return nil },
	"getMember":                 func() interface{} { return nil },
	"getMessage":                func() interface{} { return nil },
	"getPinCount":               func() interface{} { return nil },
	"getRole":                   func() interface{} { return nil },
	"getTargetPermissionsIn":    func() interface{} { return nil },
	"getThread":                 func() interface{} { return nil },
	"giveRoleID":                func() interface{} { return nil },
	"giveRoleName":              func() interface{} { return nil },
	"gt":                        func() interface{} { return nil },
	"hasPermissions":            func() interface{} { return nil },
	"hasPrefix":                 func() interface{} { return nil },
	"hasRoleID":                 func() interface{} { return nil },
	"hasRoleName":               func() interface{} { return nil },
	"hasSuffix":                 func() interface{} { return nil },
	"html":                      func() interface{} { return nil },
	"humanizeDurationHours":     func() interface{} { return nil },
	"humanizeDurationMinutes":   func() interface{} { return nil },
	"humanizeDurationSeconds":   func() interface{} { return nil },
	"humanizeThousands":         func() interface{} { return nil },
	"humanizeTimeSinceDays":     func() interface{} { return nil },
	"in":                        func() interface{} { return nil },
	"inFold":                    func() interface{} { return nil },
	"index":                     func() interface{} { return nil },
	"joinStr":                   func() interface{} { return nil },
	"js":                        func() interface{} { return nil },
	"json":                      func() interface{} { return nil },
	"kindOf":                    func() interface{} { return nil },
	"le":                        func() interface{} { return nil },
	"len":                       func() interface{} { return nil },
	"loadLocation":              func() interface{} { return nil },
	"log":                       func() interface{} { return nil },
	"lower":                     func() interface{} { return nil },
	"lt":                        func() interface{} { return nil },
	"mathConst":                 func() interface{} { return nil },
	"max":                       func() interface{} { return nil },
	"mentionEveryone":           func() interface{} { return nil },
	"mentionHere":               func() interface{} { return nil },
	"mentionRoleID":             func() interface{} { return nil },
	"mentionRoleName":           func() interface{} { return nil },
	"min":                       func() interface{} { return nil },
	"mod":                       func() interface{} { return nil },
	"mult":                      func() interface{} { return nil },
	"ne":                        func() interface{} { return nil },
	"newDate":                   func() interface{} { return nil },
	"not":                       func() interface{} { return nil },
	"noun":                      func() interface{} { return nil },
	"onlineCount":               func() interface{} { return nil },
	"onlineCountBots":           func() interface{} { return nil },
	"or":                        func() interface{} { return nil },
	"parseArgs":                 func() interface{} { return nil },
	"pastNicknames":             func() interface{} { return nil },
	"pastUsernames":             func() interface{} { return nil },
	"pinMessage":                func() interface{} { return nil },
	"pow":                       func() interface{} { return nil },
	"print":                     func() interface{} { return nil },
	"printf":                    func() interface{} { return nil },
	"println":                   func() interface{} { return nil },
	"randInt":                   func() interface{} { return nil },
	"reFind":                    func() interface{} { return nil },
	"reFindAll":                 func() interface{} { return nil },
	"reFindAllSubmatches":       func() interface{} { return nil },
	"reQuoteMeta":               func() interface{} { return nil },
	"reReplace":                 func() interface{} { return nil },
	"reSplit":                   func() interface{} { return nil },
	"removeRoleID":              func() interface{} { return nil },
	"removeRoleName":            func() interface{} { return nil },
	"roleAbove":                 func() interface{} { return nil },
	"round":                     func() interface{} { return nil },
	"roundCeil":                 func() interface{} { return nil },
	"roundEven":                 func() interface{} { return nil },
	"roundFloor":                func() interface{} { return nil },
	"scheduleUniqueCC":          func() interface{} { return nil },
	"sdict":                     func() interface{} { return nil },
	"sendDM":                    func() interface{} { return nil },
	"sendMessage":               func() interface{} { return nil },
	"sendMessageNoEscape":       func() interface{} { return nil },
	"sendMessageNoEscapeRetID":  func() interface{} { return nil },
	"sendMessageRetID":          func() interface{} { return nil },
	"sendTemplate":              func() interface{} { return nil },
	"sendTemplateDM":            func() interface{} { return nil },
	"seq":                       func() interface{} { return nil },
	"setRoles":                  func() interface{} { return nil },
	"shuffle":                   func() interface{} { return nil },
	"sleep":                     func() interface{} { return nil },
	"slice":                     func() interface{} { return nil },
	"snowflakeToTime":           func() interface{} { return nil },
	"sort":                      func() interface{} { return nil },
	"split":                     func() interface{} { return nil },
	"sqrt":                      func() interface{} { return nil },
	"str":                       func() interface{} { return nil },
	"structToSdict":             func() interface{} { return nil },
	"sub":                       func() interface{} { return nil },
	"takeRoleID":                func() interface{} { return nil },
	"takeRoleName":              func() interface{} { return nil },
	"targetHasPermissions":      func() interface{} { return nil },
	"targetHasRoleID":           func() interface{} { return nil },
	"targetHasRoleName":         func() interface{} { return nil },
	"title":                     func() interface{} { return nil },
	"toByte":                    func() interface{} { return nil },
	"toDuration":                func() interface{} { return nil },
	"toFloat":                   func() interface{} { return nil },
	"toInt":                     func() interface{} { return nil },
	"toInt64":                   func() interface{} { return nil },
	"toRune":                    func() interface{} { return nil },
	"toString":                  func() interface{} { return nil },
	"trimSpace":                 func() interface{} { return nil },
	"unpinMessage":              func() interface{} { return nil },
	"upper":                     func() interface{} { return nil },
	"urlescape":                 func() interface{} { return nil },
	"urlquery":                  func() interface{} { return nil },
	"urlunescape":               func() interface{} { return nil },
	"userArg":                   func() interface{} { return nil },
	"verb":                      func() interface{} { return nil },
	"weekNumber":                func() interface{} { return nil },
}
