package repo

import (
	"testing"

	"animalpoke/backend/internal/models"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSeedContentMigratesExistingAuthoredNodeWithoutReactivatingIt(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NarrativeNode{}, &models.NarrativeChoice{}, &models.StoryFragment{}))

	narratives := NewNarrativeRepo(db)
	require.NoError(t, narratives.SeedContent())
	require.NoError(t, db.Model(&models.NarrativeNode{}).
		Where("node_id = ?", "ch4_ending_materials").
		Updates(map[string]any{"kind": "ending", "active": false, "withdrawn": true}).Error)

	require.NoError(t, narratives.SeedContent())
	var node models.NarrativeNode
	require.NoError(t, db.Where("node_id = ?", "ch4_ending_materials").First(&node).Error)
	require.Equal(t, "story", node.Kind)
	require.False(t, node.Active, "a content refresh must not republish a withdrawn node")
	require.True(t, node.Withdrawn, "a content refresh must not clear an editorial withdrawal")
}
