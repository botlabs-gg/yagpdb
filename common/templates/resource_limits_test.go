package templates

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/lib/template"
)

func TestPrintfWidthCountsTowardsLimit(t *testing.T) {
	printf := withOutputLimitF(fmt.Sprintf, MaxStringLength)

	format := strings.Repeat("%1000000[1]d", 50)
	if _, err := printf(format, 1); err == nil {
		t.Fatal("expected a format that pads to 50MB to be rejected")
	}

	if _, err := printf("%*d", 5000000, 1); err == nil {
		t.Fatal("expected a * width beyond the limit to be rejected")
	}

	out, err := printf("%5.2f|%-8s|%03d|%%|%[1]v", 3.14159, "ab", 7)
	if err != nil {
		t.Fatalf("normal format rejected: %v", err)
	}
	if out != " 3.14|ab      |007|%|3.14159" {
		t.Fatalf("unexpected output %q", out)
	}
}

func aliasedSlice(depth int) Slice {
	x := Slice{"a"}
	for range depth {
		x = Slice{x, x}
	}
	return x
}

func TestCycleCheckIsLinearOnAliasedValues(t *testing.T) {
	dag := aliasedSlice(60)

	start := time.Now()
	if _, err := (Slice{}).Append(dag); err != nil {
		t.Fatalf("an aliased but acyclic value was reported as a cycle: %v", err)
	}
	d := SDict{}
	if _, err := d.Set("k", dag); err != nil {
		t.Fatalf("an aliased but acyclic value was reported as a cycle: %v", err)
	}
	if took := time.Since(start); took > time.Second {
		t.Fatalf("cycle check took %s on an aliased value", took)
	}
}

func TestCycleCheckStillFindsCycles(t *testing.T) {
	s := Slice{"a", "b"}
	if _, err := s.Set(0, s); err == nil {
		t.Fatal("expected a slice containing itself to be rejected")
	}

	inner := SDict{}
	outer := SDict{"inner": inner}
	if _, err := inner.Set("outer", outer); err == nil {
		t.Fatal("expected two dicts containing each other to be rejected")
	}
}

func TestCycleCheckFollowsMapKeys(t *testing.T) {
	builder := &ComponentBuilder{}
	d := Dict{}
	if _, err := d.Set(builder, "v"); err != nil {
		t.Fatalf("an acyclic builder key was reported as a cycle: %v", err)
	}
	if _, err := builder.Add("k", d); err == nil {
		t.Fatal("expected a builder holding a dict keyed by itself to be rejected")
	}

	builder = &ComponentBuilder{}
	d = Dict{}
	if _, err := builder.Add("k", d); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := d.Set(builder, "v"); err == nil {
		t.Fatal("expected keying a dict by a builder that holds it to be rejected")
	}

	shared := &ComponentBuilder{}
	if _, err := (Dict{shared: 1, "a": shared}).Set("b", Slice{shared}); err != nil {
		t.Fatalf("an aliased but acyclic key was reported as a cycle: %v", err)
	}
}

func TestSendTemplateDataCheckIsLinear(t *testing.T) {
	c := &Context{Data: map[string]any{"x": 1}}

	start := time.Now()
	if c.containsContextData(aliasedSlice(60)) {
		t.Fatal("unexpected match")
	}
	if took := time.Since(start); took > time.Second {
		t.Fatalf("context data check took %s on an aliased value", took)
	}

	if !c.containsContextData(SDict{"a": Slice{c.Data}}) {
		t.Fatal("expected the context data to be found")
	}
}

func TestSplitIsChargedAgainstAllocLimit(t *testing.T) {
	tmpl := template.Must(template.New("t").Funcs(map[string]any{"split": strings.Split}).Parse(`{{$x := split .S ""}}`)).
		MaxAllocBytes(1 << 20)

	_, err := tmpl.ExecuteCountingAlloc(&bytes.Buffer{}, map[string]any{"S": strings.Repeat("a", 1<<20)})
	if err == nil || !strings.Contains(err.Error(), "exceeded max memory allocation") {
		t.Fatalf("expected split into 1M strings to exceed a 1MB budget, got %v", err)
	}
}

func TestRangeErrorDoesNotFormatTheValue(t *testing.T) {
	tmpl := template.Must(template.New("t").Parse(`{{range .}}{{end}}`))

	start := time.Now()
	_, err := tmpl.ExecuteCountingAlloc(&bytes.Buffer{}, &ComponentBuilder{Values: []any{aliasedSlice(60)}})
	if err == nil || !strings.Contains(err.Error(), "range can't iterate over templates.ComponentBuilder") {
		t.Fatalf("unexpected error %v", err)
	}
	if took := time.Since(start); took > time.Second {
		t.Fatalf("range error took %s", took)
	}
}

func TestNestedExecutionSharesBudget(t *testing.T) {
	run := func(n int) error {
		c := &Context{Data: map[string]any{"S": make([]int, n)}, CurrentFrame: &ContextFrame{}}
		nested := func() (string, error) {
			parent := c.CurrentFrame.parsedTemplate
			defer func() { c.CurrentFrame.parsedTemplate = parent }()
			c.CurrentFrame.parsedTemplate = parent.Lookup("child")
			return c.executeParsed()
		}
		tmpl, err := template.New("t").Funcs(map[string]any{"nested": nested}).
			Parse(`{{define "child"}}{{range .S}}{{end}}{{end}}{{range .S}}{{end}}{{nested}}{{nested}}`)
		if err != nil {
			t.Fatal(err)
		}
		c.CurrentFrame.parsedTemplate = tmpl
		_, err = c.executeParsed()
		return err
	}

	if err := run(MaxOpsNormal / 6); err != nil {
		t.Fatalf("nested executions within the budget failed: %v", err)
	}
	if err := run(MaxOpsNormal / 2); err == nil || !strings.Contains(err.Error(), "exceeded max operations") {
		t.Fatalf("expected nested executions to share one op budget, got %v", err)
	}
}

func TestDescribeInputDoesNotExpandContainers(t *testing.T) {
	if got := describeInput(aliasedSlice(60)); got != "templates.Slice" {
		t.Fatalf("unexpected %q", got)
	}
	if got := describeInput("role name"); got != "role name" {
		t.Fatalf("unexpected %q", got)
	}
}
