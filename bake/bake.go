package bake

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"m31labs.dev/sirena"
	"m31labs.dev/sirena/ingest/mermaid"
	_ "m31labs.dev/sirena/layout" // registers the layout engine into sirena.Render
	"m31labs.dev/sirena/render/svg"
)

// Options configures a single Bake call.
type Options struct {
	// Infer promotes Mermaid shapes/labels to typed sirena kinds when set.
	Infer bool
	// Theme is the SVG theme name; empty selects the default theme.
	Theme string
}

// BlockError records a per-block render failure.
type BlockError struct {
	Index int // 1-based block index
	Lang  string
	Err   error
}

func (e BlockError) Error() string {
	return fmt.Sprintf("block %d (%s): %v", e.Index, e.Lang, e.Err)
}

// Result summarises a completed Bake call.
type Result struct {
	// BlocksBaked is the count of blocks that were successfully rendered and
	// rewritten.
	BlocksBaked int
	// SVGsWritten is the count of SVG files written (equals BlocksBaked on a
	// clean run; less when some blocks fail).
	SVGsWritten int
	// BlockErrors holds per-block render failures. Each failed block's source
	// is preserved unchanged in the output file.
	BlockErrors []BlockError
}

// Bake finds diagram fences in mdPath, renders each to SVG, and rewrites the
// markdown in place. Each SVG is written to <docbase>.<N>.svg next to mdPath
// where N is the 1-based diagram index.
//
// If no blocks are detected the file is left byte-identical (no write, no temp
// file churn). If any block fails to render the corresponding source is kept
// unchanged, the error is recorded in Result.BlockErrors, and Bake returns a
// non-nil error summarising the failures after processing all blocks. Other
// blocks still bake.
//
// The write is atomic: a temp file in the same directory is written and then
// renamed over mdPath.
func Bake(mdPath string, opts Options) (Result, error) {
	var res Result

	src, err := os.ReadFile(mdPath)
	if err != nil {
		return res, fmt.Errorf("bake: read %s: %w", mdPath, err)
	}

	blocks := Scan(src)
	if len(blocks) == 0 {
		// No diagram blocks — leave the file byte-identical.
		return res, nil
	}

	dir := filepath.Dir(mdPath)
	docbase := strings.TrimSuffix(filepath.Base(mdPath), filepath.Ext(mdPath))

	// Choose theme once.
	themeName := opts.Theme
	if themeName == "" {
		themeName = svg.DefaultThemeName
	}
	theme, err := svg.ThemeForName(themeName)
	if err != nil {
		return res, fmt.Errorf("bake: unknown theme %q: %w", themeName, err)
	}

	// Process blocks in document order, building the rewritten source via
	// forward-splicing to avoid offset shifts.
	var out strings.Builder
	prevEnd := 0

	for idx, blk := range blocks {
		n := idx + 1 // 1-based index

		// Append unchanged content between the previous block end and this block.
		out.Write(src[prevEnd:blk.Start])

		// Strip \r from body before rendering (CRLF-source note).
		body := strings.ReplaceAll(blk.Body, "\r", "")

		// Render the block to SVG.
		svgBytes, rerr := renderBlock(blk.Lang, body, opts.Infer, theme)
		if rerr != nil {
			// Render failure: keep the original source unchanged.
			out.Write(src[blk.Start:blk.End])
			res.BlockErrors = append(res.BlockErrors, BlockError{
				Index: n,
				Lang:  blk.Lang,
				Err:   rerr,
			})
			prevEnd = blk.End
			continue
		}

		// Write the SVG file.
		svgName := fmt.Sprintf("%s.%d.svg", docbase, n)
		svgPath := filepath.Join(dir, svgName)
		if werr := os.WriteFile(svgPath, svgBytes, 0o644); werr != nil {
			return res, fmt.Errorf("bake: write %s: %w", svgPath, werr)
		}
		res.SVGsWritten++

		// Append the rewritten baked block.
		out.WriteString(rewriteBlock(blk.Lang, body, svgName, n))
		res.BlocksBaked++
		prevEnd = blk.End
	}

	// Append any trailing content after the last block.
	out.Write(src[prevEnd:])

	// Atomic write: temp file in the same directory, then rename.
	tmp, terr := os.CreateTemp(dir, ".bake-tmp-*")
	if terr != nil {
		return res, fmt.Errorf("bake: create temp: %w", terr)
	}
	tmpPath := tmp.Name()
	_, werr := tmp.WriteString(out.String())
	cerr := tmp.Close()
	if werr != nil || cerr != nil {
		os.Remove(tmpPath)
		if werr != nil {
			return res, fmt.Errorf("bake: write temp: %w", werr)
		}
		return res, fmt.Errorf("bake: close temp: %w", cerr)
	}
	if rerr := os.Rename(tmpPath, mdPath); rerr != nil {
		os.Remove(tmpPath)
		return res, fmt.Errorf("bake: rename temp: %w", rerr)
	}

	// Return a non-nil error if any block failed, so callers can exit non-zero.
	if len(res.BlockErrors) > 0 {
		return res, joinBlockErrors(res.BlockErrors)
	}
	return res, nil
}

// renderBlock dispatches to the correct render path based on lang.
func renderBlock(lang, body string, infer bool, theme *svg.Theme) ([]byte, error) {
	switch lang {
	case "mermaid":
		return renderMermaidBody(body, infer, theme)
	case "sir", "sirena":
		return renderSirenaBody(body, theme)
	default:
		return nil, fmt.Errorf("unknown language %q", lang)
	}
}

// renderMermaidBody renders a Mermaid source body to SVG bytes.
func renderMermaidBody(body string, infer bool, theme *svg.Theme) ([]byte, error) {
	doc, _, err := mermaid.Parse([]byte(body), mermaid.Options{Infer: infer})
	if err != nil {
		return nil, fmt.Errorf("mermaid parse: %w", err)
	}
	if doc == nil {
		return nil, errors.New("SIR-MERMAID-NOT-A-GRAPH: input is not a supported Mermaid diagram type")
	}

	rv := sirena.AllElementsView(doc)
	lr, _, rerr := sirena.Render(rv, sirena.RenderOptions{})
	if rerr != nil {
		return nil, fmt.Errorf("sirena render: %w", rerr)
	}
	if lr == nil {
		return nil, errors.New("sirena render produced no layout")
	}

	out, serr := svg.Render(lr, theme)
	if serr != nil {
		return nil, fmt.Errorf("svg render: %w", serr)
	}
	return out, nil
}

// renderSirenaBody renders a sirena (sir/sirena) source body to SVG bytes via
// fence.Render semantics, but inline (no fence package import to avoid cycle;
// fence is already a wrapper around this exact sequence).
func renderSirenaBody(body string, theme *svg.Theme) ([]byte, error) {
	ws, err := sirena.NewFenceWorkspace([]byte(body), sirena.FenceOptions{})
	if err != nil {
		return nil, fmt.Errorf("sirena workspace: %w", err)
	}

	doc := ws.Files[0].Document
	if doc == nil {
		loaded, lerr := ws.LoadDocument(ws.Files[0])
		if lerr != nil {
			return nil, fmt.Errorf("sirena load document: %w", lerr)
		}
		doc = loaded
	}

	// Build the view: first declared view, or all-elements synthesis.
	var rv *sirena.ResolvedView
	if len(doc.Views) > 0 {
		rv, err = sirena.EvaluateView(ws, doc.Views[0])
		if err != nil {
			return nil, fmt.Errorf("sirena evaluate view: %w", err)
		}
	} else if docHasRenderableContent(doc) {
		rv = sirena.AllElementsView(doc)
	} else {
		return nil, errors.New("sirena fence has no view and no renderable elements")
	}

	lr, _, rerr := sirena.Render(rv, sirena.RenderOptions{})
	if rerr != nil {
		return nil, fmt.Errorf("sirena render: %w", rerr)
	}
	if lr == nil {
		return nil, errors.New("sirena render produced no layout")
	}

	out, serr := svg.Render(lr, theme)
	if serr != nil {
		return nil, fmt.Errorf("svg render: %w", serr)
	}
	return out, nil
}

// docHasRenderableContent reports whether the document has any elements,
// boundaries, or edges worth rendering.
func docHasRenderableContent(doc *sirena.Document) bool {
	for _, sys := range doc.Systems {
		if len(sys.Elements) > 0 || len(sys.Boundaries) > 0 || len(sys.Edges) > 0 {
			return true
		}
	}
	return false
}

// rewriteBlock builds the source-preserving baked block markup for one diagram.
//
// Format (exact, per spec):
//
//	<!-- sirena-baked:<lang> -->
//	![diagram <N>](<svgName>)
//	<details><summary>diagram source</summary>
//
//	```text
//	<body verbatim>
//	```
//
//	</details>
func rewriteBlock(lang, body, svgName string, n int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- sirena-baked:%s -->\n", lang)
	fmt.Fprintf(&b, "![diagram %d](%s)\n", n, svgName)
	b.WriteString("<details><summary>diagram source</summary>\n")
	b.WriteString("\n")
	b.WriteString("```text\n")
	b.WriteString(body)
	// Ensure body ends with a newline so the closing ``` is on its own line.
	if !strings.HasSuffix(body, "\n") {
		b.WriteString("\n")
	}
	b.WriteString("```\n")
	b.WriteString("\n")
	b.WriteString("</details>")
	// Note: no trailing newline here; the original fence's End already accounts
	// for the newline that followed the closing ``` — we rely on forward-splicing
	// to carry that forward. But because the spec format shows a trailing newline
	// after </details> and Scan includes it in the span, we add one.
	b.WriteString("\n")
	return b.String()
}

// joinBlockErrors formats multiple block errors into a single error summary.
func joinBlockErrors(errs []BlockError) error {
	if len(errs) == 1 {
		return fmt.Errorf("bake: 1 block failed: %w", errs[0].Err)
	}
	msgs := make([]string, len(errs))
	for i, e := range errs {
		msgs[i] = e.Error()
	}
	return fmt.Errorf("bake: %d blocks failed: %s", len(errs), strings.Join(msgs, "; "))
}
