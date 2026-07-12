package analytics

import (
	"encoding/json"
	"strings"
	"time"
)

// Narrative event names (AP-135). No free-text dialogue, photos, coords.
const (
	EvChapterComplete = "narrative_chapter_complete"
	EvSegmentSkip     = "narrative_segment_skip"
	EvSegmentRewind   = "narrative_segment_rewind"
	EvChoice          = "narrative_choice"
	EvStuck           = "narrative_stuck"
	EvOptionalFound   = "narrative_optional_found"
	EvTechInterrupt   = "narrative_tech_interrupt"
	EvSurveySample    = "narrative_survey_sample" // consented aggregate only
)

func init() {
	// extend allow-list
	for _, n := range []string{
		EvChapterComplete, EvSegmentSkip, EvSegmentRewind, EvChoice,
		EvStuck, EvOptionalFound, EvTechInterrupt, EvSurveySample,
	} {
		AllowedEvents[n] = struct{}{}
	}
}

// NarrativeHealth separates understanding / pacing / tech / intentional skip.
type NarrativeHealth struct {
	ChapterVersion   string         `json:"chapter_version"`
	Completes        int            `json:"completes"`
	Skips            int            `json:"skips"`
	Rewinds          int            `json:"rewinds"`
	Choices          map[string]int `json:"choices"`
	StuckReasons     map[string]int `json:"stuck_reasons"` // confuse|bore|pace|other (enum)
	TechInterrupts   int            `json:"tech_interrupts"`
	OptionalFindRate float64        `json:"optional_find_rate"`
	SkipIntentional  int            `json:"skip_intentional"`
	SkipConfused     int            `json:"skip_confused"`
	SurveyN          int            `json:"survey_n"`
	MeaningfulChoice float64        `json:"survey_meaningful_choice"` // 0-1 aggregate
	FeltLectured     float64        `json:"survey_felt_lectured"`
	// Explicitly NOT optimizing FOMO/time-on-screen as sole goals.
	HealthNotes []string  `json:"health_notes"`
	ComputedAt  time.Time `json:"computed_at"`
}

// SummarizeNarrative recomputes chapter-level health without personal paths.
func (s *Store) SummarizeNarrative(chapterVersion string, asOf time.Time) NarrativeHealth {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if asOf.IsZero() {
		asOf = s.now()
	}
	h := NarrativeHealth{
		ChapterVersion: chapterVersion,
		Choices:        map[string]int{},
		StuckReasons:   map[string]int{},
		ComputedAt:     asOf,
	}
	optionalTotal, optionalFound := 0, 0
	var meaningfulSum, lecturedSum float64
	for _, e := range s.byID {
		if e.Deleted {
			continue
		}
		// only events at or before asOf (point-in-time health)
		if e.TS.After(asOf) {
			continue
		}
		props := map[string]any{}
		_ = json.Unmarshal([]byte(e.PropsJSON), &props)
		ver, _ := props["chapter_version"].(string)
		if chapterVersion != "" && ver != "" && ver != chapterVersion {
			continue
		}
		if chapterVersion != "" && ver == "" && e.Name != EvSurveySample {
			// only count versioned narrative events when filtering
			if strings.HasPrefix(e.Name, "narrative_") {
				continue
			}
		}
		switch e.Name {
		case EvChapterComplete:
			h.Completes++
		case EvSegmentSkip:
			h.Skips++
			reason, _ := props["reason"].(string)
			switch reason {
			case "intentional":
				h.SkipIntentional++
			case "confused":
				h.SkipConfused++
			}
		case EvSegmentRewind:
			h.Rewinds++
		case EvChoice:
			cid, _ := props["choice_id"].(string)
			if cid != "" {
				// aggregate only — not ordered path
				h.Choices[cid]++
			}
		case EvStuck:
			r, _ := props["reason"].(string)
			r = normalizeStuck(r)
			h.StuckReasons[r]++
		case EvTechInterrupt:
			h.TechInterrupts++
		case EvOptionalFound:
			optionalFound++
			optionalTotal++
		case EvSurveySample:
			// consented aggregate fields only
			h.SurveyN++
			if v, ok := props["meaningful_choice"].(float64); ok {
				meaningfulSum += v
			}
			if v, ok := props["felt_lectured"].(float64); ok {
				lecturedSum += v
			}
		}
		if e.Name == EvOptionalFound || (e.Name == EvChapterComplete && propBool(props, "had_optional")) {
			if e.Name == EvChapterComplete {
				optionalTotal++
				if propBool(props, "found_optional") {
					optionalFound++
				}
			}
		}
	}
	if optionalTotal > 0 {
		h.OptionalFindRate = float64(optionalFound) / float64(optionalTotal)
	}
	if h.SurveyN > 0 {
		h.MeaningfulChoice = meaningfulSum / float64(h.SurveyN)
		h.FeltLectured = lecturedSum / float64(h.SurveyN)
	}
	h.HealthNotes = buildNotes(h)
	return h
}

func normalizeStuck(r string) string {
	switch strings.ToLower(strings.TrimSpace(r)) {
	case "confuse", "confused", "understanding":
		return "understanding"
	case "bore", "bored", "pacing", "pace":
		return "pacing"
	case "tech", "error", "crash":
		return "tech"
	default:
		if r == "" {
			return "other"
		}
		return "other"
	}
}

func propBool(m map[string]any, k string) bool {
	v, ok := m[k]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

func buildNotes(h NarrativeHealth) []string {
	var notes []string
	if h.SkipConfused > h.SkipIntentional && h.Skips > 0 {
		notes = append(notes, "skip_mix_suggests_understanding_issues")
	}
	if h.StuckReasons["pacing"] > h.StuckReasons["understanding"] && h.StuckReasons["pacing"] > 0 {
		notes = append(notes, "pacing_friction_dominant")
	}
	if h.TechInterrupts > 0 && h.TechInterrupts >= h.Skips/2 {
		notes = append(notes, "investigate_tech_interrupts")
	}
	if h.SurveyN >= 3 && h.FeltLectured > 0.5 {
		notes = append(notes, "tone_review_felt_lectured")
	}
	if h.SurveyN >= 3 && h.MeaningfulChoice < 0.4 {
		notes = append(notes, "choices_feel_low_agency")
	}
	if len(notes) == 0 {
		notes = append(notes, "no_critical_narrative_health_flags")
	}
	return notes
}
