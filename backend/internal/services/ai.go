// Package services MB3: 统一 AI 编排(视觉检测 + 深度分析 + 数值/叙事生成)。
package services

import (
	"animalpoke/backend/internal/middleware"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"text/template"
	"time"
	"unicode/utf8"

	"animalpoke/backend/internal/ai/prompts"
	"animalpoke/backend/internal/narrativepolicy"
	"animalpoke/backend/internal/taxonomy"
)

// ---------- VLM 相关类型 ----------

// DetectBox 检测框。
type DetectBox struct {
	Species     string      `json:"species"`
	Label       string      `json:"label,omitempty"` // 原始标签，仅审计
	TargetID    string      `json:"target_id"`
	Confidence  float64     `json:"confidence"`
	BoundingBox BoundingBox `json:"bounding_box"`
}

// MarshalJSON omits an absent bounding box. Photo-level recognition no longer
// asks the VLM for coordinates, while old stored results may still contain one.
func (d DetectBox) MarshalJSON() ([]byte, error) {
	type detectBoxJSON struct {
		Species     string       `json:"species"`
		Label       string       `json:"label,omitempty"`
		TargetID    string       `json:"target_id"`
		Confidence  float64      `json:"confidence"`
		BoundingBox *BoundingBox `json:"bounding_box,omitempty"`
	}
	var box *BoundingBox
	if d.BoundingBox != (BoundingBox{}) {
		box = &d.BoundingBox
	}
	return json.Marshal(detectBoxJSON{
		Species: d.Species, Label: d.Label, TargetID: d.TargetID,
		Confidence: d.Confidence, BoundingBox: box,
	})
}

// BoundingBox 归一化检测框（0~1）。
type BoundingBox struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// DetectResult VLM 检测结果（标准 envelope）。
// animals 保留兼容；targets 为多目标权威列表（与 animals 同步）。
type DetectResult struct {
	Animals       []DetectBox `json:"animals"`
	Targets       []DetectBox `json:"targets,omitempty"`
	Source        string      `json:"source,omitempty"` // real|mock|cache|safety
	Degraded      bool        `json:"degraded,omitempty"`
	ReasonCode    string      `json:"reason_code,omitempty"`
	InferenceID   string      `json:"inference_id,omitempty"`
	Model         string      `json:"model,omitempty"`
	PromptVersion string      `json:"prompt_version,omitempty"`
	// Safety is the public moderation decision (AP-056); never includes model internals or images.
	Safety *SafetySummary `json:"safety,omitempty"`
	// SafetyLabels free-text signals for moderation only (never serialized to clients).
	SafetyLabels []string `json:"-"`
}

// SafetySummary public moderation fields attached to vision responses.
type SafetySummary struct {
	Allowed      bool     `json:"allowed"`
	Collectable  bool     `json:"collectable"`
	DecisionCode string   `json:"decision_code"`
	Action       string   `json:"action"`
	Flags        []string `json:"flags,omitempty"`
	ReportPath   string   `json:"report_path,omitempty"`
}

// AnalysisResult 深度分析结果。
type AnalysisResult struct {
	SpeciesLabelZH      string `json:"species_label_zh,omitempty"`
	Breed               string `json:"breed"`
	Color               string `json:"color"`
	BodyType            string `json:"body_type"`
	QualityScore        int    `json:"quality_score"`
	SubjectCompleteness int    `json:"subject_completeness"`
	Clarity             int    `json:"clarity"`
	Lighting            int    `json:"lighting"`
	Composition         int    `json:"composition"`
	Pose                int    `json:"pose"`
	Angle               int    `json:"angle"`
	// 多目标一致性：回传锁定目标
	Species           string         `json:"species,omitempty"`
	TargetID          string         `json:"target_id,omitempty"`
	DetectInferenceID string         `json:"detect_inference_id,omitempty"`
	Box               *BoundingBox   `json:"box,omitempty"`
	Source            string         `json:"source,omitempty"`
	Degraded          bool           `json:"degraded,omitempty"`
	ReasonCode        string         `json:"reason_code,omitempty"`
	InferenceID       string         `json:"inference_id,omitempty"`
	Model             string         `json:"model,omitempty"`
	PromptVersion     string         `json:"prompt_version,omitempty"`
	Safety            *SafetySummary `json:"safety,omitempty"`
}

// ---------- LLM 相关类型 ----------

// ValueResult 数值生成结果（稀有度/属性由确定性算法产生；narrative 可来自 LLM）。
type ValueResult struct {
	SpeciesLabelZH string        `json:"species_label_zh,omitempty"`
	Rarity         int           `json:"rarity"`    // 1-5 星级
	HP             int           `json:"hp"`        // 10-100
	ATK            int           `json:"atk"`       // 5-50
	DEF            int           `json:"def"`       // 5-50
	SPD            int           `json:"spd"`       // 5-50
	Class          string        `json:"class"`     // Warrior/Mage/Ranger/Tank/Support/Assassin
	Element        string        `json:"element"`   // Fire/Water/Grass/Electric/Ice/Dark/Light/Earth/Wind
	Narrative      string        `json:"narrative"` // 虚构伙伴描述（非事实）
	Fiction        bool          `json:"fiction"`   // 恒为 true：LLM/模板输出均为虚构
	Disclaimer     string        `json:"disclaimer,omitempty"`
	Layer          string        `json:"layer,omitempty"` // fictional_vignette | authored_canon
	PolicyVersion  string        `json:"policy_version,omitempty"`
	Factors        *ValueFactors `json:"factors,omitempty"`
	ConfigVersion  string        `json:"config_version,omitempty"`
	SeedID         string        `json:"seed_id,omitempty"`
	Source         string        `json:"source,omitempty"`
	Degraded       bool          `json:"degraded,omitempty"`
	ReasonCode     string        `json:"reason_code,omitempty"`
	InferenceID    string        `json:"inference_id,omitempty"`
	// AnimalUUID 是服务端在 value 完成后直接创建的动物记录标识。
	AnimalUUID    string `json:"animal_uuid,omitempty"`
	Model         string `json:"model,omitempty"`
	PromptVersion string `json:"prompt_version,omitempty"`
}

// ValueInput 数值生成输入（分析字段 + 稳定 seed）。
type ValueInput struct {
	Species             string
	SpeciesLabelZH      string
	Breed               string
	Color               string
	BodyType            string
	SubjectCompleteness int
	Clarity             int
	Lighting            int
	Composition         int
	Pose                int
	Angle               int
	// SeedID capture/parent inference id；同一 SeedID + config 永远产出相同 rarity/stats。
	SeedID string
}

// Validate 校验 Value 输入边界。
func (in ValueInput) Validate() error {
	if strings.TrimSpace(in.Species) == "" {
		return fmt.Errorf("species is required")
	}
	norm, _ := taxonomy.Normalize(in.Species)
	if !taxonomy.Capturable(norm) {
		return fmt.Errorf("species not capturable: %s", in.Species)
	}
	in.Species = norm // note: caller should re-assign; normalize checked only
	if strings.TrimSpace(in.SpeciesLabelZH) != "" {
		label, labelSpecies, err := NormalizeConcreteAnimalLabel(in.SpeciesLabelZH)
		if err != nil || labelSpecies != norm {
			return fmt.Errorf("species_label_zh does not match species")
		}
		in.SpeciesLabelZH = label
	} else if norm == "other_animal" {
		return fmt.Errorf("species_label_zh required for other_animal")
	}
	if len(in.Species) > 64 || len(in.Breed) > 64 || len(in.Color) > 64 || len(in.BodyType) > 64 {
		return fmt.Errorf("string fields exceed max length 64")
	}
	for _, v := range []int{in.SubjectCompleteness, in.Clarity, in.Lighting, in.Composition, in.Pose, in.Angle} {
		if v != 0 && (v < 1 || v > 10) {
			return fmt.Errorf("quality scores must be 1-10")
		}
	}
	return nil
}

// Validate 校验 Value 输出边界。
func (v *ValueResult) Validate() error {
	if v.Rarity < 1 || v.Rarity > 5 {
		return fmt.Errorf("rarity out of range")
	}
	if v.HP < 10 || v.HP > 100 || v.ATK < 5 || v.ATK > 50 || v.DEF < 5 || v.DEF > 50 || v.SPD < 5 || v.SPD > 50 {
		return fmt.Errorf("stats out of range")
	}
	validClass := map[string]bool{"Warrior": true, "Mage": true, "Ranger": true, "Tank": true, "Support": true, "Assassin": true}
	validElement := map[string]bool{"Fire": true, "Water": true, "Grass": true, "Electric": true, "Ice": true, "Dark": true, "Light": true, "Earth": true, "Wind": true}
	if !validClass[v.Class] {
		return fmt.Errorf("invalid class")
	}
	if !validElement[v.Element] {
		return fmt.Errorf("invalid element")
	}
	if len(v.Narrative) > 2000 {
		return fmt.Errorf("narrative too long")
	}
	return nil
}

// ---------- 视觉方法 ----------

// Detect 调用 Vision 进行动物检测。imageData 为图片字节数据, 推理后不落盘。
func (s *AIService) Detect(imageData []byte, filename string) (*DetectResult, error) {
	return s.DetectContext(context.Background(), imageData, filename)
}

// DetectContext 带 context 的检测。
func (s *AIService) DetectContext(ctx context.Context, imageData []byte, filename string) (*DetectResult, error) {
	observe := func(provider, outcome string, conf float64, empty bool) {
		middleware.ObserveAI("detect", provider, outcome)
		if empty {
			middleware.ObserveDetectEmpty()
		}
		if conf > 0 {
			middleware.ObserveConfidence(conf)
		}
		middleware.ObserveFunnel("detect", outcome)
	}
	if !s.cfg.VisionConfigured() {
		if !s.mock {
			return nil, fmt.Errorf("vision provider not configured")
		}
		slog.Debug("Vision 未配置, 返回 mock 检测结果")
		r := mockDetect()
		r.Source = "mock"
		r.Degraded = true
		r.ReasonCode = "provider_not_configured"
		r.PromptVersion = prompts.DetectPromptVersion
		observe("mock", "ok", 0.5, len(r.Animals) == 0)
		return r, nil
	}

	body, model, err := s.callVision(ctx, imageData, filename, prompts.DetectPrompt)
	if err != nil {
		observe("vision", "error", 0, false)
		return nil, err
	}

	jsonStr, err := s.parseResponsesResponse(body)
	if err != nil {
		observe("vision", "error", 0, false)
		return nil, err
	}
	result, err := parseDetectJSON(jsonStr)
	if err != nil {
		observe("vision", "error", 0, false)
		return nil, err
	}
	if err := validateDetectResult(result); err != nil {
		observe("vision", "error", 0, false)
		return nil, err
	}
	result.Source = "real"
	middleware.ObserveAI("value", "llm", "ok")
	middleware.ObserveFunnel("value", "ok")
	result.Model = model
	result.PromptVersion = prompts.DetectPromptVersion
	maxConf := 0.0
	for _, box := range result.Animals {
		if box.Confidence > maxConf {
			maxConf = box.Confidence
		}
	}
	observe("vision", "ok", maxConf, len(result.Animals) == 0)
	return result, nil
}

// Analyze 调用 Vision 进行动物深度分析。
func (s *AIService) Analyze(imageData []byte, filename string) (*AnalysisResult, error) {
	return s.AnalyzeContext(context.Background(), imageData, filename)
}

// AnalyzeContext 带 context 的分析。
func (s *AIService) AnalyzeContext(ctx context.Context, imageData []byte, filename string) (*AnalysisResult, error) {
	observeA := func(provider, outcome string) {
		middleware.ObserveAI("analyze", provider, outcome)
		middleware.ObserveFunnel("analyze", outcome)
	}
	if !s.cfg.VisionConfigured() {
		if !s.mock {
			return nil, fmt.Errorf("vision provider not configured")
		}
		slog.Debug("Vision 未配置, 返回 mock 分析结果")
		r := mockAnalyze()
		r.Source = "mock"
		r.Degraded = true
		r.ReasonCode = "provider_not_configured"
		r.PromptVersion = prompts.AnalyzePromptVersion
		observeA("mock", "ok")
		return r, nil
	}

	body, model, err := s.callVision(ctx, imageData, filename, prompts.AnalyzePrompt)
	if err != nil {
		observeA("vision", "error")
		return nil, err
	}

	var result AnalysisResult
	jsonStr, err := s.parseResponsesResponse(body)
	if err != nil {
		return nil, err
	}
	if err := parseAnalysisJSON(jsonStr, &result); err != nil {
		return nil, err
	}
	if err := simplifyAnalysisDescriptions(&result); err != nil {
		return nil, err
	}
	if err := validateAnalysisResult(&result); err != nil {
		return nil, err
	}
	result.Source = "real"
	result.Model = model
	result.PromptVersion = prompts.AnalyzePromptVersion
	observeA("vision", "ok")
	return &result, nil
}

// ---------- 文本生成方法 ----------

// GenerateValue 确定性 rarity/stats + 可选 LLM 叙事。
func (s *AIService) GenerateValue(input ValueInput) (*ValueResult, error) {
	return s.GenerateValueContext(context.Background(), input)
}

// GenerateValueContext 服务端权威：HMAC 派生 rarity/stats；LLM 仅补 narrative。
func (s *AIService) GenerateValueContext(ctx context.Context, input ValueInput) (*ValueResult, error) {
	if err := input.Validate(); err != nil {
		middleware.ObserveAI("value", "unknown", "error")
		middleware.ObserveFunnel("value", "error")
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	result := ComputeDeterministicValue(input, input.SeedID, s.statsSecret, StatsConfigVersion)
	result.PromptVersion = prompts.ValuePromptVersion
	result.Fiction = true
	result.Disclaimer = "人工智能生成的中文幻想描述，不代表真实动物的经历、情绪或需求"
	result.Layer = "fictional_vignette"
	result.PolicyVersion = prompts.NarrativePolicyVersion

	// Vision-derived labels are untrusted input. Never let instruction-shaped
	// fields reach the LLM or be echoed by the deterministic fallback.
	if err := narrativepolicy.ValidatePromptInput(input.Breed, input.Color, input.BodyType); err != nil {
		result.Narrative = narrativeFallback(ValueInput{Species: input.Species}, result)
		result.Source = "algo"
		result.Degraded = true
		result.ReasonCode = "narrative_input_policy_blocked"
		return result, nil
	}

	// LLM 仅叙事；未配置时用确定性模板
	if !s.cfg.LLMConfigured() {
		if !s.mock {
			return nil, fmt.Errorf("llm provider not configured")
		}
		slog.Debug("LLM 未配置, 使用确定性 stats + mock 叙事")
		result.Narrative = narrativeFallback(input, result)
		result.Source = "mock"
		result.Degraded = true
		middleware.ObserveAI("value", "mock", "ok")
		middleware.ObserveFunnel("value", "ok")
		result.ReasonCode = "provider_not_configured"
		return result, nil
	}

	prompt, err := renderValuePrompt(input)
	if err != nil {
		return nil, err
	}
	if strings.Contains(prompt, "{{") {
		return nil, fmt.Errorf("prompt render incomplete")
	}

	body, model, err := s.callText(ctx, prompt)
	if err != nil {
		slog.Warn("LLM 叙事失败, 使用确定性模板", "err", err)
		result.Narrative = narrativeFallback(input, result)
		result.Source = "algo"
		result.Degraded = true
		result.ReasonCode = "narrative_fallback"
		result.Model = model
		return result, nil
	}

	jsonStr, err := s.parseResponsesResponse(body)
	if err != nil {
		result.Narrative = narrativeFallback(input, result)
		result.Source = "algo"
		result.Degraded = true
		result.ReasonCode = "narrative_parse_failed"
		result.Model = model
		return result, nil
	}
	var llmOut struct {
		Narrative  string `json:"narrative"`
		Fiction    *bool  `json:"fiction"`
		Disclaimer string `json:"disclaimer"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &llmOut); err != nil || strings.TrimSpace(llmOut.Narrative) == "" {
		// 兼容旧模型仍返回完整 value JSON：只取 narrative，忽略 rarity/stats
		var full ValueResult
		if err2 := json.Unmarshal([]byte(jsonStr), &full); err2 == nil && strings.TrimSpace(full.Narrative) != "" {
			result.Narrative = full.Narrative
		} else {
			result.Narrative = narrativeFallback(input, result)
			result.Degraded = true
			result.ReasonCode = "narrative_missing"
		}
	} else {
		result.Narrative = llmOut.Narrative
	}
	if normalized, err := simplifyGeneratedChinese(result.Narrative); err == nil {
		result.Narrative = normalized
	} else {
		result.Narrative = narrativeFallback(input, result)
		result.Degraded = true
		result.ReasonCode = "narrative_conversion_failed"
	}
	// 无论 LLM 是否返回 fiction 字段，服务端强制虚构层
	result.Fiction = true
	result.Layer = "fictional_vignette"
	if guidance, sensitive := narrativepolicy.SafetyGuidance(result.Narrative); sensitive {
		result.Narrative = guidance
		result.Degraded = true
		result.ReasonCode = "narrative_safety_guidance"
	}
	if err := narrativepolicy.ValidateOutput(result.Narrative); err != nil {
		result.Narrative = narrativeFallback(ValueInput{Species: input.Species}, result)
		result.Degraded = true
		result.ReasonCode = "narrative_policy_blocked"
	} else if utf8.RuneCountInString(result.Narrative) > 180 || !isChineseGeneratedText(result.Narrative) {
		result.Narrative = narrativeFallback(input, result)
		result.Degraded = true
		result.ReasonCode = "narrative_not_chinese"
	}
	if err := narrativepolicy.ValidateOutput(result.Narrative); err != nil {
		result.Narrative = narrativeFallback(ValueInput{Species: input.Species}, result)
		result.Degraded = true
		result.ReasonCode = "narrative_policy_blocked"
	}
	result.Source = "algo"
	result.Model = model
	return result, nil
}

// ---------- 统一 HTTP 调用 ----------

func (s *AIService) callVision(ctx context.Context, imageData []byte, filename, prompt string) ([]byte, string, error) {
	mimeType := http.DetectContentType(imageData)
	if !strings.HasPrefix(mimeType, "image/") {
		return nil, "", fmt.Errorf("unsupported content type: %s", mimeType)
	}
	b64 := base64.StdEncoding.EncodeToString(imageData)
	dataURL := "data:" + mimeType + ";base64," + b64

	input := []map[string]interface{}{{
		"role": "user",
		"content": []map[string]interface{}{
			{"type": "input_text", "text": prompt},
			{"type": "input_image", "image_url": dataURL},
		},
	}}
	_ = filename
	return s.callResponses(ctx, input)
}

func (s *AIService) callText(ctx context.Context, prompt string) ([]byte, string, error) {
	input := []map[string]interface{}{{
		"role": "user",
		"content": []map[string]interface{}{
			{"type": "input_text", "text": prompt},
		},
	}}
	return s.callResponses(ctx, input)
}

// callResponses is the sole model transport. The configured provider must
// implement the OpenAI Responses API, including input_text/input_image and
// output_text response items.
func (s *AIService) callResponses(ctx context.Context, input interface{}) ([]byte, string, error) {
	endpoint, key, model := s.cfg.ActiveAI()
	body := map[string]interface{}{
		"model":             model,
		"input":             input,
		"max_output_tokens": 1024,
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, "", fmt.Errorf("marshal body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(bodyBytes)), nil
	}

	// One unified multimodal Provider budget/circuit now covers every AI call.
	provider := s.visionProvider
	if provider == nil {
		provider = s.llmProvider
	}

	var (
		resp     *http.Response
		respBody []byte
	)
	if provider != nil {
		resp, respBody, err = provider.Do(ctx, req, defaultMaxResponseBytes)
	} else {
		client := s.client
		if client == nil {
			client = DefaultHTTPClient(30 * time.Second)
		}
		resp, respBody, err = DoWithRetry(ctx, client, req, defaultMaxRetries, defaultMaxResponseBytes)
	}
	if err != nil {
		return nil, "", err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("ai returned status %d: %s", resp.StatusCode, truncate(string(respBody), 200))
	}
	return respBody, model, nil
}

func (s *AIService) parseResponsesResponse(body []byte) (string, error) {
	var response struct {
		OutputText string `json:"output_text"`
		Output     []struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("unmarshal responses api response: %w", err)
	}
	content := strings.TrimSpace(response.OutputText)
	if content == "" {
		for _, item := range response.Output {
			for _, part := range item.Content {
				if part.Type == "output_text" && strings.TrimSpace(part.Text) != "" {
					content = part.Text
					break
				}
			}
			if content != "" {
				break
			}
		}
	}
	if content == "" {
		return "", fmt.Errorf("empty responses api output_text")
	}
	return extractJSON(content), nil
}

// parseDetectJSON 兼容 envelope 与裸数组；拒绝 Markdown 残留。
func parseDetectJSON(jsonStr string) (*DetectResult, error) {
	jsonStr = strings.TrimSpace(jsonStr)
	if jsonStr == "" {
		return nil, fmt.Errorf("empty detect json")
	}
	if strings.Contains(jsonStr, "```") {
		return nil, fmt.Errorf("detect json contains markdown fence")
	}
	// 标准 envelope
	var env DetectResult
	if err := json.Unmarshal([]byte(jsonStr), &env); err == nil && (env.Animals != nil || env.Targets != nil || strings.HasPrefix(jsonStr, "{")) {
		if env.Animals == nil && env.Targets != nil {
			env.Animals = env.Targets
		}
		if env.Animals == nil {
			env.Animals = []DetectBox{}
		}
		return &env, nil
	}
	// 兼容裸数组
	var arr []DetectBox
	if err := json.Unmarshal([]byte(jsonStr), &arr); err == nil {
		return &DetectResult{Animals: arr}, nil
	}
	return nil, fmt.Errorf("unable to parse detect json")
}

// minBoxArea 归一化最小框面积，过滤噪声点/退化框。
const minBoxArea = 0.0001

func validateDetectResult(r *DetectResult) error {
	if r == nil {
		return fmt.Errorf("nil detect result")
	}
	if len(r.Animals) == 0 && len(r.Targets) > 0 {
		r.Animals = r.Targets
	}
	normalized := make([]DetectBox, 0, len(r.Animals))
	safetyLabels := make([]string, 0, len(r.Animals)*2)
	seenIDs := make(map[string]int)
	for i, a := range r.Animals {
		if a.Confidence < 0 || a.Confidence > 1 {
			return fmt.Errorf("animal[%d] confidence out of range", i)
		}
		// Bounding boxes belong to the retired multi-target UI. Accept legacy
		// coordinates when supplied, but photo-level VLM responses omit them.
		if a.BoundingBox != (BoundingBox{}) {
			if err := ValidateBoundingBox(a.BoundingBox); err != nil {
				return fmt.Errorf("animal[%d] %w", i, err)
			}
		}
		// 权威 taxonomy：规范化物种，禁止静默映射为鹅
		raw := a.Species
		if raw == "" {
			raw = a.Label
		}
		if raw != "" {
			safetyLabels = append(safetyLabels, raw)
		}
		if a.Label != "" && a.Label != raw {
			safetyLabels = append(safetyLabels, a.Label)
		}
		norm, orig := taxonomy.Normalize(raw)
		a.Species = norm
		if a.Label == "" {
			a.Label = orig
		}
		if a.TargetID == "" {
			a.TargetID = fmt.Sprintf("%d", i)
		}
		if n, ok := seenIDs[a.TargetID]; ok {
			a.TargetID = fmt.Sprintf("%s_%d", a.TargetID, n)
			seenIDs[a.TargetID] = 1
			seenIDs[fmt.Sprintf("%d", i)] = n + 1
		} else {
			seenIDs[a.TargetID] = 1
		}
		// 仅 capturable 进入返回列表；unknown/unsupported 不进入捕获
		if taxonomy.Capturable(norm) {
			if norm == "other_animal" {
				label, species, err := NormalizeConcreteAnimalLabel(a.Label)
				if err != nil {
					return fmt.Errorf("animal[%d] invalid other_animal label: %w", i, err)
				}
				a.Species = species
				a.Label = label
			} else {
				labelNorm, _ := taxonomy.Normalize(a.Label)
				if labelNorm == taxonomy.SpeciesUnsupported {
					return fmt.Errorf("animal[%d] label is unsupported", i)
				}
				if labelNorm != taxonomy.SpeciesUnknown && labelNorm != norm {
					return fmt.Errorf("animal[%d] species and label mismatch", i)
				}
				a.Label = chineseSpecies(norm)
			}
			normalized = append(normalized, a)
		} else if orig != "" {
			safetyLabels = append(safetyLabels, orig, norm)
		}
	}
	// 稳定排序：confidence desc, species, target_id
	for i := range normalized {
		for j := i + 1; j < len(normalized); j++ {
			a, b := normalized[i], normalized[j]
			swap := false
			if b.Confidence > a.Confidence {
				swap = true
			} else if b.Confidence == a.Confidence {
				if b.Species < a.Species {
					swap = true
				} else if b.Species == a.Species && b.TargetID < a.TargetID {
					swap = true
				}
			}
			if swap {
				normalized[i], normalized[j] = normalized[j], normalized[i]
			}
		}
	}
	r.Animals = normalized
	r.Targets = append([]DetectBox(nil), normalized...)
	r.SafetyLabels = safetyLabels
	return nil
}

// parseAnalysisJSON 严格解析单对象；拒绝多段 JSON。
func parseAnalysisJSON(jsonStr string, out *AnalysisResult) error {
	jsonStr = strings.TrimSpace(jsonStr)
	if jsonStr == "" {
		return fmt.Errorf("empty analysis json")
	}
	if strings.Contains(jsonStr, "```") {
		return fmt.Errorf("analysis json contains markdown fence")
	}
	if !strings.HasPrefix(jsonStr, "{") {
		return fmt.Errorf("analysis json must be object")
	}
	dec := json.NewDecoder(strings.NewReader(jsonStr))
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("analysis json decode: %w", err)
	}
	var second interface{}
	if err := dec.Decode(&second); err == nil {
		return fmt.Errorf("analysis json contains multiple values")
	}
	return nil
}

func simplifyAnalysisDescriptions(result *AnalysisResult) error {
	if result == nil {
		return fmt.Errorf("nil analysis result")
	}
	for _, field := range []*string{&result.Breed, &result.Color, &result.BodyType} {
		normalized, err := simplifyGeneratedChinese(*field)
		if err != nil {
			return err
		}
		*field = normalized
	}
	return nil
}

// validateAnalysisResult 严格校验枚举/长度/1-10 分值，不静默 clamp。
func validateAnalysisResult(r *AnalysisResult) error {
	if r == nil {
		return fmt.Errorf("nil analysis result")
	}
	if strings.TrimSpace(r.Breed) == "" || len(r.Breed) > 64 {
		return fmt.Errorf("breed missing or too long")
	}
	if strings.TrimSpace(r.Color) == "" || len(r.Color) > 64 {
		return fmt.Errorf("color missing or too long")
	}
	if strings.TrimSpace(r.BodyType) == "" || len(r.BodyType) > 64 {
		return fmt.Errorf("body_type missing or too long")
	}
	if !isChineseGeneratedText(r.Breed) || !isChineseGeneratedText(r.Color) || !isChineseGeneratedText(r.BodyType) {
		return fmt.Errorf("analysis descriptions must be Simplified Chinese")
	}
	scores := []struct {
		name string
		v    int
	}{
		{"quality_score", r.QualityScore},
		{"subject_completeness", r.SubjectCompleteness},
		{"clarity", r.Clarity},
		{"lighting", r.Lighting},
		{"composition", r.Composition},
		{"pose", r.Pose},
		{"angle", r.Angle},
	}
	for _, s := range scores {
		if s.v < 1 || s.v > 10 {
			return fmt.Errorf("%s out of range (must be 1-10)", s.name)
		}
	}
	return nil
}

// ValidateBoundingBox 校验归一化框边界与面积。
func ValidateBoundingBox(bb BoundingBox) error {
	if bb.X < 0 || bb.Y < 0 || bb.Width < 0 || bb.Height < 0 ||
		bb.X > 1 || bb.Y > 1 || bb.Width > 1 || bb.Height > 1 ||
		bb.X+bb.Width > 1.0001 || bb.Y+bb.Height > 1.0001 {
		return fmt.Errorf("bounding_box out of range")
	}
	if bb.Width*bb.Height < minBoxArea {
		return fmt.Errorf("bounding_box area too small")
	}
	return nil
}

// FindTarget 在 detect 结果中按 target_id 或 box 匹配目标。
func FindTarget(targets []DetectBox, targetID string, box *BoundingBox) (*DetectBox, error) {
	if targetID != "" {
		for i := range targets {
			if targets[i].TargetID == targetID {
				t := targets[i]
				return &t, nil
			}
		}
		return nil, fmt.Errorf("target_id not found")
	}
	if box != nil {
		if err := ValidateBoundingBox(*box); err != nil {
			return nil, err
		}
		bestIdx := -1
		bestIoU := 0.0
		for i := range targets {
			iou := boxIoU(targets[i].BoundingBox, *box)
			if iou > bestIoU {
				bestIoU = iou
				bestIdx = i
			}
		}
		if bestIdx >= 0 && bestIoU >= 0.3 {
			t := targets[bestIdx]
			return &t, nil
		}
		return nil, fmt.Errorf("box does not match any target")
	}
	return nil, fmt.Errorf("target_id or box required")
}

func boxIoU(a, b BoundingBox) float64 {
	ax2, ay2 := a.X+a.Width, a.Y+a.Height
	bx2, by2 := b.X+b.Width, b.Y+b.Height
	ix1 := math.Max(a.X, b.X)
	iy1 := math.Max(a.Y, b.Y)
	ix2 := math.Min(ax2, bx2)
	iy2 := math.Min(ay2, by2)
	iw := math.Max(0, ix2-ix1)
	ih := math.Max(0, iy2-iy1)
	inter := iw * ih
	if inter <= 0 {
		return 0
	}
	union := a.Width*a.Height + b.Width*b.Height - inter
	if union <= 0 {
		return 0
	}
	return inter / union
}

// ---------- Prompt 渲染 ----------

var valuePromptTmpl = template.Must(template.New("value").Parse(prompts.ValuePrompt))

func renderValuePrompt(input ValueInput) (string, error) {
	var buf bytes.Buffer
	if err := valuePromptTmpl.Execute(&buf, input); err != nil {
		return "", fmt.Errorf("render value prompt: %w", err)
	}
	return buf.String(), nil
}

// ---------- JSON 提取 ----------

func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.Index(s, "```json"); idx >= 0 {
		s = s[idx+7:]
	} else if idx := strings.Index(s, "```"); idx >= 0 {
		s = s[idx+3:]
	}
	if idx := strings.LastIndex(s, "```"); idx >= 0 {
		s = s[:idx]
	}
	return strings.TrimSpace(s)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// ---------- Mock 函数(开发/测试用) ----------

func mockDetect() *DetectResult {
	r := &DetectResult{
		Animals: []DetectBox{
			{
				Species:    "cat",
				TargetID:   "0",
				Confidence: 0.92,
			},
		},
	}
	_ = validateDetectResult(r)
	return r
}

func mockAnalyze() *AnalysisResult {
	return &AnalysisResult{
		Breed:               "英国短毛猫",
		Color:               "蓝灰色",
		BodyType:            "敦实",
		QualityScore:        8,
		SubjectCompleteness: 9,
		Clarity:             8,
		Lighting:            7,
		Composition:         8,
		Pose:                7,
		Angle:               9,
	}
}

// mockValue 确定性 mock：与生产同一算法，仅叙事用模板。
func mockValue(input ValueInput) *ValueResult {
	r := ComputeDeterministicValue(input, input.SeedID, "", StatsConfigVersion)
	r.Narrative = narrativeFallback(input, r)
	return r
}

// ScoreString 用于日志脱敏，不输出敏感内容。
func ScoreString(n int) string {
	return strconv.Itoa(n)
}
