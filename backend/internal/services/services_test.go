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
		VLMEndpoint:      "ve",
		VLMKey:           "vk",
		LLMEndpoint:      "le",
		LLMKey:           "lk",
	}

	g := NewGeoService(cfg)
	w := NewWeatherService(cfg)
	v := NewVisionService(cfg)
	l := NewLLMService(cfg)

	assert.NotNil(t, g)
	assert.NotNil(t, w)
	assert.NotNil(t, v)
	assert.NotNil(t, l)

	// 验证 cfg 指针被正确注入(同一指针)
	assert.Same(t, cfg, g.cfg)
	assert.Same(t, cfg, w.cfg)
	assert.Same(t, cfg, v.cfg)
	assert.Same(t, cfg, l.cfg)
}
