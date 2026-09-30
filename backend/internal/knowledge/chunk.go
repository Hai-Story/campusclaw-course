package knowledge

import (
	"errors"
	"regexp"
	"strings"
	"unicode"
)

type Strategy struct {
	Mode               string `json:"mode"`
	MaxLength          int    `json:"max_length,omitempty"`
	OverlapPercent     int    `json:"overlap_percent,omitempty"`
	Separator          string `json:"separator,omitempty"`
	RemoveURLs         bool   `json:"remove_urls,omitempty"`
	RemoveEmails       bool   `json:"remove_emails,omitempty"`
	CollapseWhitespace bool   `json:"collapse_whitespace,omitempty"`
}

type Chunk struct {
	Index       int
	Text        string
	Start       int
	End         int
	OffsetBasis string
}

var (
	urlPattern   = regexp.MustCompile(`https?://[^\s<>]+`)
	emailPattern = regexp.MustCompile(`[[:alnum:]._%+-]+@[[:alnum:].-]+\.[[:alpha:]]{2,}`)
	headingLine  = regexp.MustCompile(`^#{1,3}[ \t]+`)
)

func ValidateStrategy(s Strategy) (Strategy, error) {
	if s.Mode == "" {
		s.Mode = "auto"
	}
	switch s.Mode {
	case "auto", "hierarchy":
		return Strategy{Mode: s.Mode}, nil
	case "custom":
		if s.MaxLength < 100 || s.MaxLength > 2000 {
			return Strategy{}, errors.New("custom max_length must be between 100 and 2000")
		}
		if s.OverlapPercent < 0 || s.OverlapPercent > 50 {
			return Strategy{}, errors.New("custom overlap_percent must be between 0 and 50")
		}
		if s.Separator != "newline" && s.Separator != "blank-line" && s.Separator != "period" {
			return Strategy{}, errors.New("custom separator must be newline, blank-line or period")
		}
		return s, nil
	default:
		return Strategy{}, errors.New("unknown chunk strategy")
	}
}

func Split(body string, strategy Strategy) ([]Chunk, error) {
	s, err := ValidateStrategy(strategy)
	if err != nil {
		return nil, err
	}
	basis := "original"
	text := body
	if s.Mode == "custom" {
		if s.RemoveURLs {
			text = urlPattern.ReplaceAllString(text, "")
		}
		if s.RemoveEmails {
			text = emailPattern.ReplaceAllString(text, "")
		}
		if s.CollapseWhitespace {
			text = strings.Join(strings.Fields(text), " ")
		}
		if text != body {
			basis = "processed"
		}
	}
	runes := []rune(text)
	if len(strings.TrimSpace(text)) == 0 {
		return []Chunk{}, nil
	}
	var chunks []Chunk
	switch s.Mode {
	case "auto":
		chunks = windowChunks(runes, 0, 800, 80, "auto", basis)
	case "custom":
		overlap := s.MaxLength * s.OverlapPercent / 100
		chunks = windowChunks(runes, 0, s.MaxLength, overlap, s.Separator, basis)
	case "hierarchy":
		chunks = hierarchyChunks(runes)
	}
	for i := range chunks {
		chunks[i].Index = i + 1
	}
	return chunks, nil
}

func hierarchyChunks(text []rune) []Chunk {
	starts := []int{0}
	lineStart := 0
	for i, r := range text {
		if i == lineStart && headingLine.MatchString(string(text[i:lineEnd(text, i)])) && i > 0 {
			starts = append(starts, i)
		}
		if r == '\n' {
			lineStart = i + 1
		}
	}
	starts = append(starts, len(text))
	var chunks []Chunk
	for i := 0; i < len(starts)-1; i++ {
		section := text[starts[i]:starts[i+1]]
		if len(strings.TrimSpace(string(section))) == 0 {
			continue
		}
		chunks = append(chunks, windowChunks(section, starts[i], 800, 80, "auto", "original")...)
	}
	return chunks
}

func lineEnd(text []rune, start int) int {
	for i := start; i < len(text); i++ {
		if text[i] == '\n' {
			return i
		}
	}
	return len(text)
}

func windowChunks(text []rune, base, max, overlap int, separator, basis string) []Chunk {
	var chunks []Chunk
	for start := 0; start < len(text); {
		end := start + max
		if end >= len(text) {
			end = len(text)
		} else {
			minBoundary := start + max/2
			if minBoundary < start+overlap+1 {
				minBoundary = start + overlap + 1
			}
			if boundary := preferredBoundary(text, minBoundary, end, separator); boundary > start {
				end = boundary
			}
		}
		part := text[start:end]
		if !allSpace(part) {
			chunks = append(chunks, Chunk{Text: string(part), Start: base + start, End: base + end, OffsetBasis: basis})
		}
		if end == len(text) {
			break
		}
		start = end - overlap
	}
	return chunks
}

func preferredBoundary(text []rune, min, end int, separator string) int {
	for i := end - 1; i >= min; i-- {
		switch separator {
		case "newline":
			if text[i] == '\n' {
				return i + 1
			}
		case "blank-line":
			if i > 0 && text[i] == '\n' && text[i-1] == '\n' {
				return i + 1
			}
		case "period":
			if text[i] == '。' || text[i] == '.' {
				return i + 1
			}
		default:
			if i > 0 && text[i] == '\n' && text[i-1] == '\n' {
				return i + 1
			}
		}
	}
	if separator == "auto" {
		for i := end - 1; i >= min; i-- {
			if text[i] == '\n' || text[i] == '。' || text[i] == '.' {
				return i + 1
			}
		}
	}
	return 0
}

func allSpace(text []rune) bool {
	for _, r := range text {
		if !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}
