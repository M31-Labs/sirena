package fence

import (
	"bytes"
	"testing"
)

func TestStoryboard(t *testing.T) {
	frames, err := Storyboard([][]byte{[]byte(`service a { value: 1 }`), []byte(`service a { value: 4 }`)}, Options{Diagram: "bar"})
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 2 || bytes.Equal(frames[0], frames[1]) || !bytes.Contains(frames[0], []byte(`data-morph-id="a"`)) {
		t.Fatal("missing authored states or identity")
	}
	if _, err := Storyboard([][]byte{[]byte(`service a`), []byte(`not valid!!!`)}, Options{}); err == nil {
		t.Fatal("invalid state accepted")
	}
}
