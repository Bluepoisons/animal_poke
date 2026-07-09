package services

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"animalpoke/backend/internal/config"

	"github.com/stretchr/testify/assert"
)

func TestLLMService_GenerateValue_Mock(t *testing.T) {
	cfg := &config.ThirdPartyConfig{} // 空 Key
	svc := NewLLMService(cfg)

	input := ValueInput{
		Species:             "cat",
		Breed:               "British Shorthair",
		Color:               "blue-gray",
		BodyType:            "sturdy",
		SubjectCompleteness: 9,
		Clarity:             8,
		Lighting:            7,
		Composition:         8,
		Pose:                7,
		Angle:               9,
	}

	result, err := svc.GenerateValue(input)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, result.Rarity, 1)
	assert.LessOrEqual(t, result.Rarity, 5)
	assert.GreaterOrEqual(t, result.HP, 10)
	assert.LessOrEqual(t, result.HP, 100)
	assert.GreaterOrEqual(t, result.ATK, 5)
	assert.LessOrEqual(t, result.ATK, 50)
	assert.GreaterOrEqual(t, result.DEF, 5)
	assert.LessOrEqual(t, result.DEF, 50)
	assert.GreaterOrEqual(t, result.SPD, 5)
	assert.LessOrEqual(t, result.SPD, 50)
	assert.NotEmpty(t, result.Class)
	assert.NotEmpty(t, result.Element)
	assert.NotEmpty(t, result.Narrative)
}

func TestLLMService_GenerateValue_UsesConfiguredModel(t *testing.T) {
	var requestBody struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&requestBody))
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"rarity\":3,\"hp\":60,\"atk\":20,\"def\":18,\"spd\":22,\"class\":\"Ranger\",\"element\":\"Wind\",\"narrative\":\"swift and alert\"}"}}]}`))
	}))
	defer server.Close()

	cfg := &config.ThirdPartyConfig{
		LLMEndpoint: server.URL,
		LLMKey:      "test-key",
		LLMModel:    "qwen3.6-flash",
	}
	svc := NewLLMService(cfg)

	result, err := svc.GenerateValue(ValueInput{
		Species:             "cat",
		Breed:               "Tabby",
		Color:               "orange",
		BodyType:            "lean",
		SubjectCompleteness: 8,
		Clarity:             8,
		Lighting:            8,
		Composition:         8,
		Pose:                8,
		Angle:               8,
	})
	assert.NoError(t, err)
	assert.Equal(t, "qwen3.6-flash", requestBody.Model)
	assert.NotEmpty(t, requestBody.Messages)
	assert.Equal(t, 3, result.Rarity)
}

func TestWeightedRarity_Distribution(t *testing.T) {
	counts := map[int]int{}
	rng := &struct{}{}
	_ = rng

	// 使用固定种子验证分布
	for i := 0; i < 1000; i++ {
		mockResult := mockValue(ValueInput{
			Species:             "cat",
			Breed:               "test",
			Color:               "test",
			BodyType:            "test",
			SubjectCompleteness: 5,
			Clarity:             5,
			Lighting:            5,
			Composition:         5,
			Pose:                5,
			Angle:               5,
		})
		counts[mockResult.Rarity]++
	}

	// 验证各稀有度都有产出(宽松: 至少都有 1 次)
	for r := 1; r <= 5; r++ {
		assert.Greater(t, counts[r], 0, "rarity %d should appear at least once", r)
	}
	// 低稀有度应多于高稀有度
	assert.Greater(t, counts[1], counts[5], "common should be more than legendary")
}

func TestMockValue_RangeCheck(t *testing.T) {
	validClasses := map[string]bool{
		"Warrior": true, "Mage": true, "Ranger": true, "Tank": true, "Support": true, "Assassin": true,
	}
	validElements := map[string]bool{
		"Fire": true, "Water": true, "Grass": true, "Electric": true, "Ice": true,
		"Dark": true, "Light": true, "Earth": true, "Wind": true,
	}

	for i := 0; i < 50; i++ {
		result := mockValue(ValueInput{
			Species:             "cat",
			SubjectCompleteness: 5,
			Clarity:             5,
			Lighting:            5,
			Composition:         5,
			Pose:                5,
			Angle:               5,
		})
		assert.True(t, validClasses[result.Class], "invalid class: %s", result.Class)
		assert.True(t, validElements[result.Element], "invalid element: %s", result.Element)
	}
}

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain", `{"key": "value"}`, `{"key": "value"}`},
		{"markdown_json", "```json\n{\"key\": \"value\"}\n```", `{"key": "value"}`},
		{"markdown_no_lang", "```\n{\"key\": \"value\"}\n```", `{"key": "value"}`},
		{"whitespace", "  {\"key\": \"value\"}  ", `{"key": "value"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, extractJSON(tt.input))
		})
	}
}
