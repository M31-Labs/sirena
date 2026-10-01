package mermaid

import (
	"fmt"
	"regexp"
	"strings"

	"m31labs.dev/sirena"
)

const familyIdent = `[A-Za-z_][A-Za-z0-9_]*`

var familyName = regexp.MustCompile(`^` + familyIdent + `$`)
var participantLine = regexp.MustCompile(`^(?:participant|actor)\s+(` + familyIdent + `)(?:\s+as\s+(.+))?$`)
var messageLine = regexp.MustCompile(`^(` + familyIdent + `)\s*(--?>>|--?>|--?x|--?\))\s*(` + familyIdent + `)\s*:\s*(.*)$`)
var stateAlias = regexp.MustCompile(`^state\s+"([^"]+)"\s+as\s+(` + familyIdent + `)$`)
var stateLine = regexp.MustCompile(`^(` + familyIdent + `|\[\*\])\s*-->\s*(` + familyIdent + `|\[\*\])(?:\s*:\s*(.*))?$`)
var classLine = regexp.MustCompile(`^class\s+(` + familyIdent + `)(?:\s*\{\s*)?$`)
var classEdge = regexp.MustCompile(`^(` + familyIdent + `)(?:\s+"([^"]*)")?\s*(<\|--|--\|>|\*--|--\*|o--|--o|\.\.>|<\.\.|-->|<--|--)\s*(?:"([^"]*)"\s+)?(` + familyIdent + `)(?:\s*:\s*(.*))?$`)

var classFieldMember = regexp.MustCompile(`^[+#~-]?(?:` + familyIdent + `|` + familyIdent + `\s*:\s*[A-Za-z_][A-Za-z0-9_~\[\]?,. ]*|[A-Za-z_][A-Za-z0-9_~\[\]?,.]*\s+` + familyIdent + `)$`)
var classMethodMember = regexp.MustCompile(`^[+#~-]?` + familyIdent + `\([^(){};]*\)(?:\s*:?\s*[A-Za-z_][A-Za-z0-9_~\[\]?,. ]*)?[$*]?$`)

func supportedClassMember(line string) bool {
	if len(line) > 2048 {
		return false
	}
	words := strings.Fields(line)
	if len(words) > 1 {
		switch words[0] {
		case "class", "style", "classDef", "cssClass", "click", "note", "direction":
			return false
		}
	}
	return classFieldMember.MatchString(line) || classMethodMember.MatchString(line)
}

// These deliberately bounded line grammars preserve supported semantic data.
// Control blocks, notes and styling are diagnosed instead of silently flattened.
func parseNativeFamilies(src []byte) (*sirena.Document, []sirena.Diagnostic, error, bool) {
	if len(src) > 4<<20 {
		return nil, nil, fmt.Errorf("Mermaid source exceeds 4 MiB"), true
	}
	lines := strings.Split(string(src), "\n")
	header := -1
	kind := ""
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "%%") {
			continue
		}
		header = i
		switch line {
		case "sequenceDiagram":
			kind = "sequence"
		case "stateDiagram", "stateDiagram-v2":
			kind = "state"
		case "classDiagram":
			kind = "class"
		case "mindmap":
			kind = "mindmap"
		}
		break
	}
	if kind == "" {
		return nil, nil, nil, false
	}

	sys := &sirena.SystemDecl{}
	doc := &sirena.Document{Diagram: kind, Systems: []*sirena.SystemDecl{sys}}
	elements := map[string]*sirena.Element{}
	add := func(name, label string) *sirena.Element {
		e := elements[name]
		if e == nil {
			e = &sirena.Element{Name: name, Kind: sirena.ElementKindService, Metadata: map[string]sirena.Value{}}
			elements[name] = e
			sys.Elements = append(sys.Elements, e)
		}
		if label != "" {
			e.Metadata["label"] = sirena.String{Value: label}
		}
		return e
	}
	edge := func(from, to, label string) {
		add(from, "")
		add(to, "")
		sys.Edges = append(sys.Edges, &sirena.Edge{From: from, To: to, FromRef: &sirena.Ref{Name: from}, ToRef: &sirena.Ref{Name: to}, Kind: sirena.EdgeKindFlow, Direction: sirena.DirForward, Label: label})
	}
	type branch struct {
		indent int
		name   string
	}
	var parents []branch
	class := ""
	var fields, methods []string
	offset := 0
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		start := offset
		offset += len(raw) + 1
		if i <= header || line == "" || strings.HasPrefix(line, "%%") {
			continue
		}
		accepted := false
		switch kind {
		case "sequence":
			if m := participantLine.FindStringSubmatch(line); m != nil {
				add(m[1], m[2])
				accepted = true
			} else if m := messageLine.FindStringSubmatch(line); m != nil {
				edge(m[1], m[3], m[4])
				sys.Edges[len(sys.Edges)-1].Metadata = map[string]sirena.Value{"arrow": sirena.String{Value: m[2]}}
				accepted = true
			}
		case "state":
			if m := stateAlias.FindStringSubmatch(line); m != nil {
				add(m[2], m[1])
				accepted = true
			} else if m := stateLine.FindStringSubmatch(line); m != nil {
				from, to := m[1], m[2]
				if from == "[*]" {
					from = "__sirena_state:initial"
					add(from, "").Metadata["state"] = sirena.String{Value: "initial"}
				}
				if to == "[*]" {
					to = "__sirena_state:final"
					add(to, "").Metadata["state"] = sirena.String{Value: "final"}
				}
				edge(from, to, m[3])
				accepted = true
			} else if name, label, ok := strings.Cut(line, ":"); ok && familyName.MatchString(strings.TrimSpace(name)) {
				add(strings.TrimSpace(name), strings.TrimSpace(label))
				accepted = true
			}
		case "class":
			if class != "" {
				if line == "}" {
					e := add(class, "")
					e.Metadata["fields"] = sirena.String{Value: strings.Join(fields, "; ")}
					e.Metadata["methods"] = sirena.String{Value: strings.Join(methods, "; ")}
					class = ""
					fields = nil
					methods = nil
				} else if strings.ContainsAny(line, "{}") || !supportedClassMember(line) || len(fields)+len(methods) >= 256 {
					break
				} else if strings.Contains(line, "(") {
					methods = append(methods, line)
				} else {
					fields = append(fields, line)
				}
				accepted = true
			} else if m := classLine.FindStringSubmatch(line); m != nil {
				add(m[1], "")
				if strings.HasSuffix(line, "{") {
					class = m[1]
				}
				accepted = true
			} else if m := classEdge.FindStringSubmatch(line); m != nil {
				from, to := m[1], m[5]
				if strings.HasPrefix(m[3], "<") {
					from, to = to, from
				}
				label := strings.TrimSpace(strings.Join([]string{m[2], m[6], m[4]}, " "))
				edge(from, to, label)
				e := sys.Edges[len(sys.Edges)-1]
				e.Metadata = map[string]sirena.Value{"relation": sirena.String{Value: m[3]}}
				accepted = true
			}
		case "mindmap":
			indent := len(raw) - len(strings.TrimLeft(raw, " \t"))
			name := fmt.Sprintf("__sirena_mindmap:%d", i)
			label := line
			if cut := strings.IndexAny(line, "(["); cut > 0 {
				candidate := strings.TrimSpace(line[:cut])
				if familyName.MatchString(candidate) {
					name = candidate
					label = strings.Trim(strings.TrimSpace(line[cut:]), "()[]")
				}
			}
			if _, exists := elements[name]; exists {
				return nil, nil, fmt.Errorf("Mermaid mindmap duplicate identity %q on line %d", name, i+1), true
			}
			add(name, label)
			for len(parents) > 0 && parents[len(parents)-1].indent >= indent {
				parents = parents[:len(parents)-1]
			}
			if len(parents) > 0 {
				edge(parents[len(parents)-1].name, name, "")
			}
			parents = append(parents, branch{indent, name})
			accepted = true
		}
		if !accepted {
			diag := sirena.Diagnostic{Code: "SIR-MERMAID-UNSUPPORTED", Severity: sirena.SeverityError, Message: fmt.Sprintf("unsupported %s statement on line %d: %s", kind, i+1, line), Range: sirena.Range{Start: start, End: start + len(raw)}}
			return nil, []sirena.Diagnostic{diag}, fmt.Errorf("%s", diag.Message), true
		}
		if len(sys.Elements) > 1000 || len(sys.Edges) > 2000 {
			return nil, nil, fmt.Errorf("Mermaid diagram exceeds 1000 participants or 2000 relationships"), true
		}
	}
	if class != "" || len(sys.Elements) == 0 {
		return nil, nil, fmt.Errorf("Mermaid %s is empty or has an unclosed class block", kind), true
	}
	return doc, nil, nil, true
}
