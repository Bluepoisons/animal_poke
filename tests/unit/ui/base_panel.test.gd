extends GdUnitTestSuite

# BasePanel 测试: 标题 Label 懒初始化 + 重复 set_title 复用同一 Label。

func _new_panel() -> BasePanel:
	var p: BasePanel = auto_free(BasePanel.new())
	add_child(p)
	return p


func test_title_label_initially_null() -> void:
	var p := _new_panel()
	assert_object(p.get_title_label()).is_null()


func test_set_title_creates_label() -> void:
	var p := _new_panel()
	p.set_title("Hello")
	var lbl := p.get_title_label()
	assert_object(lbl).is_not_null()
	assert_str(lbl.text).is_equal("Hello")
	assert_str(lbl.name).is_equal("TitleLabel")


func test_set_title_reuses_same_label() -> void:
	var p := _new_panel()
	p.set_title("First")
	var lbl1 := p.get_title_label()
	p.set_title("Second")
	var lbl2 := p.get_title_label()
	assert_object(lbl2).is_equal(lbl1)
	assert_str(lbl2.text).is_equal("Second")
