package speciespack

// builtinPacks 在内容目录缺失时的最小回退（与 content/species 对齐）。
func builtinPacks() []*Pack {
	return []*Pack{
		{
			ID: "cat", Version: "1.0.0", ContentID: "species.cat", Status: StatusCapturable,
			Certification: &Certification{GoldenSetVersion: "1.0.0", ModelTrack: "detect"},
			Names: Names{
				Common: Localized{"zh-CN": "猫", "en": "Cat"}, Scientific: "Felis catus",
				Aliases: []string{"kitten", "feline"}, Contains: []string{"猫", "cat"},
				ContainsExclude: []string{"cattle", "caterpillar"},
			},
			Habitat:         Localized{"zh-CN": "城市与乡村人居环境", "en": "Urban and rural human habitats"},
			ObservationTips: Localized{"zh-CN": "保持距离，避免强闪光", "en": "Keep distance; avoid strong flash"},
			Welfare:         Welfare{Level: "companion"},
			Protection:      Protection{Status: "none"},
			Assets:          Assets{Emoji: "🐱", ThrowItemEmoji: "🥫"},
			Gameplay: Gameplay{
				ThrowItem: Localized{"zh-CN": "观察贴纸", "en": "Observe sticker"},
				CaptureMechanics: Localized{"zh-CN": "标准抛物线"}, ChargeRate: 2, OptimalRange: []float64{40, 80},
				ChargeSpeed: 1.15, DetectThreshold: 0.85,
				StatModifiers: &StatModifiers{HP: 0.8, ATK: 0.9, DEF: 0.9, SPD: 1.3, Crit: 10, Eva: 5},
				RarityWeights: []RarityWeight{
					{Tier: "common", Weight: 70}, {Tier: "uncommon", Weight: 25}, {Tier: "rare", Weight: 12},
					{Tier: "epic", Weight: 3}, {Tier: "legendary", Weight: 1},
				},
			},
		},
		{
			ID: "dog", Version: "1.0.0", ContentID: "species.dog", Status: StatusCapturable,
			Certification: &Certification{GoldenSetVersion: "1.0.0", ModelTrack: "detect"},
			Names: Names{
				Common: Localized{"zh-CN": "狗", "en": "Dog"}, Scientific: "Canis familiaris",
				Aliases: []string{"puppy", "canine"}, Contains: []string{"狗", "犬", "dog"},
			},
			Habitat:         Localized{"zh-CN": "社区、公园与人行道", "en": "Neighborhoods, parks, sidewalks"},
			ObservationTips: Localized{"zh-CN": "征得主人同意后再观察", "en": "Ask the owner before observing"},
			Welfare:         Welfare{Level: "companion"},
			Protection:      Protection{Status: "none"},
			Assets:          Assets{Emoji: "🐶", ThrowItemEmoji: "🦴"},
			Gameplay: Gameplay{
				ThrowItem: Localized{"zh-CN": "镜头信号", "en": "Lens signal"},
				CaptureMechanics: Localized{"zh-CN": "下落更快"}, ChargeRate: 1.5, OptimalRange: []float64{45, 85},
				ChargeSpeed: 1.0, DetectThreshold: 0.85,
				StatModifiers: &StatModifiers{HP: 1.3, ATK: 1.2, DEF: 1.0, SPD: 0.8, Crit: 3, Eva: 2},
				RarityWeights: []RarityWeight{
					{Tier: "common", Weight: 55}, {Tier: "uncommon", Weight: 30}, {Tier: "rare", Weight: 15},
					{Tier: "epic", Weight: 5}, {Tier: "legendary", Weight: 2},
				},
			},
		},
		{
			ID: "goose", Version: "1.0.0", ContentID: "species.goose", Status: StatusCapturable,
			Certification: &Certification{GoldenSetVersion: "1.0.0", ModelTrack: "detect"},
			Names: Names{
				Common: Localized{"zh-CN": "鹅", "en": "Goose"}, Scientific: "Anser sp.",
				Aliases: []string{"geese", "gander", "gosling"}, Contains: []string{"goose", "geese", "鹅"},
				ContainsExclude: []string{"mongoose"},
			},
			Habitat:         Localized{"zh-CN": "湖泊、公园水域", "en": "Lakes and park waterways"},
			ObservationTips: Localized{"zh-CN": "勿投喂面包，保持安全距离", "en": "Do not feed bread; keep a safe distance"},
			Welfare:         Welfare{Level: "wildlife"},
			Protection:      Protection{Status: "none"},
			Assets:          Assets{Emoji: "🪿", ThrowItemEmoji: "🍞"},
			Gameplay: Gameplay{
				ThrowItem: Localized{"zh-CN": "友好光点", "en": "Friendly spark"},
				CaptureMechanics: Localized{"zh-CN": "弹跳略强"}, ChargeRate: 2.5, OptimalRange: []float64{35, 75},
				ChargeSpeed: 0.9, DetectThreshold: 0.75,
				StatModifiers: &StatModifiers{HP: 1.0, ATK: 0.8, DEF: 1.4, SPD: 0.9, Crit: 2, Eva: 8},
				RarityWeights: []RarityWeight{
					{Tier: "common", Weight: 45}, {Tier: "uncommon", Weight: 35}, {Tier: "rare", Weight: 20},
					{Tier: "epic", Weight: 7}, {Tier: "legendary", Weight: 3},
				},
			},
		},
		// 第四物种试点：仅百科，未黄金集认证 → 不可捕获/发奖
		{
			ID: "rabbit", Version: "1.0.0", ContentID: "species.rabbit", Status: StatusCatalogOnly,
			Names: Names{
				Common: Localized{"zh-CN": "兔", "en": "Rabbit"}, Scientific: "Oryctolagus cuniculus",
				Aliases: []string{"bunny", "hare", "leveret"}, Contains: []string{"兔", "rabbit", "bunny"},
			},
			Habitat:         Localized{"zh-CN": "草地、灌丛与公园边缘", "en": "Grassland, scrub, park edges"},
			ObservationTips: Localized{"zh-CN": "黎明黄昏更易遇见，保持安静", "en": "More active at dawn/dusk; stay quiet"},
			Welfare:         Welfare{Level: "wildlife", Notes: Localized{"zh-CN": "野生个体请勿追逐抓捕"}},
			Protection:      Protection{Status: "none"},
			Assets:          Assets{Emoji: "🐰", Icon: "species/rabbit.png"},
			Gameplay: Gameplay{
				// 百科预留参数；因 catalog_only 不会用于捕获
				DetectThreshold: 0.85,
			},
		},
	}
}
