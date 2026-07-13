// Package prompts MB3: AI 推理 Prompt 模板集中管理, 支持版本控制与 A/B 迭代。
package prompts

// Prompt 版本号，写入 provenance。
const (
	DetectPromptVersion  = "detect-zh-v5"
	AnalyzePromptVersion = "analyze-zh-v2"
)

// DetectPrompt VLM 动物检测 Prompt —— 判断照片中的广谱常见动物。
// 标准响应固定为带 animals 字段的 JSON 对象（envelope）。
// 通用 bird 必须保持通用，禁止映射为鹅或其他具体鸟种。
const DetectPrompt = `You are an animal detection system for a capture game. Analyze this image and:

1. Decide whether the image contains a capturable animal.
2. If it does, return only the single most confident capturable animal:
   - species: MUST be one of the following stable IDs:
     * mammals: "cat", "dog", "rabbit", "horse", "cow", "sheep", "goat", "pig", "deer", "squirrel", "monkey", "bear", "elephant", "big_cat"
     * birds: "bird", "goose", "duck", "chicken", "pigeon", "parrot", "eagle"
     * reptiles: "turtle", "lizard", "snake", "crocodile"
     * amphibians: "frog", "salamander"
     * aquatic animals: "fish", "shark", "dolphin", "whale", "octopus"
     * invertebrates: "crab", "butterfly", "bee"
     * fallback animal category: "other_animal"
     * system values: "unknown", "unsupported"
     * bird: use only when the image shows a generic or unresolved bird. Never turn generic bird into goose, duck, eagle, or another specific bird.
     * goose, duck, chicken, pigeon, parrot, eagle: use only when that category is visually supported.
     * big_cat: lions, tigers, leopards, cheetahs, jaguars, or lynx; never domestic cats.
     * other_animal: use only when you are confident the subject is a real animal but it does not fit any listed category, such as a fox, hamster, giraffe, or zebra. The label MUST be the concrete Simplified Chinese animal name, such as "赤狐" or "仓鼠", never merely "其他动物".
     * unsupported: humans, toys, dolls, plush animals, statues, screens, phones, or other non-animal objects.
     * an animal outside the listed categories: other_animal when clearly identifiable as a real animal; unknown when uncertain. Never force it into a similar listed species.
     * unknown: cannot determine with confidence
   - label: concise Simplified Chinese label for audit only (e.g. "绿头鸭", "人")
   - confidence: detection confidence score (0.0 to 1.0)

Return ONLY a JSON object with an "animals" array. If no capturable animal is present, return {"animals": []}.
All label values MUST be Simplified Chinese. Example:
{"animals": [{"species": "cat", "label": "虎斑猫", "confidence": 0.92}]}
Never map generic birds to goose or another specific bird. Never map humans, toys, or screens to an animal.`

// AnalyzePrompt VLM 深度分析 Prompt —— 品种/毛色/体型/质量/角度评分。
const AnalyzePrompt = `You are an animal analysis expert for a Simplified Chinese product. Analyze this animal image and provide:

1. breed: use a concise Simplified Chinese breed/variety name (e.g., "英国短毛猫", "金毛寻回犬")
2. color: use a Simplified Chinese color description (e.g., "蓝灰色", "金色", "黑色")
3. body_type: use a Simplified Chinese build description (e.g., "敦实", "修长", "匀称")
4. quality_score: overall photo quality assessment (1-10) considering:
   - subject_completeness: (1-10) how complete the animal appears in frame
   - clarity: (1-10) image sharpness/focus
   - lighting: (1-10) exposure and lighting quality
   - composition: (1-10) framing and composition
   - pose: (1-10) animal pose quality
   - angle: (1-10) shooting angle quality

All descriptive string values MUST be Simplified Chinese. Return ONLY a JSON object. Example:
{"breed": "英国短毛猫", "color": "蓝灰色", "body_type": "敦实", "quality_score": 8,
 "subject_completeness": 9, "clarity": 8, "lighting": 7, "composition": 8, "pose": 7, "angle": 9}`
