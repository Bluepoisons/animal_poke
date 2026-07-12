// Package prompts MB3: AI 推理 Prompt 模板集中管理, 支持版本控制与 A/B 迭代。
package prompts

// Prompt 版本号，写入 provenance。
const (
	DetectPromptVersion  = "detect-v3"
	AnalyzePromptVersion = "analyze-v1"
)

// DetectPrompt VLM 动物检测 Prompt —— 只判断拍摄照片中是否有可捕获动物。
// 标准响应固定为带 animals 字段的 JSON 对象（envelope）。
// 物种枚举严格限制为 cat|dog|goose；其它目标用 unsupported，无法判断用 unknown。禁止默认 goose。
const DetectPrompt = `You are an animal detection system for a capture game. Analyze this image and:

1. Decide whether the image contains a capturable animal.
2. If it does, return only the single most confident capturable animal:
   - species: MUST be one of: "cat", "dog", "goose", "unknown", "unsupported"
     * cat: domestic cats only
     * dog: domestic dogs only
     * goose: true geese only (NOT ducks, swans, or generic birds)
     * unsupported: ducks, swans, birds, humans, toys, screens, other non-target objects
     * unknown: cannot determine with confidence
   - label: original free-text label for audit only (e.g. "mallard", "person")
   - confidence: detection confidence score (0.0 to 1.0)

Return ONLY a JSON object with an "animals" array. If no capturable animal is present, return {"animals": []}.
Example: {"animals": [{"species": "cat", "label": "tabby cat", "confidence": 0.92}]}
Never map ducks/swans/birds/humans to goose.`

// AnalyzePrompt VLM 深度分析 Prompt —— 品种/毛色/体型/质量/角度评分。
const AnalyzePrompt = `You are an animal analysis expert. Analyze this animal image and provide:

1. breed: the specific breed/variety (e.g., "British Shorthair", "Golden Retriever")
2. color: primary coat color (e.g., "blue-gray", "golden", "black")
3. body_type: physical build (e.g., "sturdy", "slim", "medium")
4. quality_score: overall photo quality assessment (1-10) considering:
   - subject_completeness: (1-10) how complete the animal appears in frame
   - clarity: (1-10) image sharpness/focus
   - lighting: (1-10) exposure and lighting quality
   - composition: (1-10) framing and composition
   - pose: (1-10) animal pose quality
   - angle: (1-10) shooting angle quality

Return ONLY a JSON object. Example:
{"breed": "British Shorthair", "color": "blue-gray", "body_type": "sturdy", "quality_score": 8,
 "subject_completeness": 9, "clarity": 8, "lighting": 7, "composition": 8, "pose": 7, "angle": 9}`
