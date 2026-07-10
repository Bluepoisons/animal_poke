package safety

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvaluate_FixtureMatrix(t *testing.T) {
	cases := []struct {
		name       string
		fixture    string
		hasAnimal  bool
		wantCode   string
		wantAction string
		wantColl   bool
		wantFlags  []string
		wantReport string
	}{
		{
			name: "person pure portrait", fixture: FixturePerson, hasAnimal: false,
			wantCode: CodeRejectPortrait, wantAction: ActionReject, wantColl: false,
			wantFlags: []string{FlagFace},
		},
		{
			name: "child focus", fixture: FixtureChild, hasAnimal: false,
			wantCode: CodeRejectChildFocus, wantAction: ActionReject, wantColl: false,
			wantFlags: []string{FlagFace, FlagChild},
		},
		{
			name: "person plus animal", fixture: FixturePersonAnimal, hasAnimal: true,
			wantCode: CodeFlagSensitive, wantAction: ActionFlag, wantColl: true,
			wantFlags: []string{FlagFace},
		},
		{
			name: "plate only", fixture: FixturePlate, hasAnimal: false,
			wantCode: CodeRejectSensitive, wantAction: ActionReject, wantColl: false,
			wantFlags: []string{FlagPlate},
		},
		{
			name: "house only", fixture: FixtureHouse, hasAnimal: false,
			wantCode: CodeRejectSensitive, wantAction: ActionReject, wantColl: false,
			wantFlags: []string{FlagHouse},
		},
		{
			name: "abuse", fixture: FixtureAbuse, hasAnimal: false,
			wantCode: CodeFlagAbuse, wantAction: ActionFlag, wantColl: false,
			wantFlags: []string{FlagAbuse}, wantReport: ReportPathAbuse,
		},
		{
			name: "injured with animal", fixture: FixtureInjured, hasAnimal: true,
			wantCode: CodeFlagInjured, wantAction: ActionFlag, wantColl: true,
			wantFlags: []string{FlagInjured}, wantReport: ReportPathInjured,
		},
		{
			name: "safe animal", fixture: FixtureSafeAnimal, hasAnimal: true,
			wantCode: CodeOK, wantAction: ActionAllow, wantColl: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := Evaluate(Input{
				FixtureLabel:        tc.fixture,
				HasCapturableAnimal: tc.hasAnimal,
			})
			assert.Equal(t, tc.wantCode, res.DecisionCode)
			assert.Equal(t, tc.wantAction, res.Action)
			assert.Equal(t, tc.wantColl, res.Collectable)
			assert.Equal(t, tc.wantReport, res.ReportPath)
			for _, f := range tc.wantFlags {
				assert.Contains(t, res.Flags, f)
			}
			// Client view must not leak internal notes.
			view := res.ToClientView()
			b, err := json.Marshal(view)
			require.NoError(t, err)
			assert.NotContains(t, string(b), "fixture:")
			assert.NotContains(t, string(b), "InternalNotes")
			assert.Equal(t, tc.wantCode, view.DecisionCode)
		})
	}
}

func TestEvaluate_LabelHeuristics(t *testing.T) {
	res := Evaluate(Input{Labels: []string{"person", "human portrait"}})
	assert.Equal(t, CodeRejectPortrait, res.DecisionCode)
	assert.False(t, res.Collectable)

	res = Evaluate(Input{Labels: []string{"cat", "person"}, HasCapturableAnimal: true})
	assert.Equal(t, CodeFlagSensitive, res.DecisionCode)
	assert.True(t, res.Collectable)
	assert.Contains(t, res.Flags, FlagFace)

	res = Evaluate(Input{Filename: "plate_front.jpg"})
	assert.Equal(t, CodeRejectSensitive, res.DecisionCode)
	assert.Contains(t, res.Flags, FlagPlate)
}

func TestEvaluate_StableAcrossRuns(t *testing.T) {
	in := Input{FixtureLabel: "person"}
	a := Evaluate(in)
	b := Evaluate(in)
	assert.Equal(t, a, b)
}

func TestClientView_NoModelInternals(t *testing.T) {
	res := Evaluate(Input{FixtureLabel: FixtureAbuse, Labels: []string{"internal-model-xyz-v9"}})
	view := res.ToClientView()
	raw, _ := json.Marshal(view)
	s := string(raw)
	assert.NotContains(t, s, "internal-model")
	assert.NotContains(t, s, "xyz")
	assert.Contains(t, s, CodeFlagAbuse)
}

func TestLogProviderNoTrain_NoImageRetention(t *testing.T) {
	ResetPolicyAudits()
	// Simulate accidental attempt to log image-like payload — digest only is stored.
	fakeImage := []byte{0xff, 0xd8, 0xff, 0xd9, 'S', 'E', 'C', 'R', 'E', 'T'}
	digest := "abc123digest"
	entry := LogProviderNoTrain("vision", "detect", "vision-model", digest, "dev-1", "req-1")
	assert.Equal(t, ProviderNoTrainPolicyID, entry.PolicyID)
	assert.False(t, entry.RetainImage)
	assert.False(t, entry.AllowTrain)
	assert.Equal(t, digest, entry.InputDigest)
	assert.NotContains(t, entry.InputDigest, "SECRET")
	assert.NotContains(t, entry.ModelHint, string(fakeImage))

	all := RecentPolicyAudits()
	require.Len(t, all, 1)
	b, _ := json.Marshal(all[0])
	assert.NotContains(t, string(b), "SECRET")
	assert.NotContains(t, strings.ToLower(string(b)), "base64")
	assert.Contains(t, string(b), `"retain_image":false`)
	assert.Contains(t, string(b), `"allow_train":false`)
}

func TestMinorAccountDefaults_Strict(t *testing.T) {
	loose := MinorAccountDefaults(false)
	assert.Equal(t, "minor", loose.Audience)
	assert.Equal(t, 8, loose.PlayHoursStart)
	assert.Equal(t, 22, loose.PlayHoursEnd)
	assert.Equal(t, "city", loose.LocationScope)
	assert.True(t, loose.SocialEnabled)

	strict := MinorAccountDefaults(true)
	assert.True(t, strict.Strict)
	assert.Equal(t, "none", strict.LocationScope)
	assert.False(t, strict.SocialEnabled)
	assert.False(t, strict.FriendsDefault)
	assert.False(t, strict.ShareCaptureDefault)
	assert.False(t, strict.PreciseLocationDefault)

	adult := ResolveAccountDefaults(false, true)
	assert.Equal(t, "adult", adult.Audience)
	assert.True(t, adult.SocialEnabled)
}

func TestNormalizeFixture(t *testing.T) {
	assert.Equal(t, FixturePerson, NormalizeFixture("Human"))
	assert.Equal(t, FixturePersonAnimal, NormalizeFixture("person+animal"))
	assert.Equal(t, FixturePlate, NormalizeFixture("车牌"))
	assert.Equal(t, FixtureAbuse, NormalizeFixture("animal_abuse"))
}
