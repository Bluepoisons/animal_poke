package mlqa

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// DefaultRelativeDir is the golden set path relative to the backend module root.
const DefaultRelativeDir = "testdata/vision_golden"

// LocateGoldenDir finds backend/testdata/vision_golden from this package or CWD.
func LocateGoldenDir() (string, error) {
	candidates := []string{}

	// Prefer path relative to this source file (stable under go test).
	if _, file, _, ok := runtime.Caller(0); ok {
		// internal/mlqa -> backend
		backendRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
		candidates = append(candidates, filepath.Join(backendRoot, DefaultRelativeDir))
	}

	// Fallback: walk up from CWD looking for go.mod + testdata.
	if wd, err := os.Getwd(); err == nil {
		dir := wd
		for i := 0; i < 6; i++ {
			candidates = append(candidates, filepath.Join(dir, DefaultRelativeDir))
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				candidates = append(candidates, filepath.Join(dir, DefaultRelativeDir))
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
		if st, err := os.Stat(filepath.Join(c, "manifest.json")); err == nil && !st.IsDir() {
			return c, nil
		}
	}
	return "", fmt.Errorf("vision golden dir not found (looked for %s/manifest.json)", DefaultRelativeDir)
}

// LoadManifest reads and validates manifest.json from dir (or auto-locate).
func LoadManifest(dir string) (*Manifest, error) {
	if dir == "" {
		var err error
		dir, err = LocateGoldenDir()
		if err != nil {
			return nil, err
		}
	}
	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	if err := validateManifest(&m); err != nil {
		return nil, err
	}
	return &m, nil
}

// LoadBaseline reads baseline.json from dir (or auto-locate).
func LoadBaseline(dir string) (*BaselineFile, error) {
	if dir == "" {
		var err error
		dir, err = LocateGoldenDir()
		if err != nil {
			return nil, err
		}
	}
	raw, err := os.ReadFile(filepath.Join(dir, "baseline.json"))
	if err != nil {
		return nil, fmt.Errorf("read baseline: %w", err)
	}
	var b BaselineFile
	if err := json.Unmarshal(raw, &b); err != nil {
		return nil, fmt.Errorf("parse baseline: %w", err)
	}
	if b.Metrics.PerClass == nil {
		b.Metrics.PerClass = map[string]ClassMetrics{}
	}
	return &b, nil
}

func validateManifest(m *Manifest) error {
	if m.Version == "" {
		return fmt.Errorf("manifest.version required")
	}
	if len(m.Fixtures) == 0 {
		return fmt.Errorf("manifest.fixtures empty")
	}
	requiredGroups := map[string]bool{
		"cat": false, "dog": false, "goose": false,
		"duck": false, "swan": false, "bird": false,
		"person": false, "empty": false,
	}
	ids := map[string]bool{}
	for i, f := range m.Fixtures {
		if f.ID == "" {
			return fmt.Errorf("fixture[%d]: id required", i)
		}
		if ids[f.ID] {
			return fmt.Errorf("fixture id duplicated: %s", f.ID)
		}
		ids[f.ID] = true
		if f.SpeciesGroup == "" {
			return fmt.Errorf("fixture %s: species_group required", f.ID)
		}
		if _, ok := requiredGroups[f.SpeciesGroup]; ok {
			requiredGroups[f.SpeciesGroup] = true
		}
		if f.Expected.Species == "" {
			return fmt.Errorf("fixture %s: expected.species required", f.ID)
		}
		if f.Image.Kind != "synthetic" && f.Image.Kind != "metadata" {
			return fmt.Errorf("fixture %s: unsupported image.kind %q", f.ID, f.Image.Kind)
		}
		if f.Expected.Capturable {
			if f.Expected.BBox == nil {
				return fmt.Errorf("fixture %s: capturable fixtures require bbox", f.ID)
			}
			if err := validateBBox(*f.Expected.BBox); err != nil {
				return fmt.Errorf("fixture %s: %w", f.ID, err)
			}
		} else if f.Expected.BBox != nil {
			if err := validateBBox(*f.Expected.BBox); err != nil {
				return fmt.Errorf("fixture %s: %w", f.ID, err)
			}
		}
	}
	for g, seen := range requiredGroups {
		if !seen {
			return fmt.Errorf("manifest missing required species_group %q", g)
		}
	}
	return nil
}

func validateBBox(b BBox) error {
	if b.X < 0 || b.Y < 0 || b.Width <= 0 || b.Height <= 0 {
		return fmt.Errorf("bbox must be positive and in range")
	}
	if b.X > 1 || b.Y > 1 || b.Width > 1 || b.Height > 1 {
		return fmt.Errorf("bbox must be normalized to [0,1]")
	}
	if b.X+b.Width > 1.0001 || b.Y+b.Height > 1.0001 {
		return fmt.Errorf("bbox exceeds image bounds")
	}
	return nil
}
