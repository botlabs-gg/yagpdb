package dcmd

// SimpleCmd is a Cmd implementation where everythign is a structField
// This can be used if you prefer this way of organising stuff

type SimpleCmd struct {
	ShortDesc, LongDesc string

	CmdArgDefs      []*ArgDef
	RequiredArgDefs int
	ArgDefCombos    [][]int

	CmdSwitches []*ArgDef

	RunFunc func(data *Data) (interface{}, error)
}

func (s *SimpleCmd) Run(data *Data) (interface{}, error) {
	return s.RunFunc(data)
}

func (s *SimpleCmd) Descriptions(data *Data) (short, long string) {
	return s.ShortDesc, s.LongDesc
}

func (s *SimpleCmd) ArgDefs(data *Data) (args []*ArgDef, required int, combos [][]int) {
	return s.CmdArgDefs, s.RequiredArgDefs, s.ArgDefCombos
}

func (s *SimpleCmd) Switches() []*ArgDef {
	return s.CmdSwitches
}
