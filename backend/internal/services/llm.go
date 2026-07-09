// Package services MB3: 云端 LLM 编排(数值/叙事生成)。
package services

import (
	"bytes"
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
	Species            string
	Breed              string
	Color              string
	BodyType           string
	SubjectCompleteness int
	Clarity            int
	Lighting           int
	Composition        int
	Pose               int
	Angle              int
}

// GenerateValue 调用 LLM 生成稀有度/六维属性/叙事。
func (s *LLMService) GenerateValue(input ValueInput) (*ValueResult, error) {
	// dev mode: LLM Key 未配置时返回 mock
	if s.cfg.LLMKey == "" || s.cfg.LLMEndpoint == "" || s.cfg.LLMModel == "" {
		slog.Debug("LLM 未配置, 返回 mock 数值")
		return mockValue(input), nil
	}

	prompt := renderValuePrompt(input)
	return s.callLLM(prompt)
}

func (s *LLMService) callLLM(prompt string) (*ValueResult, error) {
	body := map[string]interface{}{
		"model": s.cfg.LLMModel,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"max_tokens": 512,
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
		return nil, fmt.Errorf("llm request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("llm returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var llmResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &llmResp); err != nil {
		return nil, fmt.Errorf("unmarshal llm response: %w", err)
	}
	if len(llmResp.Choices) == 0 {
		return nil, fmt.Errorf("empty llm response")
	}

	content := llmResp.Choices[0].Message.Content
	// 尝试提取 JSON 部分
	content = extractJSON(content)

	var result ValueResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("unmarshal value result: %w", err)
	}
	return &result, nil
}

func renderValuePrompt(input ValueInput) string {
	p := prompts.ValuePrompt
	p = strings.ReplaceAll(p, "{{.Species}}", input.Species)
	p = strings.ReplaceAll(p, "{{.Breed}}", input.Breed)
	p = strings.ReplaceAll(p, "{{.Color}}", input.Color)
	p = strings.ReplaceAll(p, "{{.BodyType}}", input.BodyType)
	return p
}

func extractJSON(s string) string {
	// LLM 有时会包裹 markdown 代码块, 尝试去掉
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

// mockValue 模拟数值生成(用于开发/测试)。
func mockValue(input ValueInput) *ValueResult {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	classes := []string{"Warrior", "Mage", "Ranger", "Tank", "Support", "Assassin"}
	elements := []string{"Fire", "Water", "Grass", "Electric", "Ice", "Dark", "Light", "Earth", "Wind"}

	// 稀有度加权随机: 1=40%, 2=30%, 3=18%, 4=9%, 5=3%
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
