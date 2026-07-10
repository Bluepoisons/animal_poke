// Package safety implements content moderation for the vision pipeline.
//
// Public decision codes and flags are stable client contracts.
// Internal classifier labels / model details never leave this package
// in client-facing responses or retained image payloads.
package safety

import (
	"strings"
)

// Public decision codes (no model internals).
const (
	CodeOK               = "ok"
	CodeRejectPortrait   = "reject_portrait"    // pure human portrait — not collectable
	CodeRejectChildFocus = "reject_child_focus" // child-focused content
	CodeRejectSensitive  = "reject_sensitive"   // plate/house without animal
	CodeRejectUnsafe     = "reject_unsafe"      // abuse / prohibited content
	CodeFlagSensitive    = "flag_sensitive"     // face/plate/house with animal
	CodeFlagAbuse        = "flag_abuse"         // abuse — report path
	CodeFlagInjured      = "flag_injured"       // injured animal — care/report path
)

// Public action verbs.
const (
	ActionAllow  = "allow"
	ActionReject = "reject"
	ActionFlag   = "flag"
)

// Public flags exposed to clients (coarse, non-identifying).
const (
	FlagFace    = "face"
	FlagChild   = "child"
	FlagPlate   = "plate"
	FlagHouse   = "house"
	FlagAbuse   = "abuse"
	FlagInjured = "injured"
)

// Report path categories for abuse / injured animals.
const (
	ReportPathAbuse   = "abuse"
	ReportPathInjured = "injured"
)

// Known fixture labels used by tests and (dev/mock) form field safety_fixture.
const (
	FixturePerson       = "person"
	FixtureChild        = "child"
	FixturePersonAnimal = "person_animal"
	FixturePlate        = "plate"
	FixtureHouse        = "house"
	FixtureAbuse        = "abuse"
	FixtureInjured      = "injured"
	FixtureSafeAnimal   = "safe_animal"
)

// Input to the moderation evaluator.
// FixtureLabel drives deterministic fixture tests; Labels are free-text signals
// (VLM species/label) used by stub classifiers. Image bytes are never stored.
type Input struct {
	FixtureLabel string
	Filename     string
	Labels       []string
	// HasCapturableAnimal is true when taxonomy-capturable animals are present.
	HasCapturableAnimal bool
}

// Result is the public moderation decision.
type Result struct {
	Allowed      bool     `json:"allowed"`
	Collectable  bool     `json:"collectable"`
	DecisionCode string   `json:"decision_code"`
	Action       string   `json:"action"`
	Flags        []string `json:"flags,omitempty"`
	ReportPath   string   `json:"report_path,omitempty"`
	// InternalNotes stay server-side only (json:"-").
	InternalNotes []string `json:"-"`
}

// ClientView is the JSON-safe subset attached to vision responses.
type ClientView struct {
	Allowed      bool     `json:"allowed"`
	Collectable  bool     `json:"collectable"`
	DecisionCode string   `json:"decision_code"`
	Action       string   `json:"action"`
	Flags        []string `json:"flags,omitempty"`
	ReportPath   string   `json:"report_path,omitempty"`
}

// ToClientView strips internal fields.
func (r Result) ToClientView() ClientView {
	return ClientView{
		Allowed:      r.Allowed,
		Collectable:  r.Collectable,
		DecisionCode: r.DecisionCode,
		Action:       r.Action,
		Flags:        append([]string(nil), r.Flags...),
		ReportPath:   r.ReportPath,
	}
}

// Evaluate runs stub classifiers + fixture labels and returns a stable decision.
func Evaluate(in Input) Result {
	signals := classify(in)
	return decide(signals, in.HasCapturableAnimal)
}

type signals struct {
	face     bool
	child    bool
	plate    bool
	house    bool
	abuse    bool
	injured  bool
	person   bool // pure person subject
	animal   bool // animal subject indicated by fixture/labels
	fixture  string
	internal []string
}

func decide(s signals, hasCapturable bool) Result {
	hasAnimal := hasCapturable || s.animal

	// Abuse always takes the report path and is never collectable as a "game" capture.
	if s.abuse {
		return Result{
			Allowed:       false,
			Collectable:   false,
			DecisionCode:  CodeFlagAbuse,
			Action:        ActionFlag,
			Flags:         uniqueFlags(FlagAbuse),
			ReportPath:    ReportPathAbuse,
			InternalNotes: s.internal,
		}
	}

	// Child-focused content: hard reject as collectable.
	if s.child && !hasAnimal {
		return Result{
			Allowed:       false,
			Collectable:   false,
			DecisionCode:  CodeRejectChildFocus,
			Action:        ActionReject,
			Flags:         uniqueFlags(FlagFace, FlagChild),
			InternalNotes: s.internal,
		}
	}

	// Pure human portrait: never an animal collectable.
	if (s.person || s.face) && !hasAnimal && !s.plate && !s.house && !s.injured {
		code := CodeRejectPortrait
		flags := uniqueFlags(FlagFace)
		if s.child {
			code = CodeRejectChildFocus
			flags = uniqueFlags(FlagFace, FlagChild)
		}
		return Result{
			Allowed:       false,
			Collectable:   false,
			DecisionCode:  code,
			Action:        ActionReject,
			Flags:         flags,
			InternalNotes: s.internal,
		}
	}

	// Plate / house without animal: reject as collectable, flag sensitive.
	if (s.plate || s.house) && !hasAnimal {
		flags := []string{}
		if s.plate {
			flags = append(flags, FlagPlate)
		}
		if s.house {
			flags = append(flags, FlagHouse)
		}
		return Result{
			Allowed:       false,
			Collectable:   false,
			DecisionCode:  CodeRejectSensitive,
			Action:        ActionReject,
			Flags:         uniqueFlags(flags...),
			InternalNotes: s.internal,
		}
	}

	// Injured animal: allow collectable but flag + report path.
	if s.injured && hasAnimal {
		flags := []string{FlagInjured}
		if s.face {
			flags = append(flags, FlagFace)
		}
		return Result{
			Allowed:       true,
			Collectable:   true,
			DecisionCode:  CodeFlagInjured,
			Action:        ActionFlag,
			Flags:         uniqueFlags(flags...),
			ReportPath:    ReportPathInjured,
			InternalNotes: s.internal,
		}
	}
	if s.injured && !hasAnimal {
		return Result{
			Allowed:       false,
			Collectable:   false,
			DecisionCode:  CodeFlagInjured,
			Action:        ActionFlag,
			Flags:         uniqueFlags(FlagInjured),
			ReportPath:    ReportPathInjured,
			InternalNotes: s.internal,
		}
	}

	// Person + animal, or sensitive context with animal: allow with flags.
	if hasAnimal && (s.face || s.child || s.plate || s.house || s.person) {
		flags := []string{}
		if s.face || s.person {
			flags = append(flags, FlagFace)
		}
		if s.child {
			flags = append(flags, FlagChild)
		}
		if s.plate {
			flags = append(flags, FlagPlate)
		}
		if s.house {
			flags = append(flags, FlagHouse)
		}
		return Result{
			Allowed:       true,
			Collectable:   true,
			DecisionCode:  CodeFlagSensitive,
			Action:        ActionFlag,
			Flags:         uniqueFlags(flags...),
			InternalNotes: s.internal,
		}
	}

	// Default: safe animal / empty / unknown → ok.
	return Result{
		Allowed:       true,
		Collectable:   hasAnimal || (!s.face && !s.person && !s.child && !s.plate && !s.house && !s.abuse),
		DecisionCode:  CodeOK,
		Action:        ActionAllow,
		Flags:         nil,
		InternalNotes: s.internal,
	}
}

func uniqueFlags(in ...string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, f := range in {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		if _, ok := seen[f]; ok {
			continue
		}
		seen[f] = struct{}{}
		out = append(out, f)
	}
	return out
}

// NormalizeFixture canonicalizes fixture label strings.
func NormalizeFixture(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, " ", "_")
	switch s {
	case "person", "human", "portrait", "face_only":
		return FixturePerson
	case "child", "kid", "minor_face", "children":
		return FixtureChild
	case "person_animal", "human_animal", "person+animal", "person_and_animal":
		return FixturePersonAnimal
	case "plate", "license_plate", "car_plate", "车牌":
		return FixturePlate
	case "house", "home", "residence", "住宅", "address":
		return FixtureHouse
	case "abuse", "animal_abuse", "cruelty", "不当":
		return FixtureAbuse
	case "injured", "hurt", "wounded", "受伤":
		return FixtureInjured
	case "safe_animal", "animal", "cat", "dog", "goose", "safe":
		return FixtureSafeAnimal
	default:
		return s
	}
}
