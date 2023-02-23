package common

import (
	"strings"
)

// Oh boy.
var DeSpoofReplacer = strings.NewReplacer(
	// Latin
	"\u0430", "a",
	"\u0441", "c",
	"\u0435", "e",
	"\u0456", "i",
	"\u0458", "j",
	"\u043E", "o",
	"\u0440", "p",
	"\u0455", "s",
	"\u0445", "x",
	"\u0443", "y",
	"\u0386", "A",
	"\u0391", "A",
	"\u0410", "A",
	"\u0392", "B",
	"\u0412", "B",
	"\u0421", "C",
	"\u0395", "E",
	"\u0388", "E",
	"\u0415", "E",
	"\u0389", "H",
	"\u041D", "H",
	"\u0397", "H",
	"\u038A", "I",
	"\u0406", "I",
	"\u0399", "I",
	"\u039A", "K",
	"\u03BA", "K",
	"\u039C", "M",
	"\u041C", "M",
	"\u039D", "N",
	"\u041E", "O",
	"\u039F", "O",
	"\u0420", "P",
	"\u03A1", "P",
	"\u0405", "S",
	"\u0422", "T",
	"\u03A4", "T",
	"\u0425", "X",
	"\u03A7", "X",
	"\u03A5", "Y",
	"\u038E", "Y",
	"\u0396", "Z",
	"\u01C3", "!",
	// Punctuation
	"\u2024", ".",
	"\u037E", ";",
	"\u201A", ",",
	"\u2010", "-",
	// Fake spaces
	"\u2000", " ",
	"\u2001", " ",
	"\u2002", " ",
	"\u2003", " ",
	"\u2004", " ",
	"\u2005", " ",
	"\u2006", " ",
	"\u2007", " ",
	"\u2008", " ",
	"\u2009", " ",
	// Invisible characters
	"\u200A", "",
	"\u200B", "",
	"\u200C", "",
	"\u200D", "",
	"\u2060", "",
	"\uFEFF", "",
)

func DeSpoof(input string) string {
	return DeSpoofReplacer.Replace(input)
}
