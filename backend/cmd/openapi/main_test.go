package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnnotationRouteConsistency(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "internal/openapi"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal/openapi/schemas.json"), []byte(`{"Error":{"type":"object"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	valid := "package example\nimport \"net/http\"\n// @openapi {\"method\":\"get\",\"path\":\"/api/test\",\"id\":\"test\",\"summary\":\"test\",\"success\":200}\nfunc Read(){}\nfunc main(){mux:=http.NewServeMux();mux.HandleFunc(\"GET /api/test\",Read)}\n"
	for _, c := range []struct {
		code  string
		valid bool
	}{{valid, true}, {strings.Replace(valid, "// @openapi", "// removed", 1), false}, {strings.Replace(valid, "/api/test\",Read", "/api/other\",Read", 1), false}, {strings.Replace(valid, "\"success\":200", "\"success\":200,\"input\":\"Missing\"", 1), false}, {valid + "// @openapi {\"method\":\"post\",\"path\":\"/api/test\",\"id\":\"test\",\"summary\":\"duplicate\",\"success\":200}\nfunc Duplicate(){}\n", false}} {
		if err := os.WriteFile(filepath.Join(root, "handlers.go"), []byte(c.code), 0644); err != nil {
			t.Fatal(err)
		}
		_, err := generate(root)
		if (err == nil) != c.valid {
			t.Fatalf("valid=%v err=%v", c.valid, err)
		}
	}
}
