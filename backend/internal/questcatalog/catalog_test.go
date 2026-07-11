package questcatalog

import "testing"

func TestCatalog_Min24AndTrustedOnly(t *testing.T) {
	if err := ValidateGraph(); err != nil {
		t.Fatal(err)
	}
	if Count() < 24 {
		t.Fatalf("want >=24 quests, got %d", Count())
	}
	free := FreeExecutable()
	if len(free) == 0 {
		t.Fatal("expected free quests for zero stamina")
	}
	// 确保没有把打开页面类事件写进目录
	for _, d := range All() {
		for _, o := range d.Objectives {
			if !IsTrustedEvent(o.Event) {
				t.Fatalf("untrusted event %s in %s", o.Event, d.QuestID)
			}
			if _, bad := ForbiddenEvents[o.Event]; bad {
				t.Fatalf("forbidden event %s in %s", o.Event, d.QuestID)
			}
		}
	}
}

func TestForbiddenEventsRejected(t *testing.T) {
	for e := range ForbiddenEvents {
		if IsTrustedEvent(e) {
			t.Fatalf("%s should not be trusted", e)
		}
	}
	if !IsTrustedEvent("capture_success") {
		t.Fatal("capture_success must be trusted")
	}
}
