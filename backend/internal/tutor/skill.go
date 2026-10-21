package tutor

import (
	"campusclaw/internal/mcp"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

func ParseSkill(source string) (Skill, error) {
	var skill Skill
	if !utf8.ValidString(source) || utf8.RuneCountInString(source) > 8000 {
		return skill, errors.New("技能文件必须为 UTF-8 且不超过 8000 字")
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
	tools := []string{}
	seen := map[string]bool{}
	for _, ref := range strings.FieldsFunc(fields["工具"], func(r rune) bool { return strings.ContainsRune(",，、;；|", r) }) {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		if ref != "knowledge_search" {
			if _, _, ok := mcp.ParseRef(ref); !ok {
				return skill, errors.New("工具须为 knowledge_search 或小写的 工具名@服务器名")
			}
		}
		if !seen[ref] {
			tools = append(tools, ref)
			seen[ref] = true
		}
	}
	if len(tools) > 16 {
		return skill, errors.New("技能最多绑定 16 个工具")
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
