package narrativeqa

import (
	"testing"

	"animalpoke/backend/internal/narrativecatalog"
)

func TestAnalyzeSeedPassesAndReportsEveryEndingPath(t *testing.T) {
	report := AnalyzeSeed()
	if !report.Valid() {
		t.Fatalf("seed catalog diagnostics: %#v", report.Diagnostics)
	}
	if len(report.Paths) == 0 || len(report.Endings) == 0 {
		t.Fatalf("expected reachable paths and endings, got paths=%d endings=%d", len(report.Paths), len(report.Endings))
	}
	for _, ending := range report.Endings {
		if ending == "ch4_exhibit_three" {
			return
		}
	}
	t.Fatal("expected ch4 exhibit ending in report")
}

func TestAnalyzeRejectsContinuityAndContentFailures(t *testing.T) {
	nodes := []narrativecatalog.NodeDef{
		{NodeID: "start", Body: "开始", Kind: "story"},
		{NodeID: "middle", Body: "中段", Kind: "story"},
		{NodeID: "ending", Body: "结局", Kind: "ending"},
		{NodeID: "loop", Body: "循环", Kind: "story"},
	}
	annotations := map[string]narrativecatalog.QAAnnotation{
		"start":  testAnnotation(1, false),
		"middle": {Locale: "en-US", Summary: "middle", EthicsLabels: []string{"fictional"}, AssetLicense: "internal-authored", Timeline: 2, KnowledgeRequires: []string{"introduced"}},
		"ending": testAnnotation(3, true),
		"loop":   testAnnotation(4, false),
	}
	choices := []narrativecatalog.ChoiceDef{
		{ChoiceID: "start-middle", FromNodeID: "start", ToNodeID: "middle", Label: "继续", Effects: map[string]any{"flag:stance": "open", "reward:ribbon": true}},
		{ChoiceID: "middle-ending", FromNodeID: "middle", ToNodeID: "ending", Label: "继续", Effects: map[string]any{"flag:stance": "closed", "reward:ribbon": true}},
		{ChoiceID: "ending-loop", FromNodeID: "ending", ToNodeID: "loop", Label: "继续", Effects: map[string]any{}},
		{ChoiceID: "loop-loop", FromNodeID: "loop", ToNodeID: "loop", Label: "继续", Effects: map[string]any{}},
		{ChoiceID: "missing-ref", FromNodeID: "start", ToNodeID: "missing", Label: "继续", Effects: map[string]any{}},
	}

	report := Analyze(nodes, choices, annotations, []string{"start"})
	for _, code := range []string{"locale_missing_or_invalid", "knowledge_before_discovery", "flag_conflict", "reward_duplicate", "terminal_has_outgoing_choice", "cycle", "choice_reference_missing"} {
		if !hasCode(report, code) {
			t.Errorf("expected diagnostic %q, got %#v", code, report.Diagnostics)
		}
	}
}

func testAnnotation(timeline int, terminal bool) narrativecatalog.QAAnnotation {
	return narrativecatalog.QAAnnotation{
		Locale:       "zh-CN",
		Summary:      "已人工核对的摘要。",
		EthicsLabels: []string{"fictional"},
		AssetLicense: "internal-authored",
		Timeline:     timeline,
		Terminal:     terminal,
	}
}

func hasCode(report Report, expected string) bool {
	for _, diagnostic := range report.Diagnostics {
		if diagnostic.Code == expected {
			return true
		}
	}
	return false
}
