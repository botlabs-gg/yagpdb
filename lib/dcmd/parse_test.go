package dcmd

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseArgDefs(t *testing.T) {
	cases := []struct {
		name         string
		input        string
		defs         []*ArgDef
		expectedArgs []*ParsedArg
	}{
		{"simple int", "15", []*ArgDef{{Type: Int}}, []*ParsedArg{{Value: int64(15)}}},
		{"simple float", "15.5", []*ArgDef{{Type: Float}}, []*ParsedArg{{Value: float64(15.5)}}},
		{"simple string", "hello", []*ArgDef{{Type: String}}, []*ParsedArg{{Value: "hello"}}},
		{"int float", "15 30.5", []*ArgDef{{Type: Int}, {Type: Float}}, []*ParsedArg{{Value: int64(15)}, {Value: float64(30.5)}}},
		{"string int", "hey_man 30", []*ArgDef{{Type: String}, {Type: Int}}, []*ParsedArg{{Value: "hey_man"}, {Value: int64(30)}}},
		{"quoted strings", "first `middle quoted` last", []*ArgDef{{Type: String}, {Type: String}, {Type: String}}, []*ParsedArg{{Value: "first"}, {Value: "middle quoted"}, {Value: "last"}}},
		{"escape space", "first\\ still\\ first second", []*ArgDef{{Type: String}, {Type: String}}, []*ParsedArg{{Value: "first still first"}, {Value: "second"}}},
		{"escape container", "`first \\` still first` second", []*ArgDef{{Type: String}, {Type: String}}, []*ParsedArg{{Value: "first ` still first"}, {Value: "second"}}},
		{"keep escape character", "first\\n second", []*ArgDef{{Type: String}, {Type: String}}, []*ParsedArg{{Value: "first\\n"}, {Value: "second"}}},
	}

	for i, v := range cases {
		t.Run(fmt.Sprintf("#%d-%s", i, v.name), func(t *testing.T) {
			d := new(Data)
			err := ParseArgDefs(v.defs, 0, nil, d, SplitArgs(v.input))

			if err != nil {
				t.Fatal("ParseArgDefs returned a bad error", err)
			}

			// Check if we got the expected output
			for i, ea := range v.expectedArgs {
				if i >= len(d.Args) {
					t.Fatal("Unexpected end of parsed args")
				}

				if !assert.Equal(t, ea.Value, d.Args[i].Value, "Should be equal") {
					for ei, ga := range d.Args {
						t.Errorf("Parsed arg[%d]: %v", ei, ga.Value)
					}
				}
			}
		})
	}
}

var Sink int

func BenchmarkSplitArgs(b *testing.B) {
	benchmarks := []struct {
		name string
		in   string
	}{
		{"very short input", "-xkcd"},
		{"short input", "-send bar baz"},
		{"medium-length input", "-simpleembed -title abc -desc xyz -color ff0000"},
		{"medium-length input with quoted text", `-simpleembed -title "foo bar baz" -desc "hello world"`},
		{"long input", `-ce {
			"title": "example title",
			"color" 1234,
			"description": "very interesting test case\n",
			"timestamp": "",
			"author": {},
			"image": { "url": "" }
		}`},
	}
	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				Sink += len(SplitArgs(bm.in))
			}
		})
	}
}

func TestParseSwitches(t *testing.T) {
	cases := []struct {
		name         string
		input        string
		defs         []*ArgDef
		expectedArgs []*ParsedArg
	}{
		{"simple int", "-i 15", []*ArgDef{{Name: "i", Type: Int}}, []*ParsedArg{{Value: int64(15)}}},
		{"simple float", "-f 15.5", []*ArgDef{{Name: "f", Type: Float}}, []*ParsedArg{{Value: float64(15.5)}}},
		{"simple string", "-s hello", []*ArgDef{{Name: "s", Type: String}}, []*ParsedArg{{Value: "hello"}}},
		{"simple string, long switch", "-string hello", []*ArgDef{{Name: "string", Type: String}}, []*ParsedArg{{Value: "hello"}}},
		{"int float", "-i 15 -f 30.5", []*ArgDef{{Name: "i", Type: Int}, {Name: "f", Type: Float}}, []*ParsedArg{{Value: int64(15)}, {Value: float64(30.5)}}},
		{"string int", "-s hey_man -i 30", []*ArgDef{{Name: "s", Type: String}, {Name: "i", Type: Int}}, []*ParsedArg{{Value: "hey_man"}, {Value: int64(30)}}},
		{"quoted strings", "-s1 first -s2 `middle quoted` -s3 last", []*ArgDef{{Name: "s1", Type: String}, {Name: "s2", Type: String}, {Name: "s3", Type: String}}, []*ParsedArg{{Value: "first"}, {Value: "middle quoted"}, {Value: "last"}}},
	}

	for i, v := range cases {
		t.Run(fmt.Sprintf("#%d-%s", i, v.name), func(t *testing.T) {
			d := new(Data)

			_, err := ParseSwitches(v.defs, d, SplitArgs(v.input))
			if err != nil {
				t.Fatal("ParseArgDefs returned a bad error", err)
			}

			// Check if we got the expected output
			for i, ea := range v.expectedArgs {
				assert.Equal(t, ea.Value, d.Switches[v.defs[i].Name].Value, "Should be equal")
			}
		})
	}
}
