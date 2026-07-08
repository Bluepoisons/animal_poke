extends GdUnitTestSuite

# Rarity 纯逻辑测试: 颜色/名称/掉率/粒子, 无依赖。

func test_color_of_each_tier() -> void:
	var expected_colors: Array[Color] = [
		Rarity.COLOR_COMMON, Rarity.COLOR_UNCOMMON, Rarity.COLOR_RARE, Rarity.COLOR_EPIC, Rarity.COLOR_LEGENDARY,
	]
	for tier in [Rarity.Tier.COMMON, Rarity.Tier.UNCOMMON, Rarity.Tier.RARE, Rarity.Tier.EPIC, Rarity.Tier.LEGENDARY]:
		assert_bool(Rarity.color_of(tier).is_equal_approx(expected_colors[tier])).is_true()


func test_color_of_unknown_falls_back_to_common() -> void:
	# 越界值回退灰色(COMMON), 并触发 push_warning
	assert_bool(Rarity.color_of(99).is_equal_approx(Rarity.COLOR_COMMON)).is_true()
	assert_bool(Rarity.color_of(-1).is_equal_approx(Rarity.COLOR_COMMON)).is_true()


func test_name_of_each_tier() -> void:
	assert_str(Rarity.name_of(Rarity.Tier.COMMON)).is_equal("普通")
	assert_str(Rarity.name_of(Rarity.Tier.UNCOMMON)).is_equal("非凡")
	assert_str(Rarity.name_of(Rarity.Tier.RARE)).is_equal("稀有")
	assert_str(Rarity.name_of(Rarity.Tier.EPIC)).is_equal("史诗")
	assert_str(Rarity.name_of(Rarity.Tier.LEGENDARY)).is_equal("传说")


func test_name_of_unknown() -> void:
	assert_str(Rarity.name_of(99)).is_equal("未知")


func test_rate_of_each_tier() -> void:
	assert_float(Rarity.rate_of(Rarity.Tier.COMMON)).is_equal(0.60)
	assert_float(Rarity.rate_of(Rarity.Tier.UNCOMMON)).is_equal(0.25)
	assert_float(Rarity.rate_of(Rarity.Tier.RARE)).is_equal(0.10)
	assert_float(Rarity.rate_of(Rarity.Tier.EPIC)).is_equal(0.04)
	assert_float(Rarity.rate_of(Rarity.Tier.LEGENDARY)).is_equal(0.01)


func test_rate_of_unknown_is_zero() -> void:
	assert_float(Rarity.rate_of(99)).is_equal(0.0)


func test_rates_sum_to_one() -> void:
	var sum: float = 0.0
	for tier in [Rarity.Tier.COMMON, Rarity.Tier.UNCOMMON, Rarity.Tier.RARE, Rarity.Tier.EPIC, Rarity.Tier.LEGENDARY]:
		sum += Rarity.rate_of(tier)
	assert_bool(is_equal_approx(sum, 1.0)).is_true()


func test_has_particle_only_legendary() -> void:
	assert_bool(Rarity.has_particle(Rarity.Tier.COMMON)).is_false()
	assert_bool(Rarity.has_particle(Rarity.Tier.UNCOMMON)).is_false()
	assert_bool(Rarity.has_particle(Rarity.Tier.RARE)).is_false()
	assert_bool(Rarity.has_particle(Rarity.Tier.EPIC)).is_false()
	assert_bool(Rarity.has_particle(Rarity.Tier.LEGENDARY)).is_true()
	assert_bool(Rarity.has_particle(99)).is_false()
