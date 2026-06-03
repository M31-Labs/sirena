package mermaid

import "testing"

// TestClassify covers the three unambiguous compatibility outcomes: a clean
// flowchart renders Full, a styling directive degrades to StyleDropped (the
// geometry survives, the styling is dropped), and a non-flowchart diagram
// type is Unsupported.
func TestClassify(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want Compat
	}{
		{
			name: "clean flowchart",
			src:  "graph TD\n  A --> B\n",
			want: CompatFull,
		},
		{
			name: "styling directive dropped",
			src:  "graph TD\n  A --> B\n  style A fill:#f00\n",
			want: CompatStyleDropped,
		},
		{
			name: "non-flowchart diagram type",
			src:  "sequenceDiagram\n  A->>B: hi\n",
			want: CompatUnsupported,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := Classify([]byte(tc.src))
			if got != tc.want {
				t.Errorf("Classify(%q) = %v, want %v", tc.src, got, tc.want)
			}
		})
	}
}

// TestCompat_String pins the stable lowercase labels used in the published
// compatibility matrix.
func TestCompat_String(t *testing.T) {
	want := map[Compat]string{
		CompatFull:         "full",
		CompatStyleDropped: "styling-dropped",
		CompatPartial:      "partial",
		CompatUnsupported:  "unsupported",
	}
	for c, label := range want {
		if got := c.String(); got != label {
			t.Errorf("Compat(%d).String() = %q, want %q", int(c), got, label)
		}
	}
}
