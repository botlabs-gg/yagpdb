package dcmd

import "testing"

func TestSplitString(t *testing.T) {
	cases := []struct {
		input  string
		output []string
	}{
		{
			input:  "test",
			output: []string{"test"},
		},
		{
			input:  "t1_t2_t3\nt5_t6_t7",
			output: []string{"t1_t2_t3", "t5_t6_t7"},
		},
		{
			input:  "t1_t2_t3\nt5_t6_t7_t8_t9",
			output: []string{"t1_t2_t3", "t5_t6_t7_t", "8_t9"},
		},
		{
			input:  "t1_t2_t3\n t5_t6_t7 ",
			output: []string{"t1_t2_t3", "t5_t6_t7"},
		},
	}

	for i, c := range cases {
		result := SplitString(c.input, 10)

		if len(result) != len(c.output) {
			t.Errorf("bad len on case %d, got (%d) %v, expected (%d) %v", i, len(result), result, len(c.output), c.output)
			continue
		}

		for j, v := range result {
			if c.output[j] != v {
				t.Errorf("bad output on case %d, got %+v, expected %+v", i, result, c.output)
				break
			}
		}
	}
}