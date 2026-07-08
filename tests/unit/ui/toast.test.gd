extends GdUnitTestSuite

# Toast 测试: 常量 + popup 无场景不崩溃 + 实例淡入淡出生命周期(queue_free)。
# Toast.popup 是静态方法, 依赖 Engine.get_main_loop().current_scene; 测试环境通常
# 无主场景 → push_warning 早返回(不崩溃)。实例淡入淡出可直接验证 _process。

const DEFAULT_DURATION := 2.0
const FADE_TIME := 0.25


func test_constants() -> void:
	assert_float(Toast.DEFAULT_DURATION).is_equal(2.0)
	assert_float(Toast.FADE_TIME).is_equal(0.25)


func test_popup_does_not_crash() -> void:
	# 无当前场景时仅 push_warning 并返回; 有场景时会创建 Toast(由其自身淡出回收)
	Toast.popup("test message")


func test_toast_fades_in_and_self_frees() -> void:
	var t := Toast.new()
	t._message = "hi"
	t._duration = 0.1
	add_child(t)
	# _ready 设 modulate.a=0; _process 淡入 → a 增加
	await await_millis(50)
	assert_bool(t.modulate.a > 0.0).is_true()
	# 等待 duration + 淡出完成 → queue_free
	await await_millis(500)
	assert_bool(is_instance_valid(t)).is_false()
