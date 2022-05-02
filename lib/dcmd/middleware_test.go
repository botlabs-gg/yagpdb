package dcmd

import (
	"testing"
)

func TestMiddlewareOrder(t *testing.T) {
	container := &Container{}

	i := 0

	container.AddMidlewares(func(inner RunFunc) RunFunc {
		return func(d *Data) (interface{}, error) {
			// This middleware should be ran first
			if i != 0 {
				t.Error("Unordered:", i, "expected 0")
			}
			i = 1
			t.Log("mw 1")
			return inner(d)
		}
	}, func(inner RunFunc) RunFunc {
		return func(d *Data) (interface{}, error) {
			// This middleware should be ran second
			if i != 1 {
				t.Error("Unordered:", i, "expected 1")
			}
			i = 2
			t.Log("mw 2")
			return inner(d)
		}
	})

	cmd := &TestCommand{}
	container.AddCommand(cmd, NewTrigger("test").SetMiddlewares(func(inner RunFunc) RunFunc {
		return func(d *Data) (interface{}, error) {
			// This middleware should be ran before cmd1, but not for anything else
			if i != 2 {
				t.Error("Unordered:", i, "expected 2")
			}
			i = 3
			t.Log("mw 3 cmd")
			return inner(d)
		}
	}))

	sub, _ := container.Sub("sub")
	sub.AddMidlewares(func(inner RunFunc) RunFunc {
		return func(d *Data) (interface{}, error) {
			// This middleware should be ran first in sub1, after the parent container
			if i != 2 {
				t.Error("Unordered:", i, "expected 2")
			}
			i = 3
			t.Log("mw 4 sub mw")
			return inner(d)
		}
	})

	sub.AddCommand(cmd, NewTrigger("test").SetMiddlewares(func(inner RunFunc) RunFunc {
		return func(d *Data) (interface{}, error) {
			// This middleware should be ran before cmd in the sub container, but not for anything else
			if i != 3 {
				t.Error("Unordered:", i, "expected 3")
			}
			i = 4
			t.Log("mw 5 sub cmd")
			return inner(d)
		}
	}))

	doTest := func() {
		data1 := &Data{
			TraditionalTriggerData: &TraditionalTriggerData{
				MessageStrippedPrefix: "test",
			},
			Source:      TriggerSourceGuild,
			TriggerType: TriggerTypePrefix,
		}
		data2 := &Data{
			TraditionalTriggerData: &TraditionalTriggerData{
				MessageStrippedPrefix: "sub test",
			},
			Source:      TriggerSourceGuild,
			TriggerType: TriggerTypePrefix,
		}

		i = 0
		container.Run(data1)
		i = 0
		resp, _ := container.Run(data2)
		if i != 4 {
			t.Error("i: ", i, "Expected 4")
		}
		if resp != TestResponse {
			t.Error("Response: ", resp, ", Expected: ", TestResponse)
		}
	}

	doTest()
	t.Log("Testing prebuilt chains")
	container.BuildMiddlewareChains(nil)
	doTest()
}
