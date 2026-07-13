package speciespack

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadDir_DefaultContent(t *testing.T) {
	dir, err := LocateContentDir()
	require.NoError(t, err)
	r := NewRegistry()
	require.NoError(t, r.LoadDir(dir))

	ids := r.EncyclopediaIDs()
	assert.Contains(t, ids, "cat")
	assert.Contains(t, ids, "dog")
	assert.Contains(t, ids, "goose")
	assert.Contains(t, ids, "rabbit")
	assert.Contains(t, ids, "bird")
	assert.Contains(t, ids, "horse")
	assert.Contains(t, ids, "snake")
	assert.Contains(t, ids, "fish")
	assert.Contains(t, ids, "other_animal")
	assert.Len(t, ids, 36)

	cap := r.CapturableIDs()
	assert.ElementsMatch(t, ids, cap)
	assert.Equal(t, StatusCapturable, r.EffectiveStatusOf("rabbit"))
	assert.True(t, r.Capturable("rabbit"))
}

func TestAliasConflict(t *testing.T) {
	r := NewRegistry()
	err := r.RegisterAll(
		&Pack{
			ID: "a", Version: "1", ContentID: "species.a", Status: StatusCatalogOnly,
			Names:   Names{Common: Localized{"en": "A"}, Aliases: []string{"shared"}},
			Welfare: Welfare{Level: "unknown"}, Protection: Protection{Status: "none"},
			Assets: Assets{Emoji: "A"},
		},
		&Pack{
			ID: "b", Version: "1", ContentID: "species.b", Status: StatusCatalogOnly,
			Names:   Names{Common: Localized{"en": "B"}, Aliases: []string{"shared"}},
			Welfare: Welfare{Level: "unknown"}, Protection: Protection{Status: "none"},
			Assets: Assets{Emoji: "B"},
		},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "alias conflict")
}

func TestNormalize_FromPacks(t *testing.T) {
	r := NewRegistry()
	require.NoError(t, r.RegisterAll(builtinPacks()...))

	cases := []struct {
		raw  string
		want string
	}{
		{"cat", "cat"},
		{"Kitten", "cat"},
		{"英短猫", "cat"},
		{"dog", "dog"},
		{"小狗", "dog"},
		{"goose", "goose"},
		{"大鹅", "goose"},
		{"bunny", "rabbit"},
		{"野兔", "rabbit"},
		{"rabbit", "rabbit"},
		{"duck", "duck"},
		{"bird", "bird"},
		{"swan", "bird"},
		{"horse", "horse"},
		{"青蛙", "frog"},
		{"海马", "fish"},
		{"一只海马", "fish"},
		{"牛蛙", "frog"},
		{"一只牛蛙", "frog"},
		{"食人鱼", "fish"},
		{"一条食人鱼", "fish"},
		{"河马", IDUnknown},
		{"蜗牛", IDUnknown},
		{"海牛", IDUnknown},
		{"木马", IDUnsupported},
		{"seahorse", "fish"},
		{"a horse in grass", "horse"},
		{"workhorse", IDUnknown},
		{"catfish", IDUnknown},
		{"caracal", IDUnknown},
		{"other_animal", "other_animal"},
		{"human", IDUnsupported},
		{"kid", IDUnsupported},
		{"小孩", IDUnsupported},
		{"mongoose", IDUnknown},
		{"", IDUnknown},
		{"unknown animal", IDUnknown},
	}
	for _, tc := range cases {
		got, _ := r.Normalize(tc.raw)
		assert.Equal(t, tc.want, got, "raw=%q", tc.raw)
	}
}

func TestCertificationDegrade_Expired(t *testing.T) {
	exp := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	p := &Pack{
		ID: "fox", Version: "1.0.0", ContentID: "species.fox", Status: StatusCapturable,
		Certification: &Certification{GoldenSetVersion: "1.0.0", ExpiresAt: &exp},
		Names:         Names{Common: Localized{"en": "Fox"}},
		Welfare:       Welfare{Level: "wildlife"},
		Protection:    Protection{Status: "none"},
		Assets:        Assets{Emoji: "🦊"},
		Gameplay: Gameplay{
			DetectThreshold: 0.9,
			StatModifiers:   &StatModifiers{HP: 1, ATK: 1, DEF: 1, SPD: 1},
			RarityWeights:   []RarityWeight{{Tier: "common", Weight: 1}},
		},
	}
	require.NoError(t, ValidatePack(p))
	now := time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, StatusCatalogOnly, EffectiveStatus(p, now, ""))
	assert.False(t, CanCapture(p, now, ""))
}

func TestCertificationDegrade_GoldenVersionMismatch(t *testing.T) {
	p := &Pack{
		ID: "fox", Version: "1.0.0", ContentID: "species.fox", Status: StatusCapturable,
		Certification: &Certification{GoldenSetVersion: "1.0.0"},
		Names:         Names{Common: Localized{"en": "Fox"}},
		Welfare:       Welfare{Level: "wildlife"},
		Protection:    Protection{Status: "none"},
		Assets:        Assets{Emoji: "🦊"},
		Gameplay: Gameplay{
			DetectThreshold: 0.9,
			StatModifiers:   &StatModifiers{HP: 1, ATK: 1, DEF: 1, SPD: 1},
			RarityWeights:   []RarityWeight{{Tier: "common", Weight: 1}},
		},
	}
	now := time.Now()
	assert.Equal(t, StatusCapturable, EffectiveStatus(p, now, "1.9.0"))
	assert.Equal(t, StatusCatalogOnly, EffectiveStatus(p, now, "2.0.0"))
}

func TestCapturableMissingGameplay_DegradesToCertified(t *testing.T) {
	// 构造：声明 capturable 但运行时清掉 gameplay 关键字段 → recognition_certified
	p := &Pack{
		ID: "fox", Version: "1.0.0", ContentID: "species.fox", Status: StatusCapturable,
		Certification: &Certification{GoldenSetVersion: "1.0.0"},
		Names:         Names{Common: Localized{"en": "Fox"}},
		Welfare:       Welfare{Level: "wildlife"},
		Protection:    Protection{Status: "none"},
		Assets:        Assets{Emoji: "🦊"},
		Gameplay: Gameplay{
			DetectThreshold: 0.9,
			StatModifiers:   &StatModifiers{HP: 1, ATK: 1, DEF: 1, SPD: 1},
			RarityWeights:   []RarityWeight{{Tier: "common", Weight: 1}},
		},
	}
	require.NoError(t, ValidatePack(p))
	p.Gameplay.StatModifiers = nil
	assert.Equal(t, StatusRecognitionCertified, EffectiveStatus(p, time.Now(), ""))
	assert.False(t, CanCapture(p, time.Now(), ""))
}

func TestRabbitContentRefCapturable(t *testing.T) {
	r := NewRegistry()
	require.NoError(t, r.RegisterAll(builtinPacks()...))
	assert.True(t, InEncyclopedia(mustGet(t, r, "rabbit")))
	assert.True(t, r.Capturable("rabbit"))
	assert.Equal(t, StatusCapturable, r.EffectiveStatusOf("rabbit"))
	// 交叉引用：内容 ID + 版本
	p, ok := r.Get("rabbit")
	require.True(t, ok)
	assert.Equal(t, "species.rabbit", p.ContentID)
	assert.Equal(t, "1.0.0", p.Version)
	assert.Equal(t, Ref{ID: "rabbit", Version: "1.0.0"}, p.Ref())
}

func TestMatchesAnimalLabel_BroadRegistry(t *testing.T) {
	r := NewRegistry()
	require.NoError(t, r.RegisterAll(builtinPacks()...))
	for _, label := range []string{"horse", "一只鸭子", "green snake", "青蛙", "海马", "牛蛙", "食人鱼", "fish", "octopus", "butterfly", "person with horse"} {
		assert.True(t, r.MatchesAnimalLabel(label), "label=%q", label)
	}
	for _, label := range []string{"human", "kid", "toy", "screen", "plant", "car", "河马", "蜗牛", "海牛", "木马", "workhorse", "catfish", "unknown animal"} {
		assert.False(t, r.MatchesAnimalLabel(label), "label=%q", label)
	}
}

func TestResolveExactAlias_DoesNotUseContains(t *testing.T) {
	r := NewRegistry()
	require.NoError(t, r.RegisterAll(builtinPacks()...))

	id, ok := r.ResolveExactAlias("猫")
	require.True(t, ok)
	assert.Equal(t, "cat", id)

	for _, label := range []string{"桌子猫", "长颈鹿", "斑马"} {
		_, ok := r.ResolveExactAlias(label)
		assert.False(t, ok, "label=%q", label)
	}
}

func TestProtectedPackRequiresRemoteObservation(t *testing.T) {
	p := broadBuiltinPacks()[0]
	p.ID = "protected-test"
	p.ContentID = "species.protected-test"
	p.Protection.Status = "protected"
	p.ObservationTips = Localized{"zh-CN": "请安静观察"}
	err := ValidatePack(p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "remote observation")
}

func TestSchemaRejectsReservedID(t *testing.T) {
	err := ValidatePack(&Pack{
		ID: IDUnknown, Version: "1", ContentID: "x", Status: StatusCatalogOnly,
		Names: Names{Common: Localized{"en": "U"}}, Welfare: Welfare{Level: "unknown"},
		Protection: Protection{Status: "none"}, Assets: Assets{Emoji: "?"},
	})
	require.Error(t, err)
}

func TestLoadDir_IgnoresSchemaFile(t *testing.T) {
	dir := t.TempDir()
	// copy one real pack
	src, err := LocateContentDir()
	require.NoError(t, err)
	raw, err := os.ReadFile(filepath.Join(src, "rabbit.v1.json"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "rabbit.v1.json"), raw, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.schema.json"), []byte(`{"title":"x"}`), 0o600))

	r := NewRegistry()
	require.NoError(t, r.LoadDir(dir))
	assert.Equal(t, []string{"rabbit"}, r.EncyclopediaIDs())
}

func mustGet(t *testing.T, r *Registry, id string) *Pack {
	t.Helper()
	p, ok := r.Get(id)
	require.True(t, ok)
	return p
}
