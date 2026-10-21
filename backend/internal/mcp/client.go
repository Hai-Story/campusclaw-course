package mcp

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v5"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strings"
	"unicode/utf8"
)

func normalizeURL(raw string) (string, error) {
	u, e := url.Parse(strings.TrimSpace(raw))
	if e != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return "", errors.New("Base URL 须为获准的 HTTP(S) 地址，不含凭据、查询或片段")
	}
	u.Host = strings.ToLower(u.Host)
	u.Path = strings.TrimRight(u.Path, "/")
	if strings.Contains(u.Path, "..") || u.RawPath != "" {
		return "", errors.New("不允许的 URL 路径")
	}
	return u.String(), nil
}
func compileSchema(raw json.RawMessage) (*jsonschema.Schema, error) {
	var obj map[string]any
	if json.Unmarshal(raw, &obj) != nil || obj["type"] != "object" {
		return nil, errors.New("inputSchema 必须为对象结构")
	}
	c := jsonschema.NewCompiler()
	c.Draft = jsonschema.Draft2020
	c.LoadURL = func(string) (io.ReadCloser, error) { return nil, errors.New("不允许外部 Schema 引用") }
	if e := c.AddResource("https://schema.campus.invalid/input.json", bytes.NewReader(raw)); e != nil {
		return nil, e
	}
	return c.Compile("https://schema.campus.invalid/input.json")
}
func validateTool(t *Tool) error {
	if !ToolName.MatchString(t.Name) || strings.TrimSpace(t.Description) == "" || len([]rune(t.Description)) > 4000 {
		return errors.New("工具名称或说明无效")
	}
	if _, e := compileSchema(t.InputSchema); e != nil {
		return errors.New("工具 inputSchema 无效或含外部引用")
	}
	var schema any
	json.Unmarshal(t.InputSchema, &schema)
	canonical, _ := json.Marshal(map[string]any{"name": t.Name, "description": t.Description, "schema": schema})
	h := sha256.Sum256(canonical)
	t.Hash = hex.EncodeToString(h[:])
	return nil
}
func validateArgs(t Tool, raw string) (map[string]any, error) {
	if len(raw) > 16<<10 || !utf8.ValidString(raw) {
		return nil, ErrReference
	}
	var args map[string]any
	if json.Unmarshal([]byte(raw), &args) != nil || args == nil {
		return nil, ErrReference
	}
	schema, e := compileSchema(t.InputSchema)
	if e != nil || schema.Validate(args) != nil {
		return nil, ErrReference
	}
	return args, nil
}
func (s *Service) client(base string) *http.Client {
	dialer := &net.Dialer{Timeout: s.Config.CallTimeout}
	transport := &http.Transport{Proxy: nil, DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, e := net.SplitHostPort(addr)
		if e != nil {
			return nil, errors.New("无效目标")
		}
		ips, e := net.DefaultResolver.LookupIPAddr(ctx, host)
		if e != nil {
			return nil, errors.New("目标解析失败")
		}
		// Exact, deployment-owned URL allowlist permits explicit local test targets, but never metadata/link-local.
		for _, ip := range ips {
			if (ip.IP.IsLoopback() && net.ParseIP(host) == nil) || ip.IP.IsLinkLocalUnicast() || ip.IP.IsLinkLocalMulticast() || ip.IP.IsUnspecified() || ip.IP.IsMulticast() {
				return nil, errors.New("目标地址被拒绝")
			}
		}
		var last error
		for _, ip := range ips {
			conn, e := dialer.DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
			if e == nil {
				return conn, nil
			}
			last = e
		}
		return nil, last
	}, DisableKeepAlives: true}
	return &http.Client{Transport: transport, Timeout: s.Config.CallTimeout, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("MCP 不接受重定向") }}
}

type dispatchKey struct{}

func (s *Service) rpc(ctx context.Context, server Server, method string, params any, out any) error {
	if !s.Config.allowedURL(server.BaseURL) {
		return ErrReference
	}
	body := map[string]any{"jsonrpc": "2.0", "method": method, "params": params}
	if method != "notifications/initialized" {
		body["id"] = "campus"
	}
	raw, _ := json.Marshal(body)
	req, e := http.NewRequestWithContext(ctx, "POST", server.BaseURL+"/mcp", bytes.NewReader(raw))
	if e != nil {
		return errors.New("MCP 请求无效")
	}
	if notify, ok := ctx.Value(dispatchKey{}).(func()); ok {
		req = req.WithContext(httptrace.WithClientTrace(req.Context(), &httptrace.ClientTrace{WroteRequest: func(httptrace.WroteRequestInfo) { notify() }}))
	}
	req.Header.Set("Authorization", "Bearer "+server.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if method != "initialize" {
		req.Header.Set("MCP-Protocol-Version", ProtocolVersion)
	}
	res, e := s.client(server.BaseURL).Do(req)
	if e != nil {
		return errors.New("MCP 连接失败或超时")
	}
	defer res.Body.Close()
	if method == "notifications/initialized" {
		if res.StatusCode != 202 {
			return errors.New("MCP 初始化通知失败")
		}
		return nil
	}
	if res.StatusCode != 200 {
		return fmt.Errorf("MCP 返回 HTTP %d", res.StatusCode)
	}
	data, e := io.ReadAll(io.LimitReader(res.Body, (64<<10)+1))
	if e != nil || len(data) > 64<<10 || !utf8.Valid(data) {
		return errors.New("MCP 响应无效或过大")
	}
	if strings.HasPrefix(res.Header.Get("Content-Type"), "text/event-stream") {
		scanner := bufio.NewScanner(bytes.NewReader(data))
		var frames []string
		found := false
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				if len(frames) > 0 {
					data = []byte(strings.Join(frames, "\n"))
					found = true
					break
				}
			} else if strings.HasPrefix(line, "data:") {
				frames = append(frames, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			}
		}
		if !found {
			return errors.New("MCP SSE 缺少完整响应")
		}
	} else if !strings.HasPrefix(res.Header.Get("Content-Type"), "application/json") {
		return errors.New("MCP 响应类型无效")
	}
	var reply struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      string          `json:"id"`
		Result  json.RawMessage `json:"result"`
		Error   json.RawMessage `json:"error"`
	}
	if json.Unmarshal(data, &reply) != nil || reply.JSONRPC != "2.0" || reply.ID != "campus" || len(reply.Result) == 0 || string(reply.Result) == "null" || len(reply.Error) > 0 {
		return errors.New("MCP 协议错误或请求 ID 不匹配")
	}
	if out != nil && json.Unmarshal(reply.Result, out) != nil {
		return errors.New("MCP 结果结构无效")
	}
	return nil
}
func (s *Service) discover(ctx context.Context, server Server) ([]Tool, error) {
	var init struct {
		ProtocolVersion string                     `json:"protocolVersion"`
		Capabilities    map[string]json.RawMessage `json:"capabilities"`
	}
	if e := s.rpc(ctx, server, "initialize", map[string]any{"protocolVersion": ProtocolVersion, "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "campusclaw", "version": "1.0.0"}}, &init); e != nil {
		return nil, e
	}
	if init.ProtocolVersion != ProtocolVersion || len(init.Capabilities["tools"]) == 0 {
		return nil, errors.New("MCP 版本或工具能力不兼容")
	}
	if e := s.rpc(ctx, server, "notifications/initialized", map[string]any{}, nil); e != nil {
		return nil, e
	}
	tools := []Tool{}
	cursor := ""
	seen := map[string]bool{}
	names := map[string]bool{}
	for {
		var result struct {
			Tools      []Tool `json:"tools"`
			NextCursor string `json:"nextCursor"`
		}
		params := map[string]any{}
		if cursor != "" {
			params["cursor"] = cursor
		}
		if e := s.rpc(ctx, server, "tools/list", params, &result); e != nil {
			return nil, e
		}
		for _, t := range result.Tools {
			raw, _ := json.Marshal(t)
			if server.Token != "" && bytes.Contains(raw, []byte(server.Token)) {
				return nil, errors.New("工具声明含秘密，拒绝发现")
			}
			if names[t.Name] {
				return nil, errors.New("MCP 工具名称重复")
			}
			if e := validateTool(&t); e != nil {
				return nil, e
			}
			names[t.Name] = true
			tools = append(tools, t)
			if len(tools) > 64 {
				return nil, errors.New("MCP 工具过多")
			}
		}
		if result.NextCursor == "" {
			break
		}
		if seen[result.NextCursor] || len(seen) > 16 {
			return nil, errors.New("MCP 分页无效")
		}
		seen[result.NextCursor] = true
		cursor = result.NextCursor
	}
	return tools, nil
}
func (s *Service) remoteCall(ctx context.Context, c Candidate, args map[string]any) (string, error) {
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if e := s.rpc(ctx, c.Server, "tools/call", map[string]any{"name": c.Tool.Name, "arguments": args}, &result); e != nil {
		return "", e
	}
	if result.IsError {
		return "", errors.New("工具返回业务错误")
	}
	var text strings.Builder
	for _, part := range result.Content {
		if part.Type != "text" {
			return "", errors.New("工具返回非文本结果")
		}
		text.WriteString(part.Text)
		text.WriteByte('\n')
	}
	if strings.TrimSpace(text.String()) == "" {
		return "", errors.New("工具返回空结果")
	}
	value := strings.TrimSpace(text.String())
	if c.Server.Token != "" {
		value = strings.ReplaceAll(value, c.Server.Token, "[REDACTED]")
	}
	value = truncate(value, 2000)
	if hostile(value) {
		return "", errors.New("工具返回含不可信指令，已整条拒绝")
	}
	return value, nil
}
func hostile(text string) bool {
	v := strings.ToLower(text)
	for _, needle := range []string{"忽略规则", "忽略之前", "忽略所有", "泄露密钥", "发送密钥", "读取密钥", "ignore previous", "ignore all instructions", "system prompt", "api_key", "authorization: bearer"} {
		if strings.Contains(v, needle) {
			return true
		}
	}
	return false
}
