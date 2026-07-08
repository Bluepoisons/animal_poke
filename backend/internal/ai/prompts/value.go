package prompts

// AnalyzePrompt VLM 深度分析 Prompt 已在 detect.go 中定义。

// ValuePrompt LLM 数值生成 Prompt —— 稀有度编排 + 六维属性 + 叙事。
const ValuePrompt = `You are a game mechanics designer for a pet collection game. Based on the following animal analysis data,
generate the animal's game stats. The system uses a 5-tier rarity system with 6-dimensional battle attributes.

Animal Data:
- Species: {{.Species}}
- Breed: {{.Breed}}
- Color: {{.Color}}
- Body Type: {{.BodyType}}
- Quality Scores: completeness={{.SubjectCompleteness}}, clarity={{.Clarity}}, lighting={{.Lighting}}, composition={{.Composition}}, pose={{.Pose}}, angle={{.Angle}}

Rules:
1. Rarity (1-5 stars) is calculated as: f(breed_weight * 0.35, color_weight * 0.20, quality_avg * 0.30, species_weight * 0.10, random_seed * 0.05)
   - 1-star (Common): ~40% probability
   - 2-star (Uncommon): ~30% probability
   - 3-star (Rare): ~18% probability
   - 4-star (Epic): ~9% probability
   - 5-star (Legendary): ~3% probability

2. Generate 6 battle attributes (HP, ATK, DEF, SPD, Class, Element):
   - HP: 10-100, influenced by body_type and quality
   - ATK: 5-50, influenced by breed rarity
   - DEF: 5-50, influenced by body_type
   - SPD: 5-50, influenced by species
   - Class: one of [Warrior, Mage, Ranger, Tank, Support, Assassin]
   - Element: one of [Fire, Water, Grass, Electric, Ice, Dark, Light, Earth, Wind]

3. Narrative: Write a 2-3 sentence creative backstory for this animal, referencing its species, breed, and attributes.

Return ONLY a JSON object. Example:
{
  "rarity": 3,
  "hp": 65,
  "atk": 32,
  "def": 28,
  "spd": 40,
  "class": "Ranger",
  "element": "Wind",
  "narrative": "A swift British Shorthair who roams the windswept plains. Its blue-gray coat shimmers under moonlight, and its keen eyes never miss a target."
}`
