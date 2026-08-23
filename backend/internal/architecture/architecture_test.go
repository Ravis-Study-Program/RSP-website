// Package architecture_test protects the backend's high-level dependency direction.
package architecture_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestFeaturePackagesDoNotDependOnAdapters(t *testing.T) {
	root := moduleRoot(t)
	features := []string{"accounts", "authz", "mockinterviews", "practice", "programme"}
	for _, feature := range features {
		dir := filepath.Join(root, "backend", "internal", feature)
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			checkImports(t, path, feature)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func checkImports(t *testing.T, path, feature string) {
	t.Helper()
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, path, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, imported := range file.Imports {
		importedPath, err := strconv.Unquote(imported.Path.Value)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(importedPath, "/backend/internal/httpapi") || strings.Contains(importedPath, "/backend/internal/postgres") {
			position := fileSet.Position(imported.Pos())
			t.Errorf("%s/%s imports an adapter at %s:%d", feature, filepath.Base(path), position.Filename, position.Line)
		}
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find go.mod")
		}
		dir = parent
	}
}
