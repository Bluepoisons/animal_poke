package services

import (
	"testing"

	"animalpoke/backend/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestNewServices_NonNilAndCfgWired(t *testing.T) {
	cfg := &config.ThirdPartyConfig{
		TencentMapKey:    "tk",
		CaiyunWeatherKey: "ck",
		VisionEndpoint:   "ve",
		VisionKey:        "vk",
		VisionModel:      "vm",
		LLMEndpoint:      "le",
		LLMKey:           "lk",
		LLMModel:         "lm",
	}

	g := NewGeoService(cfg)
	w := NewWeatherService(cfg)
	v := NewVisionService(cfg)
	l := NewLLMService(cfg)
	a := NewAIService(cfg)

	assert.NotNil(t, g)
	assert.NotNil(t, w)
	assert.NotNil(t, v)
	assert.NotNil(t, l)
	assert.NotNil(t, a)

	// 验证 cfg 指针被正确注入(同一指针)
	assert.Same(t, cfg, g.cfg)
	assert.Same(t, cfg, w.cfg)
	assert.Same(t, cfg, v.cfg)
	assert.Same(t, cfg, l.cfg)
	assert.Same(t, cfg, a.cfg)
	assert.True(t, cfg.VisionConfigured())
	assert.True(t, cfg.LLMConfigured())
}
