package knowledge

import (
	"strings"
	"testing"
)

func TestAutoChunkOffsetsAndOverlap(t *testing.T) {
	body := strings.Repeat("中文A", 330)
	chunks, err := Split(body, Strategy{})
	if err != nil || len(chunks) < 2 {
		t.Fatalf("expected multiple auto chunks, got %d, %v", len(chunks), err)
	}
	runes := []rune(body)
	for i, chunk := range chunks {
		if chunk.Text != string(runes[chunk.Start:chunk.End]) || chunk.End-chunk.Start > 800 {
			t.Fatalf("chunk %d has invalid original offsets", i)
		}
		if i > 0 && chunk.Start >= chunks[i-1].End {
			t.Fatalf("chunk %d did not overlap", i)
		}
	}
}

func TestCustomProcessedOffsetsLeaveBodyUntouched(t *testing.T) {
	body := "第一句。  https://example.com  " + strings.Repeat("后续文字。", 30)
	original := body
	chunks, err := Split(body, Strategy{Mode: "custom", MaxLength: 100, Separator: "period", RemoveURLs: true, CollapseWhitespace: true})
	if err != nil || len(chunks) == 0 {
		t.Fatalf("custom split failed: %v", err)
	}
	if body != original || chunks[0].OffsetBasis != "processed" || strings.Contains(chunks[0].Text, "https://") {
		t.Fatal("preprocessing changed original or failed to mark processed basis")
	}
}

func TestHierarchyRetainsHeadings(t *testing.T) {
	chunks, err := Split("# 第一章\n内容一\n## 第二节\n内容二", Strategy{Mode: "hierarchy"})
	if err != nil || len(chunks) != 2 {
		t.Fatalf("expected two sections, got %d, %v", len(chunks), err)
	}
	if !strings.HasPrefix(chunks[0].Text, "# 第一章") || !strings.HasPrefix(chunks[1].Text, "## 第二节") {
		t.Fatalf("heading missing from sections: %+v", chunks)
	}
}

func TestEmptyAndInvalidStrategy(t *testing.T) {
	chunks, err := Split("  \n ", Strategy{})
	if err != nil || len(chunks) != 0 {
		t.Fatalf("empty body should yield no chunks: %v %+v", err, chunks)
	}
	_, err = Split("text", Strategy{Mode: "custom", MaxLength: 99, Separator: "newline"})
	if err == nil {
		t.Fatal("accepted custom length below minimum")
	}
}
