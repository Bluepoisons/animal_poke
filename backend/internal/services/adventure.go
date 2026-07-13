package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"
	"unicode/utf8"

	"animalpoke/backend/internal/ai/prompts"
	"animalpoke/backend/internal/narrativepolicy"

	"github.com/google/uuid"
)

const adventureDisclaimer = "人工智能生成的全中文幻想冒险，仅用于虚构玩法，不是现实记录"

var adventureThemeNames = map[string]string{
	"mistwood":   "雾灯森径",
	"sky_ruins":  "浮空遗迹",
	"tide_isles": "潮汐群岛",
}

// AdventureInput is a whitelisted companion profile used to generate fiction.
type AdventureInput struct {
	AnimalID       string `json:"animal_id"`
	Nickname       string `json:"nickname"`
	Species        string `json:"species"`
	SpeciesLabelZH string `json:"species_label_zh"`
	Breed          string `json:"breed"`
	Class          string `json:"class"`
	Element        string `json:"element"`
	HP             int    `json:"hp"`
	ATK            int    `json:"atk"`
	DEF            int    `json:"def"`
	SPD            int    `json:"spd"`
	BondLevel      int    `json:"bond_level"`
	Theme          string `json:"theme"`
	ThemeName      string `json:"-"`
	SpeciesName    string `json:"-"`
	BreedName      string `json:"-"`
	ClassName      string `json:"-"`
	ElementName    string `json:"-"`
}

// Validate keeps user/model-derived profile fields bounded before prompt rendering.
func (in *AdventureInput) Validate() error {
	in.AnimalID = strings.TrimSpace(in.AnimalID)
	in.Nickname = strings.TrimSpace(in.Nickname)
	in.Species = strings.TrimSpace(in.Species)
	in.SpeciesLabelZH = strings.TrimSpace(in.SpeciesLabelZH)
	in.Breed = strings.TrimSpace(in.Breed)
	in.Class = strings.TrimSpace(in.Class)
	in.Element = strings.TrimSpace(in.Element)
	in.Theme = strings.TrimSpace(in.Theme)

	if in.AnimalID == "" || in.Species == "" {
		return fmt.Errorf("animal_id and species are required")
	}
	if utf8.RuneCountInString(in.AnimalID) > 128 || utf8.RuneCountInString(in.Nickname) > 64 || utf8.RuneCountInString(in.Species) > 32 || utf8.RuneCountInString(in.SpeciesLabelZH) > 64 || utf8.RuneCountInString(in.Breed) > 64 || utf8.RuneCountInString(in.Class) > 32 || utf8.RuneCountInString(in.Element) > 32 {
		return fmt.Errorf("adventure profile field too long")
	}
	themeName, ok := adventureThemeNames[in.Theme]
	if !ok {
		return fmt.Errorf("unsupported adventure theme")
	}
	in.ThemeName = themeName
	normalizedSpecies, normalizedLabel, identityErr := NormalizeAnimalIdentity(in.Species, in.SpeciesLabelZH)
	if identityErr != nil {
		return fmt.Errorf("invalid adventure animal identity: %w", identityErr)
	}
	in.Species = normalizedSpecies
	in.SpeciesLabelZH = normalizedLabel
	in.SpeciesName = normalizedLabel
	in.BreedName = chineseBreed(in.Breed, in.Species)
	in.ClassName = chineseClass(in.Class)
	in.ElementName = chineseElement(in.Element)
	for _, value := range []int{in.HP, in.ATK, in.DEF, in.SPD} {
		if value < 0 || value > 100 {
			return fmt.Errorf("adventure stats must be between 0 and 100")
		}
	}
	if in.BondLevel < 0 || in.BondLevel > 100 {
		return fmt.Errorf("bond_level must be between 0 and 100")
	}
	if err := narrativepolicy.ValidatePromptInput(in.Nickname, in.Species, in.SpeciesLabelZH, in.Breed, in.Class, in.Element); err != nil {
		return fmt.Errorf("unsafe adventure profile: %w", err)
	}
	return nil
}

// AdventureChoice is one relationship-positive way through an encounter.
type AdventureChoice struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Outcome     string `json:"outcome"`
	BondDelta   int    `json:"bond_delta"`
}

// AdventureSouvenir is the memory token awarded by an encounter.
type AdventureSouvenir struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// AdventureResult is a complete one-encounter branch generated in one AI call.
type AdventureResult struct {
	AdventureID    string            `json:"adventure_id"`
	Theme          string            `json:"theme"`
	Title          string            `json:"title"`
	Location       string            `json:"location"`
	Opening        string            `json:"opening"`
	EncounterTitle string            `json:"encounter_title"`
	Encounter      string            `json:"encounter"`
	CompanionLine  string            `json:"companion_line"`
	Choices        []AdventureChoice `json:"choices"`
	Souvenir       AdventureSouvenir `json:"souvenir"`
	Fiction        bool              `json:"fiction"`
	Disclaimer     string            `json:"disclaimer"`
	Source         string            `json:"source"`
	Degraded       bool              `json:"degraded,omitempty"`
	ReasonCode     string            `json:"reason_code,omitempty"`
	Model          string            `json:"model,omitempty"`
	PromptVersion  string            `json:"prompt_version"`
}

type generatedAdventure struct {
	Title          string            `json:"title"`
	Location       string            `json:"location"`
	Opening        string            `json:"opening"`
	EncounterTitle string            `json:"encounter_title"`
	Encounter      string            `json:"encounter"`
	CompanionLine  string            `json:"companion_line"`
	Choices        []AdventureChoice `json:"choices"`
	Souvenir       AdventureSouvenir `json:"souvenir"`
}

var adventurePromptTmpl = template.Must(template.New("adventure").Parse(prompts.AdventurePrompt))

func renderAdventurePrompt(input AdventureInput) (string, error) {
	var buf bytes.Buffer
	if err := adventurePromptTmpl.Execute(&buf, input); err != nil {
		return "", fmt.Errorf("render adventure prompt: %w", err)
	}
	return buf.String(), nil
}

// GenerateAdventureContext creates a safe structured branch from one pet profile.
func (s *AIService) GenerateAdventureContext(ctx context.Context, input AdventureInput) (*AdventureResult, error) {
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	fallback := adventureFallback(input)
	if !s.cfg.LLMConfigured() {
		if !s.mock {
			return nil, fmt.Errorf("llm provider not configured")
		}
		fallback.Source = "mock"
		fallback.Degraded = true
		fallback.ReasonCode = "provider_not_configured"
		return fallback, nil
	}

	prompt, err := renderAdventurePrompt(input)
	if err != nil {
		return nil, err
	}
	body, model, err := s.callText(ctx, prompt)
	if err != nil {
		if !s.mock {
			return nil, fmt.Errorf("adventure provider failed: %w", err)
		}
		fallback.Source = "template"
		fallback.Degraded = true
		fallback.ReasonCode = "adventure_fallback"
		fallback.Model = model
		return fallback, nil
	}
	jsonText, err := s.parseResponsesResponse(body)
	if err != nil {
		if !s.mock {
			return nil, fmt.Errorf("adventure response parse failed: %w", err)
		}
		fallback.Source = "template"
		fallback.Degraded = true
		fallback.ReasonCode = "adventure_parse_failed"
		fallback.Model = model
		return fallback, nil
	}

	var generated generatedAdventure
	validationErr := json.Unmarshal([]byte(jsonText), &generated)
	if validationErr == nil {
		validationErr = simplifyGeneratedAdventure(&generated)
	}
	if validationErr == nil {
		validationErr = validateGeneratedAdventure(generated, input.Nickname)
	}
	if validationErr != nil {
		if !s.mock {
			return nil, fmt.Errorf("adventure response failed validation")
		}
		fallback.Source = "template"
		fallback.Degraded = true
		fallback.ReasonCode = "adventure_invalid"
		fallback.Model = model
		return fallback, nil
	}

	applyAdventureMechanics(generated.Choices)
	result := &AdventureResult{
		AdventureID:    uuid.NewString(),
		Theme:          input.Theme,
		Title:          generated.Title,
		Location:       generated.Location,
		Opening:        generated.Opening,
		EncounterTitle: generated.EncounterTitle,
		Encounter:      generated.Encounter,
		CompanionLine:  generated.CompanionLine,
		Choices:        generated.Choices,
		Souvenir:       generated.Souvenir,
		Fiction:        true,
		Disclaimer:     adventureDisclaimer,
		Source:         "ai",
		Model:          model,
		PromptVersion:  prompts.AdventurePromptVersion,
	}
	return result, nil
}

func simplifyGeneratedAdventure(g *generatedAdventure) error {
	if g == nil {
		return fmt.Errorf("nil generated adventure")
	}
	fields := []*string{
		&g.Title,
		&g.Location,
		&g.Opening,
		&g.EncounterTitle,
		&g.Encounter,
		&g.CompanionLine,
		&g.Souvenir.Name,
		&g.Souvenir.Description,
	}
	for i := range g.Choices {
		fields = append(fields, &g.Choices[i].Label, &g.Choices[i].Description, &g.Choices[i].Outcome)
	}
	for _, field := range fields {
		normalized, err := simplifyGeneratedChinese(*field)
		if err != nil {
			return err
		}
		*field = normalized
	}
	return nil
}

func validateGeneratedAdventure(g generatedAdventure, nickname string) error {
	fields := []struct {
		name  string
		value string
		max   int
	}{
		{"title", g.Title, 18},
		{"location", g.Location, 12},
		{"opening", g.Opening, 200},
		{"encounter_title", g.EncounterTitle, 20},
		{"encounter", g.Encounter, 260},
		{"companion_line", g.CompanionLine, 120},
		{"souvenir.name", g.Souvenir.Name, 12},
		{"souvenir.description", g.Souvenir.Description, 120},
	}
	allText := make([]string, 0, len(fields)+len(g.Choices)*3)
	for _, field := range fields {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("missing adventure field: %s", field.name)
		}
		if !isChineseGeneratedText(field.value, nickname) {
			return fmt.Errorf("adventure field must be Simplified Chinese: %s", field.name)
		}
		if utf8.RuneCountInString(field.value) > field.max {
			return fmt.Errorf("adventure field too long: %s", field.name)
		}
		allText = append(allText, field.value)
	}
	if len(g.Choices) != 3 {
		return fmt.Errorf("adventure must have three choices")
	}
	want := map[string]bool{"courage": false, "curiosity": false, "kindness": false}
	for _, choice := range g.Choices {
		if _, ok := want[choice.ID]; !ok || want[choice.ID] {
			return fmt.Errorf("invalid adventure choice")
		}
		want[choice.ID] = true
		if strings.TrimSpace(choice.Label) == "" || strings.TrimSpace(choice.Description) == "" || strings.TrimSpace(choice.Outcome) == "" {
			return fmt.Errorf("incomplete adventure choice")
		}
		if utf8.RuneCountInString(choice.Label) > 10 || utf8.RuneCountInString(choice.Description) > 100 || utf8.RuneCountInString(choice.Outcome) > 240 {
			return fmt.Errorf("adventure choice field too long")
		}
		if !isChineseGeneratedText(choice.Label, nickname) || !isChineseGeneratedText(choice.Description, nickname) || !isChineseGeneratedText(choice.Outcome, nickname) {
			return fmt.Errorf("adventure choice must be Simplified Chinese")
		}
		allText = append(allText, choice.Label, choice.Description, choice.Outcome)
	}
	if err := narrativepolicy.ValidateOutput(strings.Join(allText, "\n")); err != nil {
		return err
	}
	return nil
}

func applyAdventureMechanics(choices []AdventureChoice) {
	for i := range choices {
		// Every valid response deepens the relationship equally. The choice is
		// role-play, not a hidden optimisation or moral score.
		choices[i].BondDelta = 6
	}
}

func adventureFallback(input AdventureInput) *AdventureResult {
	name := input.Nickname
	if name == "" {
		name = input.SpeciesName
	}
	if name == "" {
		name = "伙伴"
	}
	profileName := input.BreedName
	if profileName == "" || profileName == "品种待确认" {
		profileName = input.SpeciesName
	}

	type scene struct {
		title, location, opening, encounterTitle, encounter, souvenir, souvenirDescription string
	}
	scenes := map[string]scene{
		"mistwood": {
			title: "雾灯森林的回声", location: "萤光岔路",
			opening:        fmt.Sprintf("%s（%s）踏进会发光的苔藓小径，%s属性在雾里留下温柔的光点。远处传来一串像铃铛的回声。", name, profileName, input.ElementName),
			encounterTitle: "迷路的回声精灵", encounter: "一团小小的回声在三条路之间打转，每次呼喊都会变成不同方向的风。它需要你们一起找出真正的归途。",
			souvenir: "回声铃", souvenirDescription: "轻轻摇动时，会响起你们并肩前进的脚步声。",
		},
		"sky_ruins": {
			title: "云上遗迹的星桥", location: "风纹高台",
			opening:        fmt.Sprintf("%s（%s）登上漂浮在云海里的古老石阶，%s的力量让断裂的纹路重新明亮。前方的星桥只在默契的脚步下显形。", name, profileName, input.ClassName),
			encounterTitle: "沉睡的星图机关", encounter: "三枚星盘正以不同节奏旋转，中间那枚却失去了光。你们需要选择一种方式，让整座机关重新唱起歌。",
			souvenir: "云纹徽章", souvenirDescription: "徽章上的云纹会记住你们这次共同的节奏。",
		},
		"tide_isles": {
			title: "潮汐群岛的月贝", location: "月影浅滩",
			opening:        fmt.Sprintf("%s（%s）沿着会随歌声浮起的贝壳路前进，%s属性化成细小光浪陪在身边。海面中央升起一座只存在片刻的小岛。", name, profileName, input.ElementName),
			encounterTitle: "不会开口的月贝", encounter: "巨大的月贝紧闭着，周围三只泡泡精灵正焦急地比划。它似乎在等待一段只属于伙伴之间的暗号。",
			souvenir: "月潮贝片", souvenirDescription: "贝片映着两道并排的影子，像一枚安静的约定。",
		},
	}
	s := scenes[input.Theme]
	choices := []AdventureChoice{
		{ID: "courage", Label: "并肩向前", Description: "和伙伴一起先迈出坚定的一步", Outcome: fmt.Sprintf("你和%s同时踏出一步，眼前的魔法立刻回应了这份信任。它回头看向你，脚步也变得更轻快。", name)},
		{ID: "curiosity", Label: "观察线索", Description: "一起找出环境里隐藏的规律", Outcome: fmt.Sprintf("你们耐心比对每一处微光，%s率先发现了藏在角落的提示。你读懂了它的节奏，它也记住了你的专注。", name)},
		{ID: "kindness", Label: "温柔回应", Description: "先回应这里需要帮助的声音", Outcome: fmt.Sprintf("你和%s用最柔和的方式回应，紧张的魔法生物终于放松下来。那一刻，你们像是共享了同一个小小秘密。", name)},
	}
	applyAdventureMechanics(choices)
	return &AdventureResult{
		AdventureID:    uuid.NewString(),
		Theme:          input.Theme,
		Title:          s.title,
		Location:       s.location,
		Opening:        s.opening,
		EncounterTitle: s.encounterTitle,
		Encounter:      s.encounter,
		CompanionLine:  fmt.Sprintf("%s轻轻望向你，像是在说：这次也一起决定吧。", name),
		Choices:        choices,
		Souvenir:       AdventureSouvenir{Name: s.souvenir, Description: s.souvenirDescription},
		Fiction:        true,
		Disclaimer:     adventureDisclaimer,
		Source:         "template",
		PromptVersion:  prompts.AdventurePromptVersion,
	}
}
