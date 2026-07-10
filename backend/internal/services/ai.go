// Package services MB3: 统一 AI 编排(视觉检测 + 深度分析 + 数值/叙事生成)。
package services

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"text/template"
	"time"

	"animalpoke/backend/internal/ai/prompts"
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
	Targets       []DetectBox `json:"targets"`
	Source        string      `json:"source,omitempty"` // real|mock|cache
	Degraded      bool        `json:"degraded,omitempty"`
	ReasonCode    string      `json:"reason_code,omitempty"`
	InferenceID   string      `json:"inference_id,omitempty"`
	Model         string      `json:"model,omitempty"`
	PromptVersion string      `json:"prompt_version,omitempty"`
}

// AnalysisResult 深度分析结果。
type AnalysisResult struct {
	Breed               string       `json:"breed"`
	Color               string       `json:"color"`
	BodyType            string       `json:"body_type"`
	QualityScore        int          `json:"quality_score"`
	SubjectCompleteness int          `json:"subject_completeness"`
	Clarity             int          `json:"clarity"`
	Lighting            int          `json:"lighting"`
	Composition         int          `json:"composition"`
	Pose                int          `json:"pose"`
	Angle               int          `json:"angle"`
	// 多目标一致性：回传锁定目标
	Species           string       `json:"species,omitempty"`
	TargetID          string       `json:"target_id,omitempty"`
	DetectInferenceID string       `json:"detect_inference_id,omitempty"`
	Box               *BoundingBox `json:"box,omitempty"`
	Source            string       `json:"source,omitempty"`
	Degraded          bool         `json:"degraded,omitempty"`
	ReasonCode        string       `json:"reason_code,omitempty"`
	InferenceID       string       `json:"inference_id,omitempty"`
	Model             string       `json:"model,omitempty"`
	PromptVersion     string       `json:"prompt_version,omitempty"`
}

// ---------- LLM 相关类型 ----------

// ValueResult LLM 数值生成结果。
type ValueResult struct {
	Rarity        int    `json:"rarity"`    // 1-5 星级
	HP            int    `json:"hp"`        // 10-100
	ATK           int    `json:"atk"`       // 5-50
	DEF           int    `json:"def"`       // 5-50
	SPD           int    `json:"spd"`       // 5-50
	Class         string `json:"class"`     // Warrior/Mage/Ranger/Tank/Support/Assassin
	Element       string `json:"element"`   // Fire/Water/Grass/Electric/Ice/Dark/Light/Earth/Wind
	Narrative     string `json:"narrative"` // 2-3 句叙事
	Source        string `json:"source,omitempty"`
	Degraded      bool   `json:"degraded,omitempty"`
	ReasonCode    string `json:"reason_code,omitempty"`
	InferenceID   string `json:"inference_id,omitempty"`
	Model         string `json:"model,omitempty"`
	PromptVersion string `json:"prompt_version,omitempty"`
}

// ValueInput LLM 输入参数。
type ValueInput struct {
	Species             string
	Breed               string
	Color               string
	BodyType            string
	SubjectCompleteness int
	Clarity             int
	Lighting            int
	Composition         int
	Pose                int
	Angle               int
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
		return r, nil
	}

	body, model, err := s.callVision(ctx, imageData, filename, prompts.DetectPrompt)
	if err != nil {
		return nil, err
	}

	jsonStr, err := s.parseChatResponse(body)
	if err != nil {
		return nil, err
	}
	result, err := parseDetectJSON(jsonStr)
	if err != nil {
		return nil, err
	}
	if err := validateDetectResult(result); err != nil {
		return nil, err
	}
	result.Source = "real"
	result.Model = model
	result.PromptVersion = prompts.DetectPromptVersion
	return result, nil
}

// Analyze 调用 Vision 进行动物深度分析。
func (s *AIService) Analyze(imageData []byte, filename string) (*AnalysisResult, error) {
	return s.AnalyzeContext(context.Background(), imageData, filename)
}

// AnalyzeContext 带 context 的分析。
func (s *AIService) AnalyzeContext(ctx context.Context, imageData []byte, filename string) (*AnalysisResult, error) {
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
		return r, nil
	}

	body, model, err := s.callVision(ctx, imageData, filename, prompts.AnalyzePrompt)
	if err != nil {
		return nil, err
	}

	var result AnalysisResult
	jsonStr, err := s.parseChatResponse(body)
	if err != nil {
		return nil, err
	}
	if err := parseAnalysisJSON(jsonStr, &result); err != nil {
		return nil, err
	}
	if err := validateAnalysisResult(&result); err != nil {
		return nil, err
	}
	result.Source = "real"
	result.Model = model
	result.PromptVersion = prompts.AnalyzePromptVersion
	return &result, nil
}

// ---------- 文本生成方法 ----------

// GenerateValue 调用 LLM 生成稀有度/六维属性/叙事。
func (s *AIService) GenerateValue(input ValueInput) (*ValueResult, error) {
	return s.GenerateValueContext(context.Background(), input)
}

// GenerateValueContext 带 context 的数值生成。
func (s *AIService) GenerateValueContext(ctx context.Context, input ValueInput) (*ValueResult, error) {
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	if !s.cfg.LLMConfigured() {
		if !s.mock {
			return nil, fmt.Errorf("llm provider not configured")
		}
		slog.Debug("LLM 未配置, 返回 mock 数值")
		r := mockValue(input)
		r.Source = "mock"
		r.Degraded = true
		r.ReasonCode = "provider_not_configured"
		r.PromptVersion = prompts.ValuePromptVersion
		return r, nil
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
		return nil, err
	}

	jsonStr, err := s.parseChatResponse(body)
	if err != nil {
		return nil, err
	}
	var result ValueResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("unmarshal value result: %w", err)
	}
	if err := result.Validate(); err != nil {
		// 一次受控修复：clamp 非法字段后重验
		clampValueResult(&result)
		if err2 := result.Validate(); err2 != nil {
			return nil, fmt.Errorf("invalid model output: %w", err)
		}
	}
	result.Source = "real"
	result.Model = model
	result.PromptVersion = prompts.ValuePromptVersion
	return &result, nil
}

// ---------- 统一 HTTP 调用 ----------

func (s *AIService) callVision(ctx context.Context, imageData []byte, filename, prompt string) ([]byte, string, error) {
	mimeType := http.DetectContentType(imageData)
	if !strings.HasPrefix(mimeType, "image/") {
		return nil, "", fmt.Errorf("unsupported content type: %s", mimeType)
	}
	b64 := base64.StdEncoding.EncodeToString(imageData)
	dataURL := "data:" + mimeType + ";base64," + b64

	content := []map[string]interface{}{
		{"type": "image_url", "image_url": map[string]string{"url": dataURL}},
		{"type": "text", "text": prompt},
	}
	_ = filename
	return s.callProvider(ctx, s.cfg.VisionEndpoint, s.cfg.VisionKey, s.cfg.VisionModel, content)
}

func (s *AIService) callText(ctx context.Context, prompt string) ([]byte, string, error) {
	return s.callProvider(ctx, s.cfg.LLMEndpoint, s.cfg.LLMKey, s.cfg.LLMModel, prompt)
}

func (s *AIService) callProvider(ctx context.Context, endpoint, key, model string, content interface{}) ([]byte, string, error) {
	body := map[string]interface{}{
		"model": model,
		"messages": []map[string]interface{}{
			{"role": "user", "content": content},
		},
		"max_tokens": 1024,
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

	client := s.client
	if client == nil {
		client = DefaultHTTPClient(30 * time.Second)
	}
	resp, respBody, err := DoWithRetry(ctx, client, req, defaultMaxRetries, defaultMaxResponseBytes)
	if err != nil {
		return nil, "", err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("ai returned status %d: %s", resp.StatusCode, truncate(string(respBody), 200))
	}
	return respBody, model, nil
}

func (s *AIService) parseChatResponse(body []byte) (string, error) {
	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("unmarshal chat response: %w", err)
	}
	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("empty ai response")
	}
	content := chatResp.Choices[0].Message.Content
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
	seenIDs := make(map[string]int, len(r.Animals))
	for i, a := range r.Animals {
		if a.Confidence < 0 || a.Confidence > 1 {
			return fmt.Errorf("animal[%d] confidence out of range", i)
		}
		bb := a.BoundingBox
		if err := ValidateBoundingBox(bb); err != nil {
			return fmt.Errorf("animal[%d] %w", i, err)
		}
		// 权威 taxonomy：规范化物种，禁止静默映射为鹅
		raw := a.Species
		if raw == "" {
			raw = a.Label
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
			normalized = append(normalized, a)
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

func clampValueResult(v *ValueResult) {
	if v.Rarity < 1 {
		v.Rarity = 1
	}
	if v.Rarity > 5 {
		v.Rarity = 5
	}
	v.HP = clampInt(v.HP, 10, 100)
	v.ATK = clampInt(v.ATK, 5, 50)
	v.DEF = clampInt(v.DEF, 5, 50)
	v.SPD = clampInt(v.SPD, 5, 50)
	if v.Class == "" {
		v.Class = "Ranger"
	}
	if v.Element == "" {
		v.Element = "Wind"
	}
	if len(v.Narrative) > 2000 {
		v.Narrative = v.Narrative[:2000]
	}
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
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
				Species:     "cat",
				TargetID:    "0",
				Confidence:  0.92,
				BoundingBox: BoundingBox{X: 0.15, Y: 0.2, Width: 0.35, Height: 0.45},
			},
		},
	}
	_ = validateDetectResult(r)
	return r
}

func mockAnalyze() *AnalysisResult {
	return &AnalysisResult{
		Breed:               "British Shorthair",
		Color:               "blue-gray",
		BodyType:            "sturdy",
		QualityScore:        8,
		SubjectCompleteness: 9,
		Clarity:             8,
		Lighting:            7,
		Composition:         8,
		Pose:                7,
		Angle:               9,
	}
}

func mockValue(input ValueInput) *ValueResult {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	classes := []string{"Warrior", "Mage", "Ranger", "Tank", "Support", "Assassin"}
	elements := []string{"Fire", "Water", "Grass", "Electric", "Ice", "Dark", "Light", "Earth", "Wind"}

	rarity := weightedRarity(rng)
	qualityAvg := (input.SubjectCompleteness + input.Clarity + input.Lighting + input.Composition + input.Pose + input.Angle) / 6
	if qualityAvg == 0 {
		qualityAvg = 5
	}

	return &ValueResult{
		Rarity:    rarity,
		HP:        10 + qualityAvg*5 + rng.Intn(21),
		ATK:       5 + rarity*5 + rng.Intn(11),
		DEF:       5 + qualityAvg/2 + rng.Intn(16),
		SPD:       5 + rng.Intn(46),
		Class:     classes[rng.Intn(len(classes))],
		Element:   elements[rng.Intn(len(elements))],
		Narrative: fmt.Sprintf("A %s %s discovered in the wild. Its %s coat gleams with potential. Trainers seek it for its unique battle style.", input.Breed, input.Species, input.Color),
	}
}

func weightedRarity(rng *rand.Rand) int {
	r := rng.Intn(100)
	switch {
	case r < 40:
		return 1
	case r < 70:
		return 2
	case r < 88:
		return 3
	case r < 97:
		return 4
	default:
		return 5
	}
}

// ScoreString 用于日志脱敏，不输出敏感内容。
func ScoreString(n int) string {
	return strconv.Itoa(n)
}
