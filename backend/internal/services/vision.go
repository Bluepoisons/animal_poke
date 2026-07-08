// Package services MB3: 云端 VLM 编排(检测 + 深度分析)。
package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"time"

	"animalpoke/backend/internal/ai/prompts"
)

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

// AnalysisResult VLM 深度分析��果。
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

// Detect 调用 VLM 进行动物检测。imageData 为图片字节数据, 推理后不落盘。
func (s *VisionService) Detect(imageData []byte, filename string) (*DetectResult, error) {
	if s.cfg.VLMKey == "" || s.cfg.VLMEndpoint == "" {
		slog.Debug("VLM 未配置, 返回 mock 检测结果")
		return mockDetect(), nil
	}

	body, err := s.callVLM(imageData, filename, prompts.DetectPrompt)
	if err != nil {
		return nil, err
	}
	var result DetectResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("json unmarshal: %w", err)
	}
	return &result, nil
}

// Analyze 调用 VLM 进行动物深度分析。
func (s *VisionService) Analyze(imageData []byte, filename string) (*AnalysisResult, error) {
	if s.cfg.VLMKey == "" || s.cfg.VLMEndpoint == "" {
		slog.Debug("VLM 未配置, 返回 mock 分析结果")
		return mockAnalyze(), nil
	}

	body, err := s.callVLM(imageData, filename, prompts.AnalyzePrompt)
	if err != nil {
		return nil, err
	}
	var result AnalysisResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("json unmarshal: %w", err)
	}
	return &result, nil
}

// callVLM 通用 VLM 调用逻辑: multipart form(图片) + text prompt, 返回响应体。
func (s *VisionService) callVLM(imageData []byte, filename, prompt string) ([]byte, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	// 图片 part
	part, err := w.CreateFormFile("image", filename)
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}
	if _, err := part.Write(imageData); err != nil {
		return nil, fmt.Errorf("write image data: %w", err)
	}

	// prompt part
	if err := w.WriteField("prompt", prompt); err != nil {
		return nil, fmt.Errorf("write prompt: %w", err)
	}
	w.Close()

	req, err := http.NewRequest("POST", s.cfg.VLMEndpoint, &buf)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+s.cfg.VLMKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vlm request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vlm returned status %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// mockDetect 模拟检测结果(用于开发/测试)。
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

// mockAnalyze 模拟分析结果(用于开发/测试)。
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
