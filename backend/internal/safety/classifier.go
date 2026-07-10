package safety

import (
	"path/filepath"
	"strings"
)

// classify merges fixture labels, filename heuristics, and free-text label stubs.
// This is intentionally simple (no real CV model) and deterministic for fixtures.
func classify(in Input) signals {
	s := signals{internal: nil}
	fix := NormalizeFixture(in.FixtureLabel)
	s.fixture = fix

	switch fix {
	case FixturePerson:
		s.person, s.face = true, true
		s.internal = append(s.internal, "fixture:person")
	case FixtureChild:
		s.person, s.face, s.child = true, true, true
		s.internal = append(s.internal, "fixture:child")
	case FixturePersonAnimal:
		s.person, s.face, s.animal = true, true, true
		s.internal = append(s.internal, "fixture:person_animal")
	case FixturePlate:
		s.plate = true
		s.internal = append(s.internal, "fixture:plate")
	case FixtureHouse:
		s.house = true
		s.internal = append(s.internal, "fixture:house")
	case FixtureAbuse:
		s.abuse = true
		s.internal = append(s.internal, "fixture:abuse")
	case FixtureInjured:
		s.injured, s.animal = true, true
		s.internal = append(s.internal, "fixture:injured")
	case FixtureSafeAnimal:
		s.animal = true
		s.internal = append(s.internal, "fixture:safe_animal")
	}

	// Filename stub heuristics (dev fixtures like person.jpg, plate_01.png).
	base := strings.ToLower(filepath.Base(in.Filename))
	base = strings.TrimSuffix(base, filepath.Ext(base))
	applyKeywordSignals(&s, base, "filename")

	// Free-text labels from VLM / taxonomy (never returned to client as-is).
	for _, lab := range in.Labels {
		applyKeywordSignals(&s, strings.ToLower(strings.TrimSpace(lab)), "label")
	}

	return s
}

func applyKeywordSignals(s *signals, text, source string) {
	if text == "" {
		return
	}
	// Abuse / cruelty first.
	for _, kw := range []string{"abuse", "cruelty", "torture", "虐待", "残害"} {
		if strings.Contains(text, kw) {
			s.abuse = true
			s.internal = append(s.internal, source+":abuse")
			break
		}
	}
	for _, kw := range []string{"injured", "wound", "hurt", "bleeding", "受伤", "伤口"} {
		if strings.Contains(text, kw) {
			s.injured = true
			s.internal = append(s.internal, source+":injured")
			break
		}
	}
	for _, kw := range []string{"child", "kid", "baby", "infant", "儿童", "小孩", "婴儿"} {
		if strings.Contains(text, kw) {
			s.child = true
			s.face = true
			s.person = true
			s.internal = append(s.internal, source+":child")
			break
		}
	}
	for _, kw := range []string{"person", "human", "people", "portrait", "face", "man", "woman", "人像", "人脸", "人类"} {
		if strings.Contains(text, kw) {
			s.person = true
			s.face = true
			s.internal = append(s.internal, source+":face")
			break
		}
	}
	for _, kw := range []string{"plate", "license", "number_plate", "车牌", "号牌"} {
		if strings.Contains(text, kw) {
			s.plate = true
			s.internal = append(s.internal, source+":plate")
			break
		}
	}
	for _, kw := range []string{"house", "home", "residence", "address", "doorplate", "住宅", "门牌", "住址"} {
		if strings.Contains(text, kw) {
			s.house = true
			s.internal = append(s.internal, source+":house")
			break
		}
	}
	for _, kw := range []string{"cat", "dog", "goose", "kitten", "puppy", "animal", "pet", "猫", "狗", "鹅"} {
		if strings.Contains(text, kw) {
			s.animal = true
			s.internal = append(s.internal, source+":animal")
			break
		}
	}
	// person_animal composite token
	if strings.Contains(text, "person_animal") || strings.Contains(text, "human_animal") {
		s.person, s.face, s.animal = true, true, true
		s.internal = append(s.internal, source+":person_animal")
	}
}
