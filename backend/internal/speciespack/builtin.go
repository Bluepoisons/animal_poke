package speciespack

// builtinPacks 在内容目录缺失时的最小回退（与 content/species 对齐）。
func builtinPacks() []*Pack {
	packs := []*Pack{
		{
			ID: "cat", Version: "1.0.0", ContentID: "species.cat", Status: StatusCapturable,
			Certification: &Certification{GoldenSetVersion: "1.0.0", ModelTrack: "detect"},
			Names: Names{
				Common: Localized{"zh-CN": "猫", "en": "Cat"}, Scientific: "Felis catus",
				Aliases: []string{"kitten", "feline", "小猫", "猫咪", "英短猫"}, Contains: []string{"小猫", "猫咪", "英短猫", "猫", "cat"},
				ContainsExclude: []string{"cattle", "caterpillar"},
			},
			Habitat:         Localized{"zh-CN": "城市与乡村人居环境", "en": "Urban and rural human habitats"},
			ObservationTips: Localized{"zh-CN": "保持距离，避免强闪光", "en": "Keep distance; avoid strong flash"},
			Welfare:         Welfare{Level: "companion"},
			Protection:      Protection{Status: "none"},
			Assets:          Assets{Emoji: "🐱", ThrowItemEmoji: "🥫"},
			Gameplay: Gameplay{
				ThrowItem:        Localized{"zh-CN": "观察贴纸", "en": "Observe sticker"},
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
				Aliases: []string{"puppy", "canine", "小狗", "狗狗", "犬"}, Contains: []string{"小狗", "狗狗", "狗", "犬", "dog"},
			},
			Habitat:         Localized{"zh-CN": "社区、公园与人行道", "en": "Neighborhoods, parks, sidewalks"},
			ObservationTips: Localized{"zh-CN": "征得主人同意后再观察", "en": "Ask the owner before observing"},
			Welfare:         Welfare{Level: "companion"},
			Protection:      Protection{Status: "none"},
			Assets:          Assets{Emoji: "🐶", ThrowItemEmoji: "🦴"},
			Gameplay: Gameplay{
				ThrowItem:        Localized{"zh-CN": "镜头信号", "en": "Lens signal"},
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
				Aliases: []string{"geese", "gander", "gosling", "大鹅"}, Contains: []string{"大鹅", "goose", "geese", "鹅"},
				ContainsExclude: []string{"mongoose"},
			},
			Habitat:         Localized{"zh-CN": "湖泊、公园水域", "en": "Lakes and park waterways"},
			ObservationTips: Localized{"zh-CN": "勿投喂面包，保持安全距离", "en": "Do not feed bread; keep a safe distance"},
			Welfare:         Welfare{Level: "wildlife"},
			Protection:      Protection{Status: "none"},
			Assets:          Assets{Emoji: "🪿", ThrowItemEmoji: "🍞"},
			Gameplay: Gameplay{
				ThrowItem:        Localized{"zh-CN": "友好光点", "en": "Friendly spark"},
				CaptureMechanics: Localized{"zh-CN": "弹跳略强"}, ChargeRate: 2.5, OptimalRange: []float64{35, 75},
				ChargeSpeed: 0.9, DetectThreshold: 0.75,
				StatModifiers: &StatModifiers{HP: 1.0, ATK: 0.8, DEF: 1.4, SPD: 0.9, Crit: 2, Eva: 8},
				RarityWeights: []RarityWeight{
					{Tier: "common", Weight: 45}, {Tier: "uncommon", Weight: 35}, {Tier: "rare", Weight: 20},
					{Tier: "epic", Weight: 7}, {Tier: "legendary", Weight: 3},
				},
			},
		},
		{
			ID: "rabbit", Version: "1.0.0", ContentID: "species.rabbit", Status: StatusCapturable,
			Certification: &Certification{GoldenSetVersion: "1.0.0", ModelTrack: "detect"},
			Names: Names{
				Common: Localized{"zh-CN": "兔", "en": "Rabbit"}, Scientific: "Oryctolagus cuniculus",
				Aliases: []string{"bunny", "hare", "leveret", "野兔"}, Contains: []string{"野兔", "兔", "rabbit", "bunny"},
			},
			Habitat:         Localized{"zh-CN": "草地、灌丛与公园边缘", "en": "Grassland, scrub, park edges"},
			ObservationTips: Localized{"zh-CN": "黎明黄昏更易遇见，保持安静", "en": "More active at dawn/dusk; stay quiet"},
			Welfare:         Welfare{Level: "wildlife", Notes: Localized{"zh-CN": "野生个体请勿追逐抓捕"}},
			Protection:      Protection{Status: "none"},
			Assets:          Assets{Emoji: "🐰", Icon: "species/rabbit.png"},
			Gameplay: Gameplay{
				ThrowItem:        Localized{"zh-CN": "远距观察标记", "en": "Remote observation marker"},
				CaptureMechanics: Localized{"zh-CN": "镜头确认，不追逐、不接触动物", "en": "Confirm by camera without chasing or touching"},
				DetectThreshold:  0.8,
				StatModifiers:    &StatModifiers{HP: 0.8, ATK: 0.7, DEF: 0.8, SPD: 1.3, Crit: 5, Eva: 8},
				RarityWeights: []RarityWeight{
					{Tier: "common", Weight: 65}, {Tier: "uncommon", Weight: 25}, {Tier: "rare", Weight: 10},
				},
			},
		},
	}
	return append(packs, broadBuiltinPacks()...)
}

type broadSpeciesDef struct {
	id, zh, en, scientific, emoji string
	habitatZH, habitatEN          string
	welfare, protection           string
	aliases, contains             []string
}

func broadBuiltinPacks() []*Pack {
	defs := []broadSpeciesDef{
		{id: "other_animal", zh: "其他动物", en: "Other animal", scientific: "Animalia", emoji: "🐾", habitatZH: "多种自然或人居环境", habitatEN: "Varied natural and human habitats", welfare: "unknown", protection: "unknown", contains: []string{"其他动物"}},
		{id: "horse", zh: "马", en: "Horse", scientific: "Equus caballus", emoji: "🐴", habitatZH: "牧场、农场与草原", habitatEN: "Pastures, farms, and grasslands", welfare: "livestock", protection: "none", aliases: []string{"pony", "foal", "stallion", "mare"}, contains: []string{"马", "horse", "pony"}},
		{id: "cow", zh: "牛", en: "Cattle", scientific: "Bos taurus", emoji: "🐄", habitatZH: "农场与牧场", habitatEN: "Farms and pastures", welfare: "livestock", protection: "none", aliases: []string{"cattle", "bull", "calf", "ox"}, contains: []string{"牛", "cow", "cattle"}},
		{id: "sheep", zh: "羊", en: "Sheep", scientific: "Ovis aries", emoji: "🐑", habitatZH: "牧场与丘陵草地", habitatEN: "Pastures and hill grasslands", welfare: "livestock", protection: "none", aliases: []string{"lamb", "ewe", "ram"}, contains: []string{"绵羊", "sheep", "lamb"}},
		{id: "goat", zh: "山羊", en: "Goat", scientific: "Capra hircus", emoji: "🐐", habitatZH: "山地、牧场与农场", habitatEN: "Mountains, pastures, and farms", welfare: "livestock", protection: "none", aliases: []string{"goat kid", "billy goat"}, contains: []string{"山羊", "goat"}},
		{id: "pig", zh: "猪", en: "Pig", scientific: "Sus scrofa domesticus", emoji: "🐷", habitatZH: "农场与林缘", habitatEN: "Farms and woodland edges", welfare: "livestock", protection: "none", aliases: []string{"piglet", "hog", "boar", "swine"}, contains: []string{"猪", "pig", "boar"}},
		{id: "deer", zh: "鹿", en: "Deer", scientific: "Cervidae", emoji: "🦌", habitatZH: "森林、草地与湿地边缘", habitatEN: "Forests, grasslands, and wetland edges", welfare: "wildlife", protection: "none", aliases: []string{"fawn", "stag", "doe", "reindeer"}, contains: []string{"鹿", "deer", "reindeer"}},
		{id: "squirrel", zh: "松鼠", en: "Squirrel", scientific: "Sciuridae", emoji: "🐿️", habitatZH: "林地、公园与城市绿地", habitatEN: "Woodlands, parks, and urban green spaces", welfare: "wildlife", protection: "none", aliases: []string{"chipmunk"}, contains: []string{"松鼠", "squirrel", "chipmunk"}},
		{id: "monkey", zh: "猴", en: "Monkey", scientific: "Simiiformes", emoji: "🐒", habitatZH: "森林、山地与保护区", habitatEN: "Forests, mountains, and reserves", welfare: "wildlife", protection: "protected", aliases: []string{"ape", "macaque", "baboon", "gibbon"}, contains: []string{"猴", "猿", "monkey", "macaque"}},
		{id: "bear", zh: "熊", en: "Bear", scientific: "Ursidae", emoji: "🐻", habitatZH: "森林、山地与极地", habitatEN: "Forests, mountains, and polar regions", welfare: "wildlife", protection: "protected", aliases: []string{"panda", "polar bear", "brown bear", "black bear"}, contains: []string{"熊", "熊猫", "bear", "panda"}},
		{id: "elephant", zh: "大象", en: "Elephant", scientific: "Elephantidae", emoji: "🐘", habitatZH: "草原、森林与保护区", habitatEN: "Savannas, forests, and reserves", welfare: "wildlife", protection: "endangered", aliases: []string{"elephant calf"}, contains: []string{"大象", "象", "elephant"}},
		{id: "big_cat", zh: "大型猫科动物", en: "Big cat", scientific: "Pantherinae", emoji: "🐯", habitatZH: "森林、草原、山地与保护区", habitatEN: "Forests, grasslands, mountains, and reserves", welfare: "wildlife", protection: "endangered", aliases: []string{"lion", "tiger", "leopard", "cheetah", "jaguar", "lynx"}, contains: []string{"狮子", "老虎", "豹子", "猎豹", "lion", "tiger", "leopard", "cheetah"}},
		{id: "bird", zh: "鸟", en: "Bird", scientific: "Aves", emoji: "🐦", habitatZH: "林地、湿地、草原与城市绿地", habitatEN: "Woodlands, wetlands, grasslands, and urban green spaces", welfare: "wildlife", protection: "unknown", aliases: []string{"avian", "songbird", "sparrow", "crow", "magpie", "swan", "owl", "turkey", "peacock", "鸟类", "天鹅", "猫头鹰", "麻雀", "乌鸦", "喜鹊", "孔雀"}, contains: []string{"鸟类", "蜂鸟", "鸟", "bird", "avian"}},
		{id: "duck", zh: "鸭", en: "Duck", scientific: "Anatidae", emoji: "🦆", habitatZH: "湖泊、河流、湿地与农场", habitatEN: "Lakes, rivers, wetlands, and farms", welfare: "wildlife", protection: "none", aliases: []string{"duckling", "mallard", "drake", "鸭子"}, contains: []string{"鸭子", "鸭", "duck", "mallard"}},
		{id: "chicken", zh: "鸡", en: "Chicken", scientific: "Gallus gallus domesticus", emoji: "🐔", habitatZH: "农场与乡村环境", habitatEN: "Farms and rural habitats", welfare: "livestock", protection: "none", aliases: []string{"hen", "rooster", "chick"}, contains: []string{"鸡", "chicken", "rooster"}},
		{id: "pigeon", zh: "鸽子", en: "Pigeon", scientific: "Columbidae", emoji: "🕊️", habitatZH: "城市、广场、山崖与林地", habitatEN: "Cities, plazas, cliffs, and woodlands", welfare: "wildlife", protection: "none", aliases: []string{"dove", "squab"}, contains: []string{"鸽", "pigeon", "dove"}},
		{id: "parrot", zh: "鹦鹉", en: "Parrot", scientific: "Psittaciformes", emoji: "🦜", habitatZH: "热带林地与人居环境", habitatEN: "Tropical forests and human habitats", welfare: "wildlife", protection: "protected", aliases: []string{"parakeet", "budgie", "cockatoo", "macaw"}, contains: []string{"鹦鹉", "parrot", "parakeet"}},
		{id: "eagle", zh: "鹰", en: "Eagle", scientific: "Accipitridae", emoji: "🦅", habitatZH: "山地、森林、草原与海岸", habitatEN: "Mountains, forests, grasslands, and coasts", welfare: "wildlife", protection: "protected", aliases: []string{"hawk", "falcon", "raptor"}, contains: []string{"鹰", "隼", "雕", "eagle", "hawk", "falcon"}},
		{id: "turtle", zh: "龟", en: "Turtle", scientific: "Testudines", emoji: "🐢", habitatZH: "河湖、湿地、海洋与陆地", habitatEN: "Rivers, wetlands, oceans, and land", welfare: "wildlife", protection: "protected", aliases: []string{"tortoise", "terrapin", "sea turtle"}, contains: []string{"龟", "鳖", "turtle", "tortoise"}},
		{id: "lizard", zh: "蜥蜴", en: "Lizard", scientific: "Lacertilia", emoji: "🦎", habitatZH: "荒漠、森林、草地与人居边缘", habitatEN: "Deserts, forests, grasslands, and settlement edges", welfare: "wildlife", protection: "unknown", aliases: []string{"gecko", "iguana", "chameleon"}, contains: []string{"蜥蜴", "壁虎", "lizard", "gecko"}},
		{id: "snake", zh: "蛇", en: "Snake", scientific: "Serpentes", emoji: "🐍", habitatZH: "森林、草地、湿地与荒漠", habitatEN: "Forests, grasslands, wetlands, and deserts", welfare: "wildlife", protection: "unknown", aliases: []string{"python", "cobra", "viper"}, contains: []string{"蛇", "snake", "python", "cobra"}},
		{id: "crocodile", zh: "鳄鱼", en: "Crocodile", scientific: "Crocodylia", emoji: "🐊", habitatZH: "河流、湖泊、沼泽与河口", habitatEN: "Rivers, lakes, swamps, and estuaries", welfare: "wildlife", protection: "protected", aliases: []string{"alligator", "caiman", "croc"}, contains: []string{"鳄", "crocodile", "alligator"}},
		{id: "frog", zh: "青蛙", en: "Frog", scientific: "Anura", emoji: "🐸", habitatZH: "池塘、溪流、湿地与林地", habitatEN: "Ponds, streams, wetlands, and forests", welfare: "wildlife", protection: "unknown", aliases: []string{"toad", "tadpole", "bullfrog", "牛蛙"}, contains: []string{"青蛙", "牛蛙", "蟾蜍", "蛙", "frog", "bullfrog", "toad"}},
		{id: "salamander", zh: "蝾螈", en: "Salamander", scientific: "Urodela", emoji: "🦎", habitatZH: "湿润林地、溪流与洞穴", habitatEN: "Moist forests, streams, and caves", welfare: "wildlife", protection: "protected", aliases: []string{"newt", "axolotl"}, contains: []string{"蝾螈", "娃娃鱼", "salamander", "newt", "axolotl"}},
		{id: "fish", zh: "鱼", en: "Fish", scientific: "Actinopterygii", emoji: "🐟", habitatZH: "河湖、湿地与海洋", habitatEN: "Rivers, lakes, wetlands, and oceans", welfare: "wildlife", protection: "unknown", aliases: []string{"goldfish", "koi", "salmon", "tuna", "seahorse", "piranha", "海马", "食人鱼"}, contains: []string{"海马", "食人鱼", "鱼", "fish", "goldfish", "koi", "seahorse", "piranha"}},
		{id: "shark", zh: "鲨鱼", en: "Shark", scientific: "Selachimorpha", emoji: "🦈", habitatZH: "近岸与远洋海域", habitatEN: "Coastal and open ocean waters", welfare: "wildlife", protection: "protected", aliases: []string{"shark pup"}, contains: []string{"鲨", "shark"}},
		{id: "dolphin", zh: "海豚", en: "Dolphin", scientific: "Delphinidae", emoji: "🐬", habitatZH: "近岸、海湾与远洋", habitatEN: "Coasts, bays, and open oceans", welfare: "wildlife", protection: "protected", aliases: []string{"porpoise"}, contains: []string{"海豚", "江豚", "dolphin", "porpoise"}},
		{id: "whale", zh: "鲸", en: "Whale", scientific: "Cetacea", emoji: "🐋", habitatZH: "海洋与部分大型河流", habitatEN: "Oceans and some large rivers", welfare: "wildlife", protection: "endangered", aliases: []string{"orca", "killer whale"}, contains: []string{"鲸", "whale", "orca"}},
		{id: "octopus", zh: "章鱼", en: "Octopus", scientific: "Octopoda", emoji: "🐙", habitatZH: "礁石、海床与近岸海域", habitatEN: "Reefs, seafloors, and coastal waters", welfare: "wildlife", protection: "unknown", aliases: []string{"octopi"}, contains: []string{"章鱼", "八爪鱼", "octopus"}},
		{id: "crab", zh: "螃蟹", en: "Crab", scientific: "Brachyura", emoji: "🦀", habitatZH: "海岸、滩涂、河口与淡水", habitatEN: "Coasts, mudflats, estuaries, and fresh water", welfare: "wildlife", protection: "unknown", aliases: []string{"hermit crab"}, contains: []string{"螃蟹", "蟹", "crab"}},
		{id: "butterfly", zh: "蝴蝶", en: "Butterfly", scientific: "Rhopalocera", emoji: "🦋", habitatZH: "花园、草地与林缘", habitatEN: "Gardens, grasslands, and woodland edges", welfare: "wildlife", protection: "unknown", aliases: []string{"moth", "caterpillar"}, contains: []string{"蝴蝶", "蛾", "butterfly", "moth"}},
		{id: "bee", zh: "蜜蜂", en: "Bee", scientific: "Anthophila", emoji: "🐝", habitatZH: "花园、农田、草地与林缘", habitatEN: "Gardens, farmland, grasslands, and woodland edges", welfare: "wildlife", protection: "unknown", aliases: []string{"bumblebee", "honeybee"}, contains: []string{"蜜蜂", "bee", "bumblebee"}},
	}

	packs := make([]*Pack, 0, len(defs))
	for _, def := range defs {
		tipZH := "保持距离，用镜头安静观察；不追逐、不投喂、不触碰"
		tipEN := "Keep distance and observe quietly by camera; do not chase, feed, or touch"
		if def.protection == "protected" || def.protection == "endangered" {
			tipZH = "仅可远距离观察和拍摄；不靠近、不追逐、不投喂、不触碰"
			tipEN = "Remote observation and photography only; never approach, chase, feed, or touch"
		}
		pack := &Pack{
			ID: def.id, Version: "1.0.0", ContentID: "species." + def.id, Status: StatusCapturable,
			Certification: &Certification{GoldenSetVersion: "1.0.0", ModelTrack: "detect"},
			Names: Names{
				Common: Localized{"zh-CN": def.zh, "en": def.en}, Scientific: def.scientific,
				Aliases: def.aliases, Contains: def.contains,
			},
			Habitat:         Localized{"zh-CN": def.habitatZH, "en": def.habitatEN},
			ObservationTips: Localized{"zh-CN": tipZH, "en": tipEN},
			Welfare:         Welfare{Level: def.welfare},
			Protection:      Protection{Status: def.protection},
			Assets:          Assets{Emoji: def.emoji, Icon: "species/" + def.id + ".png", PlaceholderTone: "natural"},
			Gameplay: Gameplay{
				ThrowItem:        Localized{"zh-CN": "远距观察标记", "en": "Remote observation marker"},
				CaptureMechanics: Localized{"zh-CN": "镜头确认，不接触动物", "en": "Confirm by camera without animal contact"},
				ChargeRate:       1, OptimalRange: []float64{45, 85}, ChargeSpeed: 1,
				DetectThreshold: 0.72,
				StatModifiers:   &StatModifiers{HP: 1, ATK: 1, DEF: 1, SPD: 1, Crit: 3, Eva: 3},
				RarityWeights: []RarityWeight{
					{Tier: "common", Weight: 65}, {Tier: "uncommon", Weight: 25}, {Tier: "rare", Weight: 10},
				},
			},
			I18n: map[string]Localized{
				"zh-CN": {"blurb": "可进入动物记录与中文幻想探险"},
				"en":    {"blurb": "Available for observation records and fictional adventures"},
			},
		}
		if def.protection == "protected" || def.protection == "endangered" {
			pack.Protection.Notes = Localized{
				"zh-CN": "保护动物仅限远距离观察，不得诱导接近或干扰",
				"en":    "Protected wildlife is for remote observation only; never encourage approach or disturbance",
			}
			pack.I18n["zh-CN"]["blurb"] = "保护动物：仅限远距离观察记录与幻想探险"
		}
		packs = append(packs, pack)
	}
	return packs
}
