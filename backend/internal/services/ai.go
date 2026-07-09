// Package services MB3: 统一 AI 编排(视觉检测 + 深度分析 + 数值/叙事生成)。
package services

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"animalpoke/backend/internal/ai/prompts"
)

// ---------- VLM 相关类型 ----------

// DetectBox 检测框。
type DetectBox struct {
	Species    string  `json:"species"`
	Confidence float64 `json:"confidence"`
	BoundingBox struct {
		X      float64 `json:"x"`
		Y      float64 `json:"y"`
		Width  float64 `json:"width"`
		Height float64 `json:"height"`
	} `json:"bounding_box"`
}

// DetectResult VLM 检测结果。
type DetectResult struct {
	Animals []DetectBox `json:"animals"`
}

// AnalysisResult 深度分析结果。
type AnalysisResult struct {
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
}

// ---------- LLM 相关类型 ----------

// ValueResult LLM 数值生成结果。
type ValueResult struct {
	Rarity    int    `json:"rarity"`    // 1-5 星级
	HP        int    `json:"hp"`        // 10-100
	ATK       int    `json:"atk"`       // 5-50
	DEF       int    `json:"def"`       // 5-50
	SPD       int    `json:"spd"`       // 5-50
	Class     string `json:"class"`     // Warrior/Mage/Ranger/Tank/Support/Assassin
	Element   string `json:"element"`   // Fire/Water/Grass/Electric/Ice/Dark/Light/Earth/Wind
	Narrative string `json:"narrative"` // 2-3 句叙事
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

// ---------- 视觉方法 ----------

// Detect 调用 AI 进行动物检测。imageData 为图片字节数据, 推理后不落盘。
func (s *AIService) Detect(imageData []byte, filename string) (*DetectResult, error) {
	if s.cfg.LLMKey == "" || s.cfg.LLMEndpoint == "" || s.cfg.LLMModel == "" {
		slog.Debug("AI 未配置, 返回 mock 检测结果")
		return mockDetect(), nil
	}

	body, err := s.callVision(imageData, filename, prompts.DetectPrompt)
	if err != nil {
		return nil, err
	}

	var result DetectResult
	jsonStr, err := s.parseChatResponse(body)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("json unmarshal: %w", err)
	}
	return &result, nil
}

// Analyze 调用 AI 进行动物深度分析。
func (s *AIService) Analyze(imageData []byte, filename string) (*AnalysisResult, error) {
	if s.cfg.LLMKey == "" || s.cfg.LLMEndpoint == "" || s.cfg.LLMModel == "" {
		slog.Debug("AI 未配置, 返回 mock 分析结果")
		return mockAnalyze(), nil
	}

	body, err := s.callVision(imageData, filename, prompts.AnalyzePrompt)
	if err != nil {
		return nil, err
	}

	var result AnalysisResult
	jsonStr, err := s.parseChatResponse(body)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("json unmarshal: %w", err)
	}
	return &result, nil
}

// ---------- 文本生成方法 ----------

// GenerateValue 调用 LLM 生成稀有度/六维属性/叙事。
func (s *AIService) GenerateValue(input ValueInput) (*ValueResult, error) {
	if s.cfg.LLMKey == "" || s.cfg.LLMEndpoint == "" || s.cfg.LLMModel == "" {
		slog.Debug("AI 未配置, 返回 mock 数值")
		return mockValue(input), nil
	}

	prompt := renderValuePrompt(input)
	body, err := s.callText(prompt)
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
	return &result, nil
}

// ---------- 统一 HTTP 调用 ----------

// callVision 发送图片 + 文本 prompt, 使用 OpenAI vision 格式(base64 图片)。
func (s *AIService) callVision(imageData []byte, filename, prompt string) ([]byte, error) {
	mimeType := http.DetectContentType(imageData)
	if !strings.HasPrefix(mimeType, "image/") {
		mimeType = "image/jpeg"
	}
	b64 := base64.StdEncoding.EncodeToString(imageData)
	dataURL := "data:" + mimeType + ";base64," + b64

	content := []map[string]interface{}{
		{"type": "image_url", "image_url": map[string]string{"url": dataURL}},
		{"type": "text", "text": prompt},
	}

	return s.call(content)
}

// callText 发送纯文本 prompt(OpenAI chat 格式)。
func (s *AIService) callText(prompt string) ([]byte, error) {
	return s.call(prompt)
}

// call 统一 HTTP 调用: 构建 OpenAI-compatible chat request, 发送并返回 body。
func (s *AIService) call(content interface{}) ([]byte, error) {
	body := map[string]interface{}{
		"model": s.cfg.LLMModel,
		"messages": []map[string]interface{}{
			{"role": "user", "content": content},
		},
		"max_tokens": 1024,
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal body: %w", err)
	}

	req, err := http.NewRequest("POST", s.cfg.LLMEndpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.cfg.LLMKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ai request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ai returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// parseChatResponse 解析 OpenAI chat completion 响应, 提取 content 文本并尝试剥离 markdown 代码块。
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

// ---------- Prompt 渲染 ----------

func renderValuePrompt(input ValueInput) string {
	p := prompts.ValuePrompt
	p = strings.ReplaceAll(p, "{{.Species}}", input.Species)
	p = strings.ReplaceAll(p, "{{.Breed}}", input.Breed)
	p = strings.ReplaceAll(p, "{{.Color}}", input.Color)
	p = strings.ReplaceAll(p, "{{.BodyType}}", input.BodyType)
	return p
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

// ---------- Mock 函数(开发/测试用) ----------

func mockDetect() *DetectResult {
	return &DetectResult{
		Animals: []DetectBox{
			{
				Species:    "cat",
				Confidence: 0.92,
				BoundingBox: struct {
					X      float64 `json:"x"`
					Y      float64 `json:"y"`
					Width  float64 `json:"width"`
					Height float64 `json:"height"`
				}{X: 0.15, Y: 0.2, Width: 0.35, Height: 0.45},
			},
		},
	}
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
