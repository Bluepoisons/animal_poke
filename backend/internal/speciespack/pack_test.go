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

	cap := r.CapturableIDs()
	assert.Equal(t, []string{"cat", "dog", "goose"}, cap)
	assert.NotContains(t, cap, "rabbit")
	assert.Equal(t, StatusCatalogOnly, r.EffectiveStatusOf("rabbit"))
	assert.False(t, r.Capturable("rabbit"))
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
		{"duck", IDUnsupported},
		{"bird", IDUnsupported},
		{"human", IDUnsupported},
		{"mongoose", IDUnsupported},
		{"", IDUnknown},
		{"horse", IDUnknown},
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

func TestFourthSpeciesPilot_EncyclopediaOnly(t *testing.T) {
	r := NewRegistry()
	require.NoError(t, r.RegisterAll(builtinPacks()...))
	assert.True(t, InEncyclopedia(mustGet(t, r, "rabbit")))
	assert.False(t, r.Capturable("rabbit"))
	assert.Equal(t, StatusCatalogOnly, r.EffectiveStatusOf("rabbit"))
	// 交叉引用：内容 ID + 版本
	p, ok := r.Get("rabbit")
	require.True(t, ok)
	assert.Equal(t, "species.rabbit", p.ContentID)
	assert.Equal(t, "1.0.0", p.Version)
	assert.Equal(t, Ref{ID: "rabbit", Version: "1.0.0"}, p.Ref())
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
