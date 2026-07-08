extends GdUnitTestSuite

# LoadingIndicator 测试: set_text + custom_minimum_size + _angle 随 _process 递增。

func _new_indicator() -> LoadingIndicator:
	var li: LoadingIndicator = auto_free(LoadingIndicator.new())
	add_child(li)
	return li


func test_custom_minimum_size() -> void:
	var li := _new_indicator()
	assert_vector(li.custom_minimum_size).is_equal(Vector2(80, 80))


func test_set_text() -> void:
	var li := _new_indicator()
	li.set_text("加载中")
	assert_str(li._label.text).is_equal("加载中")


func test_angle_advances_with_processing() -> void:
	var li := _new_indicator()
	var before: float = li._angle
	# _process 由 SceneTree 驱动, 等若干帧
	await await_millis(60)
	assert_bool(li._angle > before).is_true()
