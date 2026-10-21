package tutor

import (
	"campusclaw/internal/mcp"
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
	"strings"
	"unicode/utf8"
)

func ParseSkill(source string) (Skill, error) {
	var skill Skill
	if !utf8.ValidString(source) || utf8.RuneCountInString(source) > 8000 {
		return skill, errors.New("技能文件必须为 UTF-8 且不超过 8000 字")
	}
	if IsStandardSkill(source) {
		return parseMarkdownSkill(source)
	}
	lines := strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n")
	fields := map[string]string{}
	body := ""
	started := false
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if started {
				body = strings.TrimSpace(strings.Join(lines[i+1:], "\n"))
				break
			}
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(strings.ReplaceAll(line, "：", ":"), ":")
		key = strings.TrimSpace(key)
		if !ok {
			return skill, errors.New("元数据须为 字段：内容，说明前须有空行")
		}
		switch key {
		case "名称", "适用条件", "资料", "工具", "启用":
		default:
			return skill, fmt.Errorf("未知字段：%s", key)
		}
		if _, exists := fields[key]; exists {
			return skill, fmt.Errorf("重复字段：%s", key)
		}
		fields[key] = strings.TrimSpace(value)
		started = true
	}
	for _, key := range []string{"名称", "适用条件", "启用"} {
		if fields[key] == "" {
			return skill, fmt.Errorf("缺少必填字段：%s", key)
		}
	}
	if body == "" {
		return skill, errors.New("技能文件需要空行分隔和非空说明正文")
	}
	for _, item := range []struct {
		name, text string
		max        int
	}{{"名称", fields["名称"], 32}, {"适用条件", fields["适用条件"], 120}, {"资料", fields["资料"], 32}, {"说明", body, 2000}} {
		if utf8.RuneCountInString(item.text) > item.max {
			return skill, fmt.Errorf("%s不能超过 %d 字", item.name, item.max)
		}
	}
	if fields["启用"] != "是" && fields["启用"] != "否" {
		return skill, errors.New("启用只能为是或否")
	}
	tools, err := parseTools(fields["工具"])
	if err != nil {
		return skill, err
	}

	return Skill{Name: fields["名称"], Trigger: fields["适用条件"], Subject: fields["资料"], Tools: tools, Description: body, Source: source, Enabled: fields["启用"] == "是"}, nil
}

func RouteSkill(question string, skills []Skill) *Skill {
	question = strings.ToLower(question)
	best := 0
	var selected *Skill
	for i := range skills {
		item := &skills[i]
		if !item.Enabled {
			continue
		}
		score := 0
		for _, word := range strings.FieldsFunc(strings.ToLower(item.Trigger), func(r rune) bool { return strings.ContainsRune(",，、;；|", r) }) {
			word = strings.TrimSpace(word)
			if word != "" && strings.Contains(question, word) {
				score++
			}
		}
		if score > 0 && (score > best || (score == best && (selected == nil || item.ID < selected.ID))) {
			best = score
			selected = item
		}
	}
	return selected
}

func truncate(text string, n int) string {
	r := []rune(text)
	if len(r) > n {
		return string(r[:n])
	}
	return text
}

// SkillRouting is explicit host configuration; frontmatter never grants tools.
type SkillRouting struct {
	Trigger string   `json:"trigger"`
	Subject string   `json:"subject"`
	Tools   []string `json:"tools"`
	Enabled *bool    `json:"enabled"`
}

func IsStandardSkill(source string) bool {
	return strings.HasPrefix(strings.ReplaceAll(source, "\r\n", "\n"), "---\n")
}
func parseTools(raw string) ([]string, error) {
	tools := []string{}
	seen := map[string]bool{}
	for _, ref := range strings.FieldsFunc(raw, func(r rune) bool { return strings.ContainsRune(",，、;；|", r) }) {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		if ref != "knowledge_search" {
			if _, _, ok := mcp.ParseRef(ref); !ok {
				return nil, errors.New("工具须为 knowledge_search 或小写的 工具名@服务器名")
			}
		}
		if !seen[ref] {
			tools = append(tools, ref)
			seen[ref] = true
		}
	}
	if len(tools) > 16 {
		return nil, errors.New("技能最多绑定 16 个工具")
	}

	return tools, nil
}
func parseMarkdownSkill(source string) (Skill, error) {
	lines := strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n")
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return Skill{}, errors.New("SKILL.md 缺少 frontmatter 结束分隔线")
	}
	decoder := yaml.NewDecoder(strings.NewReader(strings.Join(lines[1:end], "\n")))
	var metadata map[string]yaml.Node
	if err := decoder.Decode(&metadata); err != nil {
		return Skill{}, errors.New("SKILL.md YAML 无效或有重复字段")
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return Skill{}, errors.New("SKILL.md 只允许一个 YAML 文档")
	}
	values := map[string]string{}
	for _, key := range []string{"name", "description"} {
		n, ok := metadata[key]
		if !ok || n.Kind != yaml.ScalarNode || n.Tag != "!!str" || strings.TrimSpace(n.Value) == "" {
			return Skill{}, fmt.Errorf("SKILL.md 缺少非空文本 %s", key)
		}
		values[key] = strings.TrimSpace(n.Value)
	}
	if utf8.RuneCountInString(values["name"]) > 64 {
		return Skill{}, errors.New("技能名称不能超过 64 字")
	}
	body := strings.TrimSpace(strings.Join(lines[end+1:], "\n"))
	if body == "" {
		return Skill{}, errors.New("SKILL.md 需要非空 Markdown 正文")
	}
	return Skill{Name: values["name"], Description: values["description"] + "\n\n" + body, Source: source, Tools: []string{}, Enabled: true}, nil
}
func ParseSkillInput(source string, routing *SkillRouting) (Skill, error) {
	item, err := ParseSkill(source)
	if err != nil {
		return item, err
	}
	if !IsStandardSkill(source) {
		if routing != nil {
			return item, errors.New("课程格式请在原文中配置触发条件和工具")
		}
		return item, nil
	}
	if routing == nil || routing.Enabled == nil || strings.TrimSpace(routing.Trigger) == "" || routing.Tools == nil {
		return item, errors.New("标准技能须显式提供 routing 的 trigger、subject、tools、enabled")
	}
	item.Trigger = strings.TrimSpace(routing.Trigger)
	item.Subject = strings.TrimSpace(routing.Subject)
	if utf8.RuneCountInString(item.Trigger) > 120 || utf8.RuneCountInString(item.Subject) > 32 {
		return item, errors.New("触发条件最多 120 字，资料范围最多 32 字")
	}
	item.Tools, err = parseTools(strings.Join(routing.Tools, ","))
	item.Enabled = *routing.Enabled
	return item, err
}
