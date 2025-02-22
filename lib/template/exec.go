// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package template

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"
	"runtime"
	"sort"
	"strings"

	"github.com/botlabs-gg/yagpdb/v2/lib/template/parse"
)

// maxExecDepth specifies the maximum stack depth of templates within
// templates. This limit is only practically reached by accidentally
// recursive template invocations. This limit allows us to return
// an error instead of triggering a stack overflow.
var maxExecDepth = initMaxExecDepth()
var maxStringLength = 1000000

func initMaxExecDepth() int {
	if runtime.GOARCH == "wasm" {
		return 1000
	}
	return 100000
}

// state represents the state of an execution. It's not part of the
// template so that multiple executions of the same template
// can execute in parallel.
type state struct {
	tmpl        *Template
	wr          io.Writer
	node        parse.Node // current node, for errors
	vars        []variable // push-down stack of variable values.
	depth       int        // the height of the stack of executing templates.
	tryDepth    int        // depth of try actions.
	operations  int
	returnValue reflect.Value

	parent *state
}

// variable holds the dynamic value of a variable such as $, $x etc.
type variable struct {
	name  string
	value reflect.Value
}

// push pushes a new variable on the stack.
func (s *state) push(name string, value reflect.Value) {
	s.vars = append(s.vars, variable{name, value})
}

// mark returns the length of the variable stack.
func (s *state) mark() int {
	return len(s.vars)
}

// pop pops the variable stack up to the mark.
func (s *state) pop(mark int) {
	s.vars = s.vars[0:mark]
}

// setVar overwrites the last declared variable with the given name.
// Used by variable assignments.
func (s *state) setVar(name string, value reflect.Value) {
	for i := s.mark() - 1; i >= 0; i-- {
		if s.vars[i].name == name {
			s.vars[i].value = value
			return
		}
	}
	s.errorf("undefined variable: %s", name)
}

// setTopVar overwrites the top-nth variable on the stack. Used by range iterations.
func (s *state) setTopVar(n int, value reflect.Value) {
	s.vars[len(s.vars)-n].value = value
}

// varValue returns the value of the named variable.
func (s *state) varValue(name string) reflect.Value {
	for i := s.mark() - 1; i >= 0; i-- {
		if s.vars[i].name == name {
			return s.vars[i].value
		}
	}
	s.errorf("undefined variable: %s", name)
	return zero
}

var zero reflect.Value

type missingValType struct{}

var missingVal = reflect.ValueOf(missingValType{})

// at marks the state to be on node n, for error reporting.
func (s *state) at(node parse.Node) {
	s.node = node
}

// doublePercent returns the string with %'s replaced by %%, if necessary,
// so it can be used safely inside a Printf format string.
func doublePercent(str string) string {
	return strings.Replace(str, "%", "%%", -1)
}

// TODO: It would be nice if ExecError was more broken down, but
// the way ErrorContext embeds the template name makes the
// processing too clumsy.

// ExecError is the custom error type returned when Execute has an
// error evaluating its template. (If a write error occurs, the actual
// error is returned; it will not be of type ExecError.)
type ExecError struct {
	Name string // Name of template.
	Err  error  // Pre-formatted error.
}

func (e ExecError) Error() string {
	return e.Err.Error()
}

// PassthroughError wraps err in such a way that it will not be stripped of
// information when handled by a try action.
func PassthroughError(err error) error {
	return passthroughError{err}
}

// UncatchableError wraps err in such a way that it will not be handled by a
// try-catch action.
func UncatchableError(err error) error {
	return uncatchableError{err}
}

// uncatchableError is a wrapper type indicating that the wrapped error should
// not be ignored by a try-catch action.
type uncatchableError struct {
	Err error
}

func (u uncatchableError) Error() string {
	return u.Err.Error()
}

// passthroughError is a wrapper type indicating that the wrapped error should
// not be stripped of additional information if handled by a try action.
type passthroughError struct {
	Err error
}

func (p passthroughError) Error() string {
	return p.Err.Error()
}

// funcCallError is the wrapper type used internally when execution of a a try
// action resulted in an error being returned from a function call.
type funcCallError struct {
	Err error // Original error.
}

// Strip removes additional fields and methods from the wrapped error so as to
// not expose sensitive information. If the wrapped error is of type
// passthroughError, Strip simply digs down to the wrapped error without
// performing further sanitization.
func (f funcCallError) Strip() error {
	if p, ok := f.Err.(passthroughError); ok {
		return p.Err
	}
	return errors.New(f.Err.Error())
}

// errorf records an ExecError and terminates processing.
func (s *state) errorf(format string, args ...interface{}) {
	name := doublePercent(s.tmpl.Name())
	if s.node == nil {
		format = fmt.Sprintf("template: %s: %s", name, format)
	} else {
		location, context := s.tmpl.ErrorContext(s.node)
		format = fmt.Sprintf("template: %s: executing %q at <%s>: %s", location, name, doublePercent(context), format)
	}
	panic(ExecError{
		Name: s.tmpl.Name(),
		Err:  fmt.Errorf(format, args...),
	})
}

// writeError is the wrapper type used internally when Execute has an
// error writing to its output. We strip the wrapper in errRecover.
// Note that this is not an implementation of error, so it cannot escape
// from the package as an error value.
type writeError struct {
	Err error // Original error.
}

func (s *state) writeError(err error) {
	panic(writeError{
		Err: err,
	})
}

// errRecover is the handler that turns panics into returns from the top
// level of Parse.
func errRecover(errp *error) {
	e := recover()
	if e != nil {
		switch err := e.(type) {
		case runtime.Error:
			panic(e)
		case writeError:
			*errp = err.Err // Strip the wrapper.
		case ExecError:
			*errp = err // Keep the wrapper.
		default:
			panic(e)
		}
	}
}

// ExecuteTemplate applies the template associated with t that has the given name
// to the specified data object and writes the output to wr.
// If an error occurs executing the template or writing its output,
// execution stops, but partial results may already have been written to
// the output writer.
// A template may be executed safely in parallel, although if parallel
// executions share a Writer the output may be interleaved.
func (t *Template) ExecuteTemplate(wr io.Writer, name string, data interface{}) error {
	var tmpl *Template
	if t.common != nil {
		tmpl = t.tmpl[name]
	}
	if tmpl == nil {
		return fmt.Errorf("template: no template %q associated with template %q", name, t.name)
	}
	return tmpl.Execute(wr, data)
}

// Execute applies a parsed template to the specified data object,
// and writes the output to wr.
// If an error occurs executing the template or writing its output,
// execution stops, but partial results may already have been written to
// the output writer.
// A template may be executed safely in parallel, although if parallel
// executions share a Writer the output may be interleaved.
//
// If data is a reflect.Value, the template applies to the concrete
// value that the reflect.Value holds, as in fmt.Print.
func (t *Template) Execute(wr io.Writer, data interface{}) error {
	return t.execute(wr, data)
}

func (t *Template) execute(wr io.Writer, data interface{}) (err error) {
	defer errRecover(&err)
	value, ok := data.(reflect.Value)
	if !ok {
		value = reflect.ValueOf(data)
	}
	state := &state{
		tmpl: t,
		wr:   wr,
		vars: []variable{{"$", value}},
	}
	if t.Tree == nil || t.Root == nil {
		state.errorf("%q is an incomplete or empty template", t.Name())
	}

	state.walk(value, t.Root)
	return
}

// DefinedTemplates returns a string listing the defined templates,
// prefixed by the string "; defined templates are: ". If there are none,
// it returns the empty string. For generating an error message here
// and in html/template.
func (t *Template) DefinedTemplates() string {
	if t.common == nil {
		return ""
	}
	var b bytes.Buffer
	for name, tmpl := range t.tmpl {
		if tmpl.Tree == nil || tmpl.Root == nil {
			continue
		}
		if b.Len() > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%q", name)
	}
	var s string
	if b.Len() > 0 {
		s = "; defined templates are: " + b.String()
	}
	return s
}

type controlFlowSignal uint8

// A set of signals that mark a disruption in the normal control flow of the program.
const (
	controlFlowNone controlFlowSignal = iota
	controlFlowBreak
	controlFlowContinue
	controlFlowReturnValue
)

// Walk functions step through the major pieces of the template structure,
// generating output as they go.
func (s *state) walk(dot reflect.Value, node parse.Node) controlFlowSignal {
	s.at(node)
	switch node := node.(type) {
	case *parse.ActionNode:
		// Do not pop variables so they persist until next end.
		// Also, if the action declares variables, don't print the result.
		val := s.evalPipeline(dot, node.Pipe)
		if len(node.Pipe.Decl) == 0 {
			s.printValue(node, val)
		}
	case *parse.TryNode:
		return s.walkTry(dot, node.List, node.CatchList)
	case *parse.IfNode:
		return s.walkIfOrWith(parse.NodeIf, dot, node.Pipe, node.List, node.ElseList)
	case *parse.ListNode:
		for _, node := range node.Nodes {
			if signal := s.walk(dot, node); signal != controlFlowNone {
				return signal
			}
		}
	case *parse.RangeNode:
		return s.walkRange(dot, node)
	case *parse.ReturnNode:
		s.returnValue = s.evalPipeline(dot, node.Pipe)
		return controlFlowReturnValue
	case *parse.WhileNode:
		return s.walkWhile(dot, node)
	case *parse.TemplateNode:
		s.walkTemplate(dot, node)
	case *parse.TextNode:
		if _, err := s.wr.Write(node.Text); err != nil {
			s.writeError(err)
		}
	case *parse.WithNode:
		return s.walkIfOrWith(parse.NodeWith, dot, node.Pipe, node.List, node.ElseList)
	case *parse.BreakNode:
		return controlFlowBreak
	case *parse.ContinueNode:
		return controlFlowContinue
	default:
		s.errorf("unknown node: %s", node)
	}

	return controlFlowNone
}

func (s *state) walkTry(dot reflect.Value, list, catchList *parse.ListNode) (sig controlFlowSignal) {
	defer func() {
		s.pop(s.mark())
		s.tryDepth--
		if r := recover(); r != nil {
			switch err := r.(type) {
			case funcCallError:
				sig = s.walk(reflect.ValueOf(err.Strip()), catchList)
			default:
				panic(err)
			}
		}
	}()

	s.tryDepth++
	return s.walk(dot, list)
}

// walkIfOrWith walks an 'if' or 'with' node. The two control structures
// are identical in behavior except that 'with' sets dot.
func (s *state) walkIfOrWith(typ parse.NodeType, dot reflect.Value, pipe *parse.PipeNode, list, elseList *parse.ListNode) controlFlowSignal {
	defer s.pop(s.mark())
	val, _ := indirect(s.evalPipeline(dot, pipe))
	truth, ok := isTrue(val)
	if !ok {
		s.errorf("if/with can't use %v", val)
	}
	if truth {
		if typ == parse.NodeWith {
			return s.walk(val, list)
		} else {
			return s.walk(dot, list)
		}
	} else if elseList != nil {
		return s.walk(dot, elseList)
	}

	return controlFlowNone
}

// IsTrue reports whether the value is 'true', in the sense of not the zero of its type,
// and whether the value has a meaningful truth value. This is the definition of
// truth used by if and other such actions.
func IsTrue(val interface{}) (truth, ok bool) {
	return isTrue(reflect.ValueOf(val))
}

func isTrue(val reflect.Value) (truth, ok bool) {
	if !val.IsValid() {
		// Something like var x interface{}, never set. It's a form of nil.
		return false, true
	}
	switch val.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		truth = val.Len() > 0
	case reflect.Bool:
		truth = val.Bool()
	case reflect.Complex64, reflect.Complex128:
		truth = val.Complex() != 0
	case reflect.Chan, reflect.Func, reflect.Ptr, reflect.Interface:
		truth = !val.IsNil()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		truth = val.Int() != 0
	case reflect.Float32, reflect.Float64:
		truth = val.Float() != 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		truth = val.Uint() != 0
	case reflect.Struct:
		truth = true // Struct values are always true.
	default:
		return
	}
	return truth, true
}

func (s *state) walkRange(dot reflect.Value, r *parse.RangeNode) controlFlowSignal {
	s.incrOPs(1)

	s.at(r)

	defer s.pop(s.mark())
	val, _ := indirect(s.evalPipeline(dot, r.Pipe))
	// mark top of stack before any variables in the body are pushed.
	mark := s.mark()
	oneIteration := func(index, elem reflect.Value) controlFlowSignal {
		s.incrOPs(1)

		// Set top var (lexically the second if there are two) to the element.
		if len(r.Pipe.Decl) > 0 {
			s.setTopVar(1, elem)
		}
		// Set next var (lexically the first if there are two) to the index.
		if len(r.Pipe.Decl) > 1 {
			s.setTopVar(2, index)
		}

		signal := s.walk(elem, r.List)
		s.pop(mark)
		return signal
	}
	switch val.Kind() {
	case reflect.Array, reflect.Slice:
		if val.Len() == 0 {
			break
		}
		for i := 0; i < val.Len(); i++ {
			switch oneIteration(reflect.ValueOf(i), val.Index(i)) {
			case controlFlowBreak:
				return controlFlowNone
			case controlFlowReturnValue:
				return controlFlowReturnValue
			}
		}
		return controlFlowNone
	case reflect.Map:
		if val.Len() == 0 {
			break
		}
		for _, key := range sortKeys(val.MapKeys()) {
			switch oneIteration(key, val.MapIndex(key)) {
			case controlFlowBreak:
				return controlFlowNone
			case controlFlowReturnValue:
				return controlFlowReturnValue
			}
		}
		return controlFlowNone
	case reflect.Chan:
		if val.IsNil() {
			break
		}
		i := 0
		for ; ; i++ {
			elem, ok := val.Recv()
			if !ok {
				break
			}
			switch oneIteration(reflect.ValueOf(i), elem) {
			case controlFlowBreak:
				return controlFlowNone
			case controlFlowReturnValue:
				return controlFlowReturnValue
			}
		}
		if i == 0 {
			break
		}
		return controlFlowNone
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		intVal := int(val.Int())
		if intVal == 0 {
			break
		}
		if intVal < 0 {
			s.errorf("range can't iterate over a negative number")
			return controlFlowNone
		}
		for i := 0; i < intVal; i++ {
			val := reflect.ValueOf(i)
			switch oneIteration(val, val) {
			case controlFlowBreak:
				return controlFlowNone
			case controlFlowReturnValue:
				return controlFlowReturnValue
			}
		}
		return controlFlowNone
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		intVal := int(val.Uint())
		if intVal == 0 {
			break
		}
		if intVal < 0 {
			s.errorf("range can't iterate over a negative number")
			return controlFlowNone
		}
		for i := 0; i < intVal; i++ {
			val := reflect.ValueOf(i)
			switch oneIteration(val, val) {
			case controlFlowBreak:
				return controlFlowNone
			case controlFlowReturnValue:
				return controlFlowReturnValue
			}
		}
		return controlFlowNone
	case reflect.Invalid:
		break // An invalid value is likely a nil map, etc. and acts like an empty map.
	default:
		s.errorf("range can't iterate over %v", val)
	}
	if r.ElseList != nil {
		return s.walk(dot, r.ElseList)
	}
	return controlFlowNone
}

func (s *state) walkWhile(dot reflect.Value, w *parse.WhileNode) controlFlowSignal {
	s.incrOPs(1)

	s.at(w)
	defer s.pop(s.mark())
	// mark top of stack before any variables in the body are pushed.
	mark := s.mark()

	i := 0
	for ; ; i++ {
		s.incrOPs(5)

		val, _ := indirect(s.evalPipeline(dot, w.Pipe))
		truth, ok := isTrue(val)
		if !ok {
			s.errorf("while can't use %v", val)
		}
		if !truth {
			break
		}

		signal := s.walk(dot, w.List)
		s.pop(mark)
		switch signal {
		case controlFlowBreak:
			return controlFlowNone
		case controlFlowReturnValue:
			return controlFlowReturnValue
		}
	}

	if i == 0 && w.ElseList != nil {
		return s.walk(dot, w.ElseList)
	}
	return controlFlowNone
}

func (s *state) walkTemplate(dot reflect.Value, t *parse.TemplateNode) {
	s.at(t)
	s.incrOPs(100)

	tmpl := s.tmpl.tmpl[t.Name]
	if tmpl == nil {
		s.errorf("template %q not defined", t.Name)
	}
	if s.depth == maxExecDepth {
		s.errorf("exceeded maximum template depth (%v)", maxExecDepth)
	}
	// Variables declared by the pipeline persist.
	dot = s.evalPipeline(dot, t.Pipe)
	newState := *s
	newState.parent = s
	newState.depth++
	newState.tmpl = tmpl
	newState.tmpl.maxOps = s.tmpl.maxOps
	// No dynamic scoping: template invocations inherit no variables.
	newState.vars = []variable{{"$", dot}}

	newState.walk(dot, tmpl.Root)
}

// Eval functions evaluate pipelines, commands, and their elements and extract
// values from the data structure by examining fields, calling methods, and so on.
// The printing of those values happens only through walk functions.

// evalPipeline returns the value acquired by evaluating a pipeline. If the
// pipeline has a variable declaration, the variable will be pushed on the
// stack. Callers should therefore pop the stack after they are finished
// executing commands depending on the pipeline value.
func (s *state) evalPipeline(dot reflect.Value, pipe *parse.PipeNode) (value reflect.Value) {
	if pipe == nil {
		return
	}
	s.at(pipe)
	value = missingVal
	for _, cmd := range pipe.Cmds {
		value = s.evalCommand(dot, cmd, value) // previous value is this one's final arg.
		// If the object has type interface{}, dig down one level to the thing inside.
		if value.Kind() == reflect.Interface && value.Type().NumMethod() == 0 {
			value = reflect.ValueOf(value.Interface()) // lovely!
		}
	}
	for _, variable := range pipe.Decl {
		if pipe.IsAssign {
			s.setVar(variable.Ident[0], value)
		} else {
			s.push(variable.Ident[0], value)
		}
	}
	return value
}

func (s *state) notAFunction(args []parse.Node, final reflect.Value) {
	if len(args) > 1 || final != missingVal {
		s.errorf("can't give argument to non-function %s", args[0])
	}
}

func (s *state) evalCommand(dot reflect.Value, cmd *parse.CommandNode, final reflect.Value) reflect.Value {
	firstWord := cmd.Args[0]
	switch n := firstWord.(type) {
	case *parse.FieldNode:
		return s.evalFieldNode(dot, n, cmd.Args, final)
	case *parse.ChainNode:
		return s.evalChainNode(dot, n, cmd.Args, final)
	case *parse.IdentifierNode:
		// Must be a function.
		return s.evalFunction(dot, n, cmd, cmd.Args, final)
	case *parse.PipeNode:
		// Parenthesized pipeline. The arguments are all inside the pipeline; final is ignored.
		s.notAFunction(cmd.Args, final)
		return s.evalPipeline(dot, n)
	case *parse.VariableNode:
		return s.evalVariableNode(dot, n, cmd.Args, final)
	}
	s.at(firstWord)
	s.notAFunction(cmd.Args, final)
	switch word := firstWord.(type) {
	case *parse.BoolNode:
		return reflect.ValueOf(word.True)
	case *parse.DotNode:
		return dot
	case *parse.NilNode:
		s.errorf("nil is not a command")
	case *parse.NumberNode:
		return s.idealConstant(word)
	case *parse.StringNode:
		return reflect.ValueOf(word.Text)
	}
	s.errorf("can't evaluate command %q", firstWord)
	panic("not reached")
}

// idealConstant is called to return the value of a number in a context where
// we don't know the type. In that case, the syntax of the number tells us
// its type, and we use Go rules to resolve. Note there is no such thing as
// a uint ideal constant in this situation - the value must be of int type.
func (s *state) idealConstant(constant *parse.NumberNode) reflect.Value {
	// These are ideal constants but we don't know the type
	// and we have no context.  (If it was a method argument,
	// we'd know what we need.) The syntax guides us to some extent.
	s.at(constant)
	switch {
	case constant.IsComplex:
		return reflect.ValueOf(constant.Complex128) // incontrovertible.
	case constant.IsFloat && !isHexInt(constant.Text) && strings.ContainsAny(constant.Text, ".eEpP"):
		return reflect.ValueOf(constant.Float64)
	case constant.IsInt:
		n := int(constant.Int64)
		if int64(n) != constant.Int64 {
			s.errorf("%s overflows int", constant.Text)
		}
		return reflect.ValueOf(n)
	case constant.IsUint:
		s.errorf("%s overflows int", constant.Text)
	}
	return zero
}

func isHexInt(s string) bool {
	return len(s) > 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X') && !strings.ContainsAny(s, "pP")
}

func (s *state) evalFieldNode(dot reflect.Value, field *parse.FieldNode, args []parse.Node, final reflect.Value) reflect.Value {
	s.at(field)
	return s.evalFieldChain(dot, dot, field, field.Ident, args, final)
}

func (s *state) evalChainNode(dot reflect.Value, chain *parse.ChainNode, args []parse.Node, final reflect.Value) reflect.Value {
	s.at(chain)
	if len(chain.Field) == 0 {
		s.errorf("internal error: no fields in evalChainNode")
	}
	if chain.Node.Type() == parse.NodeNil {
		s.errorf("indirection through explicit nil in %s", chain)
	}
	// (pipe).Field1.Field2 has pipe as .Node, fields as .Field. Eval the pipeline, then the fields.
	pipe := s.evalArg(dot, nil, chain.Node)
	return s.evalFieldChain(dot, pipe, chain, chain.Field, args, final)
}

func (s *state) evalVariableNode(dot reflect.Value, variable *parse.VariableNode, args []parse.Node, final reflect.Value) reflect.Value {
	// $x.Field has $x as the first ident, Field as the second. Eval the var, then the fields.
	s.at(variable)
	value := s.varValue(variable.Ident[0])
	if len(variable.Ident) == 1 {
		s.notAFunction(args, final)
		return value
	}
	return s.evalFieldChain(dot, value, variable, variable.Ident[1:], args, final)
}

// evalFieldChain evaluates .X.Y.Z possibly followed by arguments.
// dot is the environment in which to evaluate arguments, while
// receiver is the value being walked along the chain.
func (s *state) evalFieldChain(dot, receiver reflect.Value, node parse.Node, ident []string, args []parse.Node, final reflect.Value) reflect.Value {
	n := len(ident)
	for i := 0; i < n-1; i++ {
		receiver = s.evalField(dot, ident[i], node, nil, missingVal, receiver)
	}
	// Now if it's a method, it gets the arguments.
	return s.evalField(dot, ident[n-1], node, args, final, receiver)
}

func (s *state) evalFunction(dot reflect.Value, node *parse.IdentifierNode, cmd parse.Node, args []parse.Node, final reflect.Value) reflect.Value {
	s.at(node)
	name := node.Ident
	function, ok := findFunction(name, s.tmpl)
	if !ok {
		s.errorf("%q is not a defined function", name)
	}
	return s.evalCall(dot, function, cmd, name, args, final)
}

// evalField evaluates an expression like (.Field) or (.Field arg1 arg2).
// The 'final' argument represents the return value from the preceding
// value of the pipeline, if any.
func (s *state) evalField(dot reflect.Value, fieldName string, node parse.Node, args []parse.Node, final, receiver reflect.Value) reflect.Value {
	if !receiver.IsValid() {
		if s.tmpl.option.missingKey == mapError { // Treat invalid value as missing map key.
			s.errorf("nil data; no entry for key %q", fieldName)
		}
		return zero
	}
	typ := receiver.Type()
	receiver, isNil := indirect(receiver)
	if receiver.Kind() == reflect.Interface && isNil {
		// Calling a method on a nil interface can't work. The
		// MethodByName method call below would panic.
		s.errorf("nil pointer evaluating %s.%s", typ, fieldName)
		return zero
	}

	// Unless it's an interface, need to get to a value of type *T to guarantee
	// we see all methods of T and *T.
	ptr := receiver
	if ptr.Kind() != reflect.Interface && ptr.Kind() != reflect.Ptr && ptr.CanAddr() {
		ptr = ptr.Addr()
	}
	if method := ptr.MethodByName(fieldName); method.IsValid() {
		return s.evalCall(dot, method, node, fieldName, args, final)
	}
	hasArgs := len(args) > 1 || final != missingVal
	// It's not a method; must be a field of a struct or an element of a map.
	switch receiver.Kind() {
	case reflect.Struct:
		tField, ok := receiver.Type().FieldByName(fieldName)
		if ok {
			field := receiver.FieldByIndex(tField.Index)
			if tField.PkgPath != "" { // field is unexported
				s.errorf("%s is an unexported field of struct type %s", fieldName, typ)
			}
			// If it's a function, we must call it.
			if hasArgs {
				s.errorf("%s has arguments but cannot be invoked as function", fieldName)
			}
			return field
		}
	case reflect.Map:
		// If it's a map, attempt to use the field name as a key.
		nameVal := reflect.ValueOf(fieldName)
		if nameVal.Type().AssignableTo(receiver.Type().Key()) {
			if hasArgs {
				s.errorf("%s is not a method but has arguments", fieldName)
			}
			result := receiver.MapIndex(nameVal)
			if !result.IsValid() {
				switch s.tmpl.option.missingKey {
				case mapInvalid:
					// Just use the invalid value.
				case mapZeroValue:
					result = reflect.Zero(receiver.Type().Elem())
				case mapError:
					s.errorf("map has no entry for key %q", fieldName)
				}
			}
			return result
		}
	case reflect.Ptr:
		etyp := receiver.Type().Elem()
		if etyp.Kind() == reflect.Struct {
			if _, ok := etyp.FieldByName(fieldName); !ok {
				// If there's no such field, say "can't evaluate"
				// instead of "nil pointer evaluating".
				break
			}
		}
		if isNil {
			s.errorf("nil pointer evaluating %s.%s", typ, fieldName)
		}
	}
	s.errorf("can't evaluate field %s in type %s", fieldName, typ)
	panic("not reached")
}

var (
	stringType       = reflect.TypeOf("")
	errorType        = reflect.TypeOf((*error)(nil)).Elem()
	fmtStringerType  = reflect.TypeOf((*fmt.Stringer)(nil)).Elem()
	reflectValueType = reflect.TypeOf((*reflect.Value)(nil)).Elem()
)

// evalCall executes a function or method call. If it's a method, fun already has the receiver bound, so
// it looks just like a function call. The arg list, if non-nil, includes (in the manner of the shell), arg[0]
// as the function itself.
func (s *state) evalCall(dot, fun reflect.Value, node parse.Node, name string, args []parse.Node, final reflect.Value) reflect.Value {
	s.incrOPs(10)

	if args != nil {
		args = args[1:] // Zeroth arg is function name/node; not passed to function.
	}

	typ := fun.Type()
	numIn := len(args)
	if final != missingVal {
		numIn++
	}

	numFixed := len(args)
	if typ.IsVariadic() {
		numFixed = typ.NumIn() - 1 // last arg is the variadic one.
		if numIn < numFixed {
			s.errorf("wrong number of args for %s: want at least %d got %d", name, typ.NumIn()-1, len(args))
		}
	} else if numIn != typ.NumIn() {
		s.errorf("wrong number of args for %s: want %d got %d", name, typ.NumIn(), numIn)
	}
	if !goodFunc(typ) {
		// TODO: This could still be a confusing error; maybe goodFunc should provide info.
		s.errorf("can't call method/function %q with %d results", name, typ.NumOut())
	}

	// Build the arg list.
	argv := make([]reflect.Value, numIn)
	// Args must be evaluated. Fixed args first.
	i := 0
	for ; i < numFixed && i < len(args); i++ {
		argv[i] = s.evalArg(dot, typ.In(i), args[i])
	}
	// Now the ... args.
	if typ.IsVariadic() {
		argType := typ.In(typ.NumIn() - 1).Elem() // Argument is a slice.
		for ; i < len(args); i++ {
			argv[i] = s.evalArg(dot, argType, args[i])
		}
	}
	// Add final value if necessary.
	if final != missingVal {
		t := typ.In(typ.NumIn() - 1)
		if typ.IsVariadic() {
			if numIn-1 < numFixed {
				// The added final argument corresponds to a fixed parameter of the function.
				// Validate against the type of the actual parameter.
				t = typ.In(numIn - 1)
			} else {
				// The added final argument corresponds to the variadic part.
				// Validate against the type of the elements of the variadic slice.
				t = t.Elem()
			}
		}
		argv[i] = s.validateType(final, t)
	}

	// Special case for builtin execTemplate.
	if fun == builtinExecTemplate {
		v := s.callExecTemplate(dot, node, argv)
		if v.Kind() == reflect.String && v.Len() > maxStringLength {
			s.errorf("function response %s exceeds maximum allowed size", name)
		}
		return v
	}

	v, panicked, err := safeCall(fun, argv)
	// If we have an error that is not nil, stop execution and return that
	// error to the caller.
	if err != nil {
		reportError := panicked || s.tryDepth == 0
		if _, ok := err.(uncatchableError); ok {
			reportError = true
		}

		// Stop execution immediately if the error can't be caught, or if it was
		// a panic.
		if reportError {
			s.at(node)
			s.errorf("error calling %s: %v", name, err)
		}
		panic(funcCallError{err})
	}
	if v.Kind() == reflect.String && v.Len() > maxStringLength {
		s.errorf("function response %s exceeds maximum allowed size", name)
	}
	if v.Type() == reflectValueType {
		v = v.Interface().(reflect.Value)
	}
	return v
}

func (s *state) callExecTemplate(dot reflect.Value, node parse.Node, args []reflect.Value) reflect.Value {
	s.at(node)
	s.incrOPs(100)

	if len(args) > 2 {
		s.errorf("too many args for execTemplate: want at most 2 got %d", len(args))
	}
	// evalCall ensures that we have at least one argument and the first is a
	// string, so this is safe.
	name := args[0].String()
	var newDot reflect.Value
	if len(args) > 1 {
		newDot = args[1]
	}

	tmpl := s.tmpl.tmpl[name]
	if tmpl == nil {
		s.errorf("template %q not defined", name)
	}
	if s.depth == maxExecDepth {
		s.errorf("exceeded maximum template depth (%v)", maxExecDepth)
	}

	newState := *s
	newState.parent = s
	newState.depth++
	newState.tmpl = tmpl
	newState.tmpl.maxOps = s.tmpl.maxOps
	newState.vars = []variable{{"$", newDot}}

	if newState.walk(newDot, tmpl.Root) == controlFlowReturnValue {
		return newState.returnValue
	}
	return reflect.Value{}
}

// canBeNil reports whether an untyped nil can be assigned to the type. See reflect.Zero.
func canBeNil(typ reflect.Type) bool {
	switch typ.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return true
	case reflect.Struct:
		return typ == reflectValueType
	}
	return false
}

// validateType guarantees that the value is valid and assignable to the type.
func (s *state) validateType(value reflect.Value, typ reflect.Type) reflect.Value {
	if !value.IsValid() {
		if typ == nil {
			// An untyped nil interface{}. Accept as a proper nil value.
			return reflect.ValueOf(nil)
		}
		if canBeNil(typ) {
			// Like above, but use the zero value of the non-nil type.
			return reflect.Zero(typ)
		}
		s.errorf("invalid value; expected %s", typ)
	}
	if typ == reflectValueType && value.Type() != typ {
		return reflect.ValueOf(value)
	}
	if typ != nil && !value.Type().AssignableTo(typ) {
		if value.Kind() == reflect.Interface && !value.IsNil() {
			value = value.Elem()
			if value.Type().AssignableTo(typ) {
				return value
			}
			// fallthrough
		}
		// Does one dereference or indirection work? We could do more, as we
		// do with method receivers, but that gets messy and method receivers
		// are much more constrained, so it makes more sense there than here.
		// Besides, one is almost always all you need.
		switch {
		case value.Kind() == reflect.Ptr && value.Type().Elem().AssignableTo(typ):
			value = value.Elem()
			if !value.IsValid() {
				s.errorf("dereference of nil pointer of type %s", typ)
			}
		case reflect.PtrTo(value.Type()).AssignableTo(typ) && value.CanAddr():
			value = value.Addr()
		default:
			kind1, _ := basicKindT(typ.Kind())
			kind2, _ := basicKindT(value.Kind())

			if kind1 == intKind && kind2 == intKind {
				vc := value.Int()
				switch typ.Kind() {
				case reflect.Int:
					return reflect.ValueOf(int(vc))
				case reflect.Int8:
					return reflect.ValueOf(int8(vc))
				case reflect.Int16:
					return reflect.ValueOf(int16(vc))
				case reflect.Int32:
					return reflect.ValueOf(int32(vc))
				case reflect.Int64:
					return reflect.ValueOf(vc)
				}
			} else if kind1 == uintKind && kind2 == uintKind {
				vc := value.Uint()
				switch typ.Kind() {
				case reflect.Uint:
					return reflect.ValueOf(uint(vc))
				case reflect.Uint8:
					return reflect.ValueOf(uint8(vc))
				case reflect.Uint16:
					return reflect.ValueOf(uint16(vc))
				case reflect.Uint32:
					return reflect.ValueOf(uint32(vc))
				case reflect.Uint64:
					return reflect.ValueOf(uint64(vc))
				}
			} else if kind1 == floatKind && kind2 == floatKind {
				vc := value.Float()
				switch typ.Kind() {
				case reflect.Float64:
					return reflect.ValueOf(float64(vc))
				case reflect.Float32:
					return reflect.ValueOf(float32(vc))
				}
			}

			s.errorf("wrong type for value; expected %s; got %s", typ, value.Type())
		}
	}
	return value
}

func (s *state) evalArg(dot reflect.Value, typ reflect.Type, n parse.Node) reflect.Value {
	s.at(n)
	switch arg := n.(type) {
	case *parse.DotNode:
		return s.validateType(dot, typ)
	case *parse.NilNode:
		if canBeNil(typ) {
			return reflect.Zero(typ)
		}
		s.errorf("cannot assign nil to %s", typ)
	case *parse.FieldNode:
		return s.validateType(s.evalFieldNode(dot, arg, []parse.Node{n}, missingVal), typ)
	case *parse.VariableNode:
		return s.validateType(s.evalVariableNode(dot, arg, nil, missingVal), typ)
	case *parse.PipeNode:
		return s.validateType(s.evalPipeline(dot, arg), typ)
	case *parse.IdentifierNode:
		return s.validateType(s.evalFunction(dot, arg, arg, nil, missingVal), typ)
	case *parse.ChainNode:
		return s.validateType(s.evalChainNode(dot, arg, nil, missingVal), typ)
	}
	switch typ.Kind() {
	case reflect.Bool:
		return s.evalBool(typ, n)
	case reflect.Complex64, reflect.Complex128:
		return s.evalComplex(typ, n)
	case reflect.Float32, reflect.Float64:
		return s.evalFloat(typ, n)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return s.evalInteger(typ, n)
	case reflect.Interface:
		if typ.NumMethod() == 0 {
			return s.evalEmptyInterface(dot, n)
		}
	case reflect.Struct:
		if typ == reflectValueType {
			return reflect.ValueOf(s.evalEmptyInterface(dot, n))
		}
	case reflect.String:
		return s.evalString(typ, n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return s.evalUnsignedInteger(typ, n)
	}
	s.errorf("can't handle %s for arg of type %s", n, typ)
	panic("not reached")
}

func (s *state) evalBool(typ reflect.Type, n parse.Node) reflect.Value {
	s.at(n)
	if n, ok := n.(*parse.BoolNode); ok {
		value := reflect.New(typ).Elem()
		value.SetBool(n.True)
		return value
	}
	s.errorf("expected bool; found %s", n)
	panic("not reached")
}

func (s *state) evalString(typ reflect.Type, n parse.Node) reflect.Value {
	s.at(n)
	if n, ok := n.(*parse.StringNode); ok {
		value := reflect.New(typ).Elem()
		value.SetString(n.Text)
		return value
	}
	s.errorf("expected string; found %s", n)
	panic("not reached")
}

func (s *state) evalInteger(typ reflect.Type, n parse.Node) reflect.Value {
	s.at(n)
	if n, ok := n.(*parse.NumberNode); ok && n.IsInt {
		value := reflect.New(typ).Elem()
		value.SetInt(n.Int64)
		return value
	}
	s.errorf("expected integer; found %s", n)
	panic("not reached")
}

func (s *state) evalUnsignedInteger(typ reflect.Type, n parse.Node) reflect.Value {
	s.at(n)
	if n, ok := n.(*parse.NumberNode); ok && n.IsUint {
		value := reflect.New(typ).Elem()
		value.SetUint(n.Uint64)
		return value
	}
	s.errorf("expected unsigned integer; found %s", n)
	panic("not reached")
}

func (s *state) evalFloat(typ reflect.Type, n parse.Node) reflect.Value {
	s.at(n)
	if n, ok := n.(*parse.NumberNode); ok && n.IsFloat {
		value := reflect.New(typ).Elem()
		value.SetFloat(n.Float64)
		return value
	}
	s.errorf("expected float; found %s", n)
	panic("not reached")
}

func (s *state) evalComplex(typ reflect.Type, n parse.Node) reflect.Value {
	if n, ok := n.(*parse.NumberNode); ok && n.IsComplex {
		value := reflect.New(typ).Elem()
		value.SetComplex(n.Complex128)
		return value
	}
	s.errorf("expected complex; found %s", n)
	panic("not reached")
}

func (s *state) evalEmptyInterface(dot reflect.Value, n parse.Node) reflect.Value {
	s.at(n)
	switch n := n.(type) {
	case *parse.BoolNode:
		return reflect.ValueOf(n.True)
	case *parse.DotNode:
		return dot
	case *parse.FieldNode:
		return s.evalFieldNode(dot, n, nil, missingVal)
	case *parse.IdentifierNode:
		return s.evalFunction(dot, n, n, nil, missingVal)
	case *parse.NilNode:
		// NilNode is handled in evalArg, the only place that calls here.
		s.errorf("evalEmptyInterface: nil (can't happen)")
	case *parse.NumberNode:
		return s.idealConstant(n)
	case *parse.StringNode:
		return reflect.ValueOf(n.Text)
	case *parse.VariableNode:
		return s.evalVariableNode(dot, n, nil, missingVal)
	case *parse.PipeNode:
		return s.evalPipeline(dot, n)
	}
	s.errorf("can't handle assignment of %s to empty interface argument", n)
	panic("not reached")
}

// indirect returns the item at the end of indirection, and a bool to indicate
// if it's nil. If the returned bool is true, the returned value's kind will be
// either a pointer or interface.
func indirect(v reflect.Value) (rv reflect.Value, isNil bool) {
	for ; v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface; v = v.Elem() {
		if v.IsNil() {
			return v, true
		}
	}
	return v, false
}

// indirectInterface returns the concrete value in an interface value,
// or else the zero reflect.Value.
// That is, if v represents the interface value x, the result is the same as reflect.ValueOf(x):
// the fact that x was an interface value is forgotten.
func indirectInterface(v reflect.Value) reflect.Value {
	if v.Kind() != reflect.Interface {
		return v
	}
	if v.IsNil() {
		return reflect.Value{}
	}
	return v.Elem()
}

// printValue writes the textual representation of the value to the output of
// the template.
func (s *state) printValue(n parse.Node, v reflect.Value) {
	s.at(n)
	iface, ok := printableValue(v)
	if !ok {
		s.errorf("can't print %s of type %s", n, v.Type())
	}
	_, err := fmt.Fprint(s.wr, iface)
	if err != nil {
		s.writeError(err)
	}
}

func (s *state) incrOPs(num int) {
	if s.tmpl.maxOps == 0 {
		return
	}

	if s.parent != nil {
		s.parent.incrOPs(num)
		return
	}

	s.operations += num
	if s.operations > s.tmpl.maxOps {
		s.errorf("exceeded max operations (%d/%d)", s.operations, s.tmpl.maxOps)
	}
}

// printableValue returns the, possibly indirected, interface value inside v that
// is best for a call to formatted printer.
func printableValue(v reflect.Value) (interface{}, bool) {
	if v.Kind() == reflect.Ptr {
		v, _ = indirect(v) // fmt.Fprint handles nil.
	}
	if !v.IsValid() {
		return "<no value>", true
	}

	if !v.Type().Implements(errorType) && !v.Type().Implements(fmtStringerType) {
		if v.CanAddr() && (reflect.PtrTo(v.Type()).Implements(errorType) || reflect.PtrTo(v.Type()).Implements(fmtStringerType)) {
			v = v.Addr()
		} else {
			switch v.Kind() {
			case reflect.Chan, reflect.Func:
				return nil, false
			}
		}
	}
	return v.Interface(), true
}

// sortKeys sorts (if it can) the slice of reflect.Values, which is a slice of map keys.
func sortKeys(v []reflect.Value) []reflect.Value {
	if len(v) <= 1 {
		return v
	}
	switch v[0].Kind() {
	case reflect.Float32, reflect.Float64:
		sort.Slice(v, func(i, j int) bool {
			return v[i].Float() < v[j].Float()
		})
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		sort.Slice(v, func(i, j int) bool {
			return v[i].Int() < v[j].Int()
		})
	case reflect.String:
		sort.Slice(v, func(i, j int) bool {
			return v[i].String() < v[j].String()
		})
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		sort.Slice(v, func(i, j int) bool {
			return v[i].Uint() < v[j].Uint()
		})
	}
	return v
}
