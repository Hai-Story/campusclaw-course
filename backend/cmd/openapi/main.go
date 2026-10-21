// OpenAPI 3 generation from handler comments; no runtime or external generator dependency.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type operation struct {
	Method      string           `json:"method"`
	Path        string           `json:"path"`
	ID          string           `json:"id"`
	Summary     string           `json:"summary"`
	Input       string           `json:"input"`
	Output      string           `json:"output"`
	Success     int              `json:"success"`
	Codes       []int            `json:"codes"`
	Public      bool             `json:"public"`
	Teacher     bool             `json:"teacher"`
	Consumes    string           `json:"consumes"`
	Produces    string           `json:"produces"`
	Query       []map[string]any `json:"query"`
	Description string           `json:"description"`
}
type annotated struct {
	op      operation
	handler string
}

func generate(root string) ([]byte, error) {
	raw, err := os.ReadFile(filepath.Join(root, "internal/openapi/schemas.json"))
	if err != nil {
		return nil, err
	}
	var schemas map[string]any
	if err = json.Unmarshal(raw, &schemas); err != nil {
		return nil, err
	}
	specs := map[string]annotated{}
	routes := map[string]string{}
	ids := map[string]bool{}
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == ".tools" || d.Name() == "node_modules" || d.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, e := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
		if e != nil {
			return e
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Doc == nil {
				continue
			}
			for _, line := range strings.Split(fn.Doc.Text(), "\n") {
				if !strings.HasPrefix(line, "@openapi ") {
					continue
				}
				var op operation
				if e = json.Unmarshal([]byte(strings.TrimPrefix(line, "@openapi ")), &op); e != nil {
					return fmt.Errorf("%s: %w", path, e)
				}
				key := strings.ToUpper(op.Method) + " " + op.Path
				if op.ID == "" || op.Summary == "" || op.Success == 0 || op.Path == "" {
					return fmt.Errorf("incomplete annotation %s", key)
				}
				if _, exists := specs[key]; exists || ids[op.ID] {
					return fmt.Errorf("duplicate operation %s", key)
				}
				ids[op.ID] = true
				specs[key] = annotated{op, fn.Name.Name}
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) < 2 {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || (selector.Sel.Name != "Handle" && selector.Sel.Name != "HandleFunc") {
				return true
			}
			literal, ok := call.Args[0].(*ast.BasicLit)
			if !ok {
				return true
			}
			key, e := strconv.Unquote(literal.Value)
			if e != nil {
				return true
			}
			parts := strings.SplitN(key, " ", 2)
			if len(parts) != 2 || (!strings.HasPrefix(parts[1], "/api/") && parts[1] != "/health") {
				return true
			}
			handler := ""
			ast.Inspect(call.Args[1], func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.SelectorExpr:
					if x.Sel.Name != "Require" {
						handler = x.Sel.Name
					}
				case *ast.Ident:
					if x.Name == "health" {
						handler = x.Name
					}
				}
				return true
			})
			if ident, ok := call.Args[1].(*ast.Ident); ok {
				handler = ident.Name
			}
			routes[key] = handler
			return true
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	for key, handler := range routes {
		spec, ok := specs[key]
		if !ok {
			return nil, fmt.Errorf("route has no annotation: %s", key)
		}
		if spec.handler != handler {
			return nil, fmt.Errorf("handler mismatch: %s (%s/%s)", key, spec.handler, handler)
		}
	}
	paths := map[string]any{}
	ref := func(name string) map[string]any { return map[string]any{"$ref": "#/components/schemas/" + name} }
	for key, item := range specs {
		if _, ok := routes[key]; !ok {
			return nil, fmt.Errorf("annotation has no route: %s", key)
		}
		op := item.op
		for _, name := range []string{op.Input, op.Output} {
			if name != "" {
				if _, ok := schemas[name]; !ok {
					return nil, fmt.Errorf("unknown schema %s", name)
				}
			}
		}
		desc := op.Description
		if op.Teacher {
			desc += "\n仅教师可写。角色、班级来自服务端用户，客户端声明不参与授权。"
		}
		if strings.Contains(op.Path, "{id}") {
			desc += "\n跨班、不可访问与不存在对象的 404 响应完全相同。"
		}
		entry := map[string]any{"operationId": op.ID, "summary": op.Summary, "description": strings.TrimSpace(desc)}
		if op.Public {
			entry["security"] = []any{}
		} else {
			entry["security"] = []any{map[string]any{"bearerAuth": []string{}}}
		}
		params := append([]map[string]any{}, op.Query...)
		if strings.Contains(op.Path, "{id}") {
			params = append(params, map[string]any{"name": "id", "in": "path", "required": true, "schema": map[string]any{"type": "integer", "minimum": 1}})
		}
		if len(params) > 0 {
			entry["parameters"] = params
		}
		if op.Input != "" {
			kind := op.Consumes
			if kind == "" {
				kind = "application/json"
			}
			entry["requestBody"] = map[string]any{"required": true, "content": map[string]any{kind: map[string]any{"schema": ref(op.Input)}}}
		}
		responses := map[string]any{}
		success := map[string]any{"description": "成功"}
		if op.Success == 204 {
			success["description"] = "成功，无响应体"
		} else {
			kind := op.Produces
			if kind == "" {
				kind = "application/json"
			}
			schema := any(ref(op.Output))
			if op.Output == "" {
				schema = map[string]any{"type": "string"}
			}
			content := map[string]any{"schema": schema}
			if kind == "text/event-stream" {
				content["example"] = "event: meta\ndata: {\"status\":\"ok\",\"skill\":null,\"citations\":[]}\n\nevent: meta\ndata: {\"status\":\"ok\",\"skill\":null,\"citations\":[],\"tool_calls\":[{\"ref\":\"formula_hint@campus-tools\",\"status\":\"ok\",\"text\":\"检查每一项。\",\"round\":1,\"call_id\":\"example-call\"}]}\n\nevent: delta\ndata: {\"text\":\"先检查每一项。\"}\n\nevent: done\ndata: {\"message_id\":1}\n\n"
			}
			success["content"] = map[string]any{kind: content}
		}
		responses[strconv.Itoa(op.Success)] = success
		descriptions := map[int]string{400: "输入、长度、格式或班内重名错误", 401: "缺少、无效或失效的 Bearer Token", 403: "学生不能执行教师管理操作", 404: "资源不存在或不可访问", 409: "索引处理中或该会话正在生成", 413: "请求过大", 500: "内部存储失败", 502: "模型引用无效", 503: "数据库、检索或生成服务暂时不可用"}
		for _, code := range op.Codes {
			responses[strconv.Itoa(code)] = map[string]any{"description": descriptions[code], "content": map[string]any{"application/json": map[string]any{"schema": ref("Error")}}}
		}
		entry["responses"] = responses
		methods, ok := paths[op.Path].(map[string]any)
		if !ok {
			methods = map[string]any{}
			paths[op.Path] = methods
		}
		methods[strings.ToLower(op.Method)] = entry
	}
	for _, match := range regexp.MustCompile(`"\$ref"\s*:\s*"#/components/schemas/([^"]+)"`).FindAllSubmatch(raw, -1) {
		if _, ok := schemas[string(match[1])]; !ok {
			return nil, fmt.Errorf("unresolved schema %s", match[1])
		}
	}
	doc := map[string]any{"openapi": "3.0.3", "info": map[string]any{"title": "CampusClaw · 第六课 API", "version": "0.6.0", "description": "登录获取 token 后在 Authorize 输入 Token。接口只接受 Bearer，不接受 Cookie。会话先推 meta；MCP 工具逐轮补发含 tool_calls 的 meta 并先于正文，无调用时省略该字段；随后增量 delta、落库后 done；流开始后失败发 error。旧 /api/ask 无依据时不生成，新会话无依据仍回答。"}, "servers": []any{map[string]any{"url": "/"}}, "paths": paths, "components": map[string]any{"schemas": schemas, "securitySchemes": map[string]any{"bearerAuth": map[string]any{"type": "http", "scheme": "bearer", "bearerFormat": "JWT"}}}}
	output, err := json.MarshalIndent(doc, "", "  ")
	return append(output, '\n'), err
}
func main() {
	root := flag.String("root", ".", "backend root")
	out := flag.String("out", "internal/openapi/openapi.json", "output")
	check := flag.Bool("check", false, "fail if output is stale")
	flag.Parse()
	data, err := generate(*root)
	if err == nil {
		if *check {
			old, e := os.ReadFile(*out)
			if e != nil || !bytes.Equal(old, data) {
				err = errors.New("OpenAPI output is stale; regenerate it")
			}
		} else {
			err = os.WriteFile(*out, data, 0644)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
