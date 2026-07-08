extends GdUnitTestSuite

# RarityBorder 测试: set/get rarity + 边框色与 Rarity.color_of 一致。

func _new_border() -> RarityBorder:
	var b: RarityBorder = auto_free(RarityBorder.new())
	add_child(b)
	return b


func test_set_and_get_rarity() -> void:
	var b := _new_border()
	b.set_rarity(Rarity.Tier.RARE)
	assert_int(b.get_rarity()).is_equal(Rarity.Tier.RARE)
	b.set_rarity(Rarity.Tier.LEGENDARY)
	assert_int(b.get_rarity()).is_equal(Rarity.Tier.LEGENDARY)


func test_border_color_matches_tier() -> void:
	for tier in [Rarity.Tier.COMMON, Rarity.Tier.UNCOMMON, Rarity.Tier.RARE, Rarity.Tier.EPIC, Rarity.Tier.LEGENDARY]:
		var b := _new_border()
		b.set_rarity(tier)
		var sb: StyleBox = b.get_theme_stylebox("panel")
		assert_bool(sb != null).is_true()
		var flat := sb as StyleBoxFlat
		assert_bool(flat != null).is_true()
		assert_bool(flat.border_color.is_equal_approx(Rarity.color_of(tier))).is_true()


func test_initial_border_applied_on_ready() -> void:
	# _ready 用默认 _tier(0=COMMON) 应用边框
	var b := _new_border()
	var sb: StyleBox = b.get_theme_stylebox("panel")
	assert_bool(sb != null).is_true()
	var flat := sb as StyleBoxFlat
	assert_bool(flat.border_color.is_equal_approx(Rarity.COLOR_COMMON)).is_true()
