package tutor

import (
	"net/http/httptest"
	"strings"
	"testing"
)

const standardSource = "---\nname: standard-math\ndescription: 数学引导\nallowed-tools: shell\n---\n先检查步骤，再提示下一步。"

func TestStandardSkillRoundTripAndHostRouting(t *testing.T) {
	enabled := true
	routing := &SkillRouting{Trigger: "方程", Subject: "数学", Tools: []string{}, Enabled: &enabled}
	item, err := ParseSkillInput(standardSource, routing)
	if err != nil || item.Source != standardSource || len(item.Tools) != 0 || item.Trigger != "方程" || !strings.Contains(item.Description, "先检查") {
		t.Fatalf("%+v %v", item, err)
	}
	if RouteSkill("方程怎么做", []Skill{item}) == nil {
		t.Fatal("not routed")
	}
	if _, err = ParseSkillInput(standardSource, nil); err == nil {
		t.Fatal("missing routing accepted")
	}
	enabled = false
	item, err = ParseSkillInput(standardSource, routing)
	if err != nil || RouteSkill("方程", []Skill{item}) != nil {
		t.Fatal("disabled skill routed")
	}
	routing.Tools = []string{"shell"}
	if _, err = ParseSkillInput(standardSource, routing); err == nil {
		t.Fatal("unauthorized tool accepted")
	}
}
func TestStandardSkillValidation(t *testing.T) {
	for _, raw := range []string{
		"---\nname: x\ndescription: y\nno-close",
		"---\nname: x\nname: y\ndescription: z\n---\nbody",
		"---\nname: [x]\ndescription: y\n---\nbody",
		"---\nname: x\n---\nbody",
		"---\nname: x\ndescription: y\n---\n",
		"---\nname: " + strings.Repeat("a", 65) + "\ndescription: y\n---\nbody",
		standardSource + strings.Repeat("字", 8001),
	} {
		if _, err := ParseSkill(raw); err == nil {
			t.Fatalf("accepted %.100q", raw)
		}
	}
	raw := "---\nname: " + strings.Repeat("a", 64) + "\ndescription: y\n---\n" + strings.Repeat("字", 2100)
	if _, err := ParseSkill(raw); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseSkill(strings.ReplaceAll(standardSource, "\n", "\r\n")); err != nil {
		t.Fatal(err)
	}
}

func TestPreviewAuthorizationBeforeDatabase(t *testing.T) {
	// The shared authorization tests exercise write access; preview must also reject students.
	s := New(nil, nil)
	request := httptest.NewRequest("POST", "/api/skills/preview", strings.NewReader(`{"source":"x"}`))
	w := httptest.NewRecorder()
	s.PreviewSkill(w, request)
	if w.Code != 401 {
		t.Fatalf("preview without login: %d", w.Code)
	}
}
