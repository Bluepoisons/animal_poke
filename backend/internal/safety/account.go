package safety

// AccountDefaults describes privacy / social defaults applied to accounts.
// When StrictMinorDefaults is enabled, minors get tighter time/location/social defaults.
type AccountDefaults struct {
	// Audience: "minor" | "adult"
	Audience string `json:"audience"`
	// Strict indicates the strict-minor config flag was applied.
	Strict bool `json:"strict"`

	// Time restrictions
	PlayHoursStart int `json:"play_hours_start"` // inclusive, local hour 0-23
	PlayHoursEnd   int `json:"play_hours_end"`   // exclusive
	DailyLimitMin  int `json:"daily_limit_min"`

	// Location defaults
	// Scope: none | city | precise
	LocationScope          string `json:"location_scope"`
	PreciseLocationDefault bool   `json:"precise_location_default"`
	ShareLocationDefault   bool   `json:"share_location_default"`

	// Social defaults
	SocialEnabled        bool `json:"social_enabled"`
	FriendsDefault       bool `json:"friends_default"`
	PublicProfileDefault bool `json:"public_profile_default"`
	ShareCaptureDefault  bool `json:"share_capture_default"`
}

// DefaultAccountDefaults returns adult defaults (open social/location opt-in still required by consent).
func DefaultAccountDefaults() AccountDefaults {
	return AccountDefaults{
		Audience:               "adult",
		Strict:                 false,
		PlayHoursStart:         0,
		PlayHoursEnd:           24,
		DailyLimitMin:          0, // unlimited
		LocationScope:          "city",
		PreciseLocationDefault: false,
		ShareLocationDefault:   false,
		SocialEnabled:          true,
		FriendsDefault:         true,
		PublicProfileDefault:   false,
		ShareCaptureDefault:    true,
	}
}

// MinorAccountDefaults returns defaults for minor accounts.
// AP-083：未成年人默认关闭社交；strict=true 时进一步收紧位置。
func MinorAccountDefaults(strict bool) AccountDefaults {
	d := AccountDefaults{
		Audience:               "minor",
		Strict:                 strict,
		PlayHoursStart:         8,
		PlayHoursEnd:           22,
		DailyLimitMin:          90,
		LocationScope:          "city",
		PreciseLocationDefault: false,
		ShareLocationDefault:   false,
		SocialEnabled:          false,
		FriendsDefault:         false,
		PublicProfileDefault:   false,
		ShareCaptureDefault:    false,
	}
	if strict {
		d.LocationScope = "none"
		d.PreciseLocationDefault = false
		d.ShareLocationDefault = false
	}
	return d
}

// ResolveAccountDefaults picks defaults from age flag + strict config.
func ResolveAccountDefaults(isMinor, strictMinor bool) AccountDefaults {
	if isMinor {
		return MinorAccountDefaults(strictMinor)
	}
	return DefaultAccountDefaults()
}
