package main

import (
	"bytes"
	"fmt"
	"os"
)

const directive = "//lint:file-ignore SA1029 oapi-codegen uses a generated string context key for bearer scopes\n"

func main() {
	if len(os.Args) != 2 {
		panic("usage: postprocess <generated-go-file>")
	}
	path := os.Args[1]
	contents, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	if bytes.Contains(contents, []byte(directive)) {
		return
	}
	needle := []byte("// Package generated")
	if !bytes.Contains(contents, needle) {
		panic(fmt.Sprintf("%s does not contain the generated package comment", path))
	}
	contents = bytes.Replace(contents, needle, append([]byte(directive+"\n"), needle...), 1)
	if err := os.WriteFile(path, contents, 0o644); err != nil {
		panic(err)
	}
}
