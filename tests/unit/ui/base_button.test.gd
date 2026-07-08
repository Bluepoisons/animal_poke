extends GdUnitTestSuite

# AppButton 测试: clicked 信号防抖(0.2s 内重复点击只发一次, 解锁后可再触发)。

func _new_button() -> AppButton:
	var btn: AppButton = auto_free(AppButton.new())
	add_child(btn)
	return btn


func test_single_press_emits_clicked_once() -> void:
	var btn := _new_button()
	var count := [0]
	btn.clicked.connect(func() -> void: count[0] += 1)
	btn._on_pressed()
	assert_int(count[0]).is_equal(1)


func test_rapid_double_press_is_debounced() -> void:
	var btn := _new_button()
	var count := [0]
	btn.clicked.connect(func() -> void: count[0] += 1)
	btn._on_pressed()
	btn._on_pressed()  # 0.2s 内, 已锁, 不发
	assert_int(count[0]).is_equal(1)


func test_unlock_after_lock_time_allows_again() -> void:
	var btn := _new_button()
	var count := [0]
	btn.clicked.connect(func() -> void: count[0] += 1)
	btn._on_pressed()
	await await_millis(250)  # > CLICK_LOCK_TIME(0.2s), 解锁
	btn._on_pressed()
	assert_int(count[0]).is_equal(2)


func test_unlock_uses_real_timer() -> void:
	# 验证 _unlock 把 _click_locked 置回 false
	var btn := _new_button()
	btn._on_pressed()
	assert_bool(btn._click_locked).is_true()
	await await_millis(250)
	assert_bool(btn._click_locked).is_false()
