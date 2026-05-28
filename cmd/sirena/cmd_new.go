package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"m31labs.dev/sirena"
)

// RunNew implements `sirena new system|view <name>`. Scaffolds a starter
// .sir file under services/ or views/ respectively. The template embeds
// CurrentSpecVersion so the resulting file passes the version check
// without authors having to look it up.
//
// The scaffolder refuses to overwrite existing files — losing
// hand-edited content to a stray invocation would be the worst kind of
// surprise. Callers who want to reset should delete the file first.
//
// Exit codes: 0 on success, 1 on I/O error or clobber attempt,
// 2 on misuse.
func RunNew(args []string, stdout, stderr io.Writer) int {
	if len(args) != 2 || (args[0] != "system" && args[0] != "view") {
		fmt.Fprintln(stderr, "usage: sirena new system|view <name>")
		return 2
	}
	kind, name := args[0], args[1]

	var path, content string
	if kind == "system" {
		path = filepath.Join("services", name+".sir")
		content = fmt.Sprintf("# sirena_version: %s\nservice %s {\n  label: \"%s\"\n}\n",
			sirena.CurrentSpecVersion, name, name)
	} else {
		path = filepath.Join("views", name+".view.sir")
		content = fmt.Sprintf("# sirena_version: %s\nview \"%s\" {\n  title: \"%s\"\n  preset: default\n}\n",
			sirena.CurrentSpecVersion, name, name)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if _, err := os.Stat(path); err == nil {
		fmt.Fprintf(stderr, "%s already exists\n", path)
		return 1
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, path)
	return 0
}
