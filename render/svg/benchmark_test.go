package svg_test

import (
	"fmt"
	"testing"

	"m31labs.dev/sirena"
	"m31labs.dev/sirena/layout"
	"m31labs.dev/sirena/render/svg"
)

// Reports allocations and encoded size for realistic repeated-label diagrams.
// Layout and SVG phases have separate baselines; the renderer never includes parsing.
func BenchmarkDiagramPipeline(b *testing.B) {
	for _, kind := range []string{"mindmap", "bar", "sequence", "class"} {
		for _, n := range []int{20, 100, 500} {
			b.Run(fmt.Sprintf("%s/%d", kind, n), func(b *testing.B) {
				rv := &sirena.ResolvedView{}
				for i := 0; i < n; i++ {
					rv.Elements = append(rv.Elements, &sirena.Element{Name: fmt.Sprintf("node%d", i), Metadata: map[string]sirena.Value{"label": sirena.String{Value: fmt.Sprintf("Service %d: request processing", i)}, "value": sirena.Number{Value: float64(i + 1)}}})
					if kind == "mindmap" && i > 0 {
						rv.Edges = append(rv.Edges, &sirena.Edge{From: "node0", To: fmt.Sprintf("node%d", i), Direction: sirena.DirForward})
					}
				}
				lr, _, err := layout.Render(rv, sirena.RenderOptions{Diagram: kind})
				if err != nil {
					b.Fatal(err)
				}
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					data, err := svg.Render(lr, nil)
					if err != nil {
						b.Fatal(err)
					}
					b.ReportMetric(float64(len(data)), "svg-bytes")
				}
			})
		}
	}
}
