// Package prompts MB3: AI 推理 Prompt 模板集中管理, 支持版本控制与 A/B 迭代。
package prompts

// DetectPrompt VLM 动物检测 Prompt —— 从图片中识别动物物种与边界框。
const DetectPrompt = `You are an animal detection system. Analyze this image and:

1. Detect all animals present in the image.
2. For each detected animal, provide:
   - species: the species name (e.g., "cat", "dog", "bird")
   - confidence: detection confidence score (0.0 to 1.0)
   - bounding_box: {x, y, width, height} as fractions of image dimensions (0.0 to 1.0)

Return ONLY a JSON array of detected animals. If no animals detected, return empty array [].
Example: [{"species": "cat", "confidence": 0.92, "bounding_box": {"x": 0.1, "y": 0.2, "width": 0.3, "height": 0.4}}]`

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
