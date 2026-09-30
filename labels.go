package sirena

// DisplayLabel returns the human-facing label for an element. A non-empty
// string-valued label metadata field overrides the stable declaration name.
func (e *Element) DisplayLabel() string {
	if e == nil {
		return ""
	}
	if value, ok := e.Metadata["label"]; ok {
		if label, ok := value.(String); ok && label.Value != "" {
			return label.Value
		}
	}
	return e.Name
}

// DisplayLabel returns the human-facing label for a boundary. A non-empty
// string-valued label metadata field overrides the stable declaration name.
func (b *Boundary) DisplayLabel() string {
	if b == nil {
		return ""
	}
	if value, ok := b.Metadata["label"]; ok {
		if label, ok := value.(String); ok && label.Value != "" {
			return label.Value
		}
	}
	return b.Name
}
