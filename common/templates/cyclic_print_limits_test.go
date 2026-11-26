package templates

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/lib/template"
)

func builderWithButton(t *testing.T) *ComponentBuilder {
	t.Helper()
	c := newTestComponents(t)
	b, err := CreateComponentBuilder("buttons", Slice{c.button})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestFixBuilderSetSelfRejected(t *testing.T) {
	b := builderWithButton(t)
	_, err := b.Set("click", b)
	t.Logf("Set self: err=%v", err)
	if err == nil {
		t.Fatal("expected Set to reject the builder containing itself")
	}
}

func TestFixBuilderSetSelfViaSDictRejected(t *testing.T) {
	b := builderWithButton(t)
	_, err := b.Set("click", SDict{"inner": b})
	t.Logf("Set sdict{self}: err=%v", err)
	if err == nil {
		t.Fatal("expected Set to reject a dict containing the builder")
	}
}

func TestFixBuilderAddSelfRejected(t *testing.T) {
	b := builderWithButton(t)
	_, err := b.Add("text", b)
	t.Logf("Add self: err=%v", err)
	if err == nil {
		t.Fatal("expected Add to reject the builder containing itself")
	}
}

func TestFixBuilderSiblingsStillAllowed(t *testing.T) {
	b := builderWithButton(t)
	inner := builderWithButton(t)
	if _, err := b.Add("container", SDict{"a": inner, "b": inner}); err != nil {
		t.Fatalf("the same builder used twice as siblings is not a cycle: %v", err)
	}
}

func TestFixEncodedSizeCatchesCycle(t *testing.T) {
	b := builderWithButton(t)
	b.Values = append(b.Values, b)
	b.Components = append(b.Components, "text")
	if !ExceedsEncodedSize(b, 100000) {
		t.Fatal("expected the size check to stop a cyclic builder before msgpack encodes it")
	}
}

func TestFixNestedSlicePrintAborts(t *testing.T) {
	src := `{{$a := cslice 1}}{{range seq 0 60}}{{$a = cslice $a $a}}{{end}}{{$a}}`
	tmpl := template.Must(template.New("eval").Funcs(map[string]any{"cslice": CreateSlice, "seq": sequence}).Parse(src)).
		MaxOps(MaxOpsEvalNormal).MaxAllocBytes(MaxAllocEvalNormal).PrintCheck(checkPrintSize)

	var buf bytes.Buffer
	start := time.Now()
	_, err := tmpl.ExecuteCountingAlloc(LimitWriter(&buf, maxOutputLength), nil)
	took := time.Since(start)
	t.Logf("took=%s err=%v", took, err)
	if err == nil || !strings.Contains(err.Error(), "response grew too big") {
		t.Fatalf("expected the print to be rejected, got %v", err)
	}
	if took > time.Second {
		t.Fatalf("rejection took too long: %s", took)
	}
}

func TestFixReportedNestedPrintPayload(t *testing.T) {
	src := `{{$x := cslice "A"}}{{range seq 0 40}}{{$x = cslice $x $x}}{{end}}{{$x}}`
	tmpl := template.Must(template.New("eval").Funcs(map[string]any{"cslice": CreateSlice, "seq": sequence}).Parse(src)).
		MaxOps(MaxOpsEvalNormal).MaxAllocBytes(MaxAllocEvalNormal).PrintCheck(checkPrintSize)

	var buf bytes.Buffer
	_, err := tmpl.ExecuteCountingAlloc(LimitWriter(&buf, maxOutputLength), nil)
	if err == nil || !strings.Contains(err.Error(), "response grew too big") {
		t.Fatalf("expected the reported payload to be rejected, got %v", err)
	}
}

func TestFixReportedBuilderInsideOwnContainerPayload(t *testing.T) {
	cb, err := CreateComponentBuilder()
	if err != nil {
		t.Fatal(err)
	}
	c, err := StringKeyDictionary("components", cb)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cb.Add("container", c); err == nil {
		t.Fatal("expected Add to reject a container holding the builder itself")
	}
}

func TestFixNormalPrintUnaffected(t *testing.T) {
	src := `{{$a := cslice 1 "two" (cslice 3 4)}}{{$a}} {{sdict "k" "v"}}`
	tmpl := template.Must(template.New("ok").Funcs(map[string]any{"cslice": CreateSlice, "sdict": StringKeyDictionary}).Parse(src)).
		PrintCheck(checkPrintSize)

	var buf bytes.Buffer
	if _, err := tmpl.ExecuteCountingAlloc(&buf, nil); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "[1 two [3 4]] map[k:v]" {
		t.Fatalf("unexpected output %q", got)
	}
}
