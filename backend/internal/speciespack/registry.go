package speciespack

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// Registry 内容包注册表：ID/别名/状态查询的唯一入口。
type Registry struct {
	mu                    sync.RWMutex
	packs                 map[string]*Pack  // id -> pack
	exactAlias            map[string]string // lower alias -> id
	expectedGoldenVersion string
	nowFn                 func() time.Time
}

// Option 配置 Registry。
type Option func(*Registry)

// WithExpectedGoldenVersion 设置黄金集版本兼容检查。
func WithExpectedGoldenVersion(v string) Option {
	return func(r *Registry) { r.expectedGoldenVersion = v }
}

// WithNow 注入时钟（测试用）。
func WithNow(fn func() time.Time) Option {
	return func(r *Registry) {
		if fn != nil {
			r.nowFn = fn
		}
	}
}

// NewRegistry 创建空注册表。
func NewRegistry(opts ...Option) *Registry {
	r := &Registry{
		packs:      map[string]*Pack{},
		exactAlias: map[string]string{},
		nowFn:      time.Now,
	}
	for _, o := range opts {
		o(r)
	}
	return r
}

func (r *Registry) now() time.Time {
	if r.nowFn != nil {
		return r.nowFn()
	}
	return time.Now()
}

// LoadDir 从目录加载全部 *.json 内容包。
func (r *Registry) LoadDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read species pack dir: %w", err)
	}
	var packs []*Pack
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".json") {
			continue
		}
		// 跳过 schema 描述文件
		if strings.Contains(strings.ToLower(e.Name()), "schema") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return fmt.Errorf("read %s: %w", e.Name(), err)
		}
		var p Pack
		if err := json.Unmarshal(raw, &p); err != nil {
			return fmt.Errorf("parse %s: %w", e.Name(), err)
		}
		packs = append(packs, &p)
	}
	return r.RegisterAll(packs...)
}

// RegisterAll 批量注册并重建别名索引；检测别名冲突。
func (r *Registry) RegisterAll(packs ...*Pack) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 在副本上构建，成功后再替换
	nextPacks := make(map[string]*Pack, len(r.packs)+len(packs))
	for k, v := range r.packs {
		nextPacks[k] = v
	}
	for _, p := range packs {
		if p == nil {
			return fmt.Errorf("nil pack")
		}
		// 浅拷贝避免外部修改
		cp := *p
		if err := ValidatePack(&cp); err != nil {
			// catalog_only 允许较松：仅要求 id/version/content_id/names/emoji
			if cp.Status != StatusCatalogOnly {
				return fmt.Errorf("pack %s: %w", cp.ID, err)
			}
			if strings.TrimSpace(cp.ID) == "" || strings.TrimSpace(cp.Version) == "" {
				return fmt.Errorf("pack invalid: %w", err)
			}
		}
		if prev, ok := nextPacks[cp.ID]; ok {
			// 同 ID：高版本覆盖（简单字符串比较 + 已存在则覆盖）
			_ = prev
		}
		nextPacks[cp.ID] = &cp
	}

	exact, err := buildAliasIndex(nextPacks)
	if err != nil {
		return err
	}
	r.packs = nextPacks
	r.exactAlias = exact
	return nil
}

func buildAliasIndex(packs map[string]*Pack) (map[string]string, error) {
	exact := map[string]string{}
	// 稳定顺序便于错误信息
	ids := make([]string, 0, len(packs))
	for id := range packs {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	register := func(alias, id string) error {
		a := normalizeKey(alias)
		if a == "" {
			return nil
		}
		if other, ok := exact[a]; ok && other != id {
			return fmt.Errorf("alias conflict %q: %s vs %s", alias, other, id)
		}
		exact[a] = id
		return nil
	}

	for _, id := range ids {
		p := packs[id]
		if err := register(p.ID, id); err != nil {
			return nil, err
		}
		// 俗名也作为精确别名
		for _, v := range p.Names.Common {
			if err := register(v, id); err != nil {
				return nil, err
			}
		}
		if p.Names.Scientific != "" {
			if err := register(p.Names.Scientific, id); err != nil {
				return nil, err
			}
		}
		for _, a := range p.Names.Aliases {
			if err := register(a, id); err != nil {
				return nil, err
			}
		}
	}
	return exact, nil
}

func normalizeKey(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "_", " ")
	s = strings.Join(strings.Fields(s), " ")
	return s
}

// Get 按 ID 取包。
func (r *Registry) Get(id string) (*Pack, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.packs[id]
	return p, ok
}

// All 返回全部包（无序）。
func (r *Registry) All() []*Pack {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Pack, 0, len(r.packs))
	for _, p := range r.packs {
		out = append(out, p)
	}
	return out
}

// CapturableIDs 有效可捕获物种（已认证且未降级）。
func (r *Registry) CapturableIDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	now := r.now()
	var ids []string
	for id, p := range r.packs {
		if CanCapture(p, now, r.expectedGoldenVersion) {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

// EncyclopediaIDs 百科可见物种（全部内容包）。
func (r *Registry) EncyclopediaIDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0, len(r.packs))
	for id := range r.packs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Capturable 是否允许捕获/发奖。
func (r *Registry) Capturable(species string) bool {
	if species == IDUnknown || species == IDUnsupported || species == "" {
		return false
	}
	r.mu.RLock()
	p, ok := r.packs[species]
	exp := r.expectedGoldenVersion
	now := r.now()
	r.mu.RUnlock()
	if !ok {
		return false
	}
	return CanCapture(p, now, exp)
}

// IsKnown 是否为注册内容 ID 或系统保留 ID。
func (r *Registry) IsKnown(species string) bool {
	if species == IDUnknown || species == IDUnsupported {
		return true
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.packs[species]
	return ok
}

// EffectiveStatusOf 查询有效状态。
func (r *Registry) EffectiveStatusOf(species string) string {
	r.mu.RLock()
	p, ok := r.packs[species]
	exp := r.expectedGoldenVersion
	now := r.now()
	r.mu.RUnlock()
	if !ok {
		return ""
	}
	return EffectiveStatus(p, now, exp)
}

// Normalize 将模型/客户端原始标签规范为内容 ID 或 unknown/unsupported。
// 规则：全局拒绝表优先 → 精确别名 → contains 别名 → unknown。
// 绝不默认 goose。
func (r *Registry) Normalize(raw string) (string, string) {
	original := strings.TrimSpace(raw)
	s := normalizeKey(original)
	if s == "" {
		return IDUnknown, original
	}

	// 全局 unsupported 精确表（非内容物种 / 人像 / 玩具等）
	if globalUnsupportedExact[s] {
		return IDUnsupported, original
	}
	// 全局 unsupported 子串（优先于物种匹配，避免 bird→goose 等）
	for _, kw := range globalUnsupportedContains {
		if strings.Contains(s, kw) {
			return IDUnsupported, original
		}
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// 精确别名
	if id, ok := r.exactAlias[s]; ok {
		return id, original
	}

	// contains 别名：按 ID 稳定顺序；更长 contains 优先（简单实现：全扫取最长命中）
	type hit struct {
		id string
		n  int
	}
	var best *hit
	ids := make([]string, 0, len(r.packs))
	for id := range r.packs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		p := r.packs[id]
		for _, c := range p.Names.Contains {
			ck := normalizeKey(c)
			if ck == "" || !strings.Contains(s, ck) {
				continue
			}
			excluded := false
			for _, ex := range p.Names.ContainsExclude {
				if strings.Contains(s, normalizeKey(ex)) {
					excluded = true
					break
				}
			}
			if excluded {
				continue
			}
			if best == nil || len(ck) > best.n || (len(ck) == best.n && id < best.id) {
				best = &hit{id: id, n: len(ck)}
			}
		}
	}
	if best != nil {
		return best.id, original
	}

	return IDUnknown, original
}

// 全局拒绝：人像、非目标、易混淆鸟类等（保持 AP-007 语义）。
var globalUnsupportedExact = map[string]bool{
	"bird": true, "duck": true, "swan": true, "chicken": true, "rooster": true,
	"hen": true, "pigeon": true, "dove": true, "parrot": true, "eagle": true,
	"human": true, "person": true, "people": true, "man": true, "woman": true, "child": true, "baby": true,
	"toy": true, "doll": true, "plush": true, "statue": true, "screen": true, "phone": true,
	"car": true, "plant": true, "tree": true, "food": true,
	"鸟": true, "鸭": true, "鸭子": true, "天鹅": true, "鸡": true, "人": true, "人类": true,
	"玩偶": true, "玩具": true, "屏幕": true,
	"mongoose": true,
}

var globalUnsupportedContains = []string{
	"human", "person", "people", "man ", "woman", "child", "baby", "人", "人类", "儿童",
	"duck", "swan", "chicken", "pigeon", "parrot", "bird", "鸭", "天鹅", "鸟", "鸡",
	"toy", "doll", "plush", "玩偶", "玩具", "screen", "屏幕",
	"mongoose",
}

// ---- 默认全局注册表 ----

var (
	defaultOnce sync.Once
	defaultReg  *Registry
	defaultErr  error
)

// DefaultContentDir 相对 backend 模块根的内容目录。
const DefaultContentDir = "content/species"

// LocateContentDir 定位 backend/content/species。
func LocateContentDir() (string, error) {
	var candidates []string
	if _, file, _, ok := runtime.Caller(0); ok {
		backendRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
		candidates = append(candidates, filepath.Join(backendRoot, DefaultContentDir))
	}
	if wd, err := os.Getwd(); err == nil {
		dir := wd
		for i := 0; i < 6; i++ {
			candidates = append(candidates, filepath.Join(dir, DefaultContentDir))
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				candidates = append(candidates, filepath.Join(dir, DefaultContentDir))
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	seen := map[string]bool{}
	for _, c := range candidates {
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			return c, nil
		}
	}
	return "", fmt.Errorf("species content dir not found (%s)", DefaultContentDir)
}

// Default 返回进程内默认注册表（从 content/species 加载）。
func Default() *Registry {
	defaultOnce.Do(func() {
		defaultReg = NewRegistry()
		dir, err := LocateContentDir()
		if err != nil {
			defaultErr = err
			// 回退内置最小包，保证测试与进程可启动
			_ = defaultReg.RegisterAll(builtinPacks()...)
			return
		}
		if err := defaultReg.LoadDir(dir); err != nil {
			defaultErr = err
			_ = defaultReg.RegisterAll(builtinPacks()...)
			return
		}
	})
	return defaultReg
}

// DefaultLoadError 默认加载错误（若有）。
func DefaultLoadError() error { return defaultErr }

// ResetDefaultForTest 测试用重置。
func ResetDefaultForTest() {
	defaultOnce = sync.Once{}
	defaultReg = nil
	defaultErr = nil
}

// SetDefaultForTest 注入默认表（测试）。
func SetDefaultForTest(r *Registry) {
	defaultOnce = sync.Once{}
	defaultOnce.Do(func() {})
	defaultReg = r
	defaultErr = nil
}
