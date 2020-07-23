package cplogs

type ActionFormat struct {
	Key          string
	FormatString string
}

var actionFormats = make(map[string]*ActionFormat)

// RegisterActionFormat sets up a action format, call this in your package init function
func RegisterActionFormat(format *ActionFormat) {
	actionFormats[format.Key] = format
}
