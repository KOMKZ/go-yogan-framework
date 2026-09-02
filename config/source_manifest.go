package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	manifestModeKey    = "yogan.config.mode"
	manifestImportsKey = "yogan.config.imports"
)

// Manifest declares config files imported by config.yaml.
type Manifest struct {
	Mode    string
	Imports []string
}

// Enabled reports whether config.yaml opted into manifest loading.
func (m Manifest) Enabled() bool {
	return strings.EqualFold(strings.TrimSpace(m.Mode), "manifest") && len(m.Imports) > 0
}

// LoadManifest reads config.yaml and extracts yogan.config imports.
func LoadManifest(path string) (Manifest, error) {
	data, err := NewFileSource(path, 10).Load()
	if err != nil {
		return Manifest{}, err
	}
	return Manifest{
		Mode:    asString(data[manifestModeKey]),
		Imports: asStringSlice(data[manifestImportsKey]),
	}, nil
}

// ManifestSource loads imported base files and their profile variants.
type ManifestSource struct {
	configPath  string
	imports     []string
	profiles    []string
	priority    int
	loadedFiles []string
}

// NewManifestSource creates a source from manifest imports.
func NewManifestSource(configPath string, imports []string, profiles []string, priority int) *ManifestSource {
	return &ManifestSource{
		configPath: configPath,
		imports:    append([]string(nil), imports...),
		profiles:   append([]string(nil), profiles...),
		priority:   priority,
	}
}

func (s *ManifestSource) Name() string {
	return "manifest:" + s.configPath
}

func (s *ManifestSource) Priority() int {
	return s.priority
}

func (s *ManifestSource) LoadedFiles() []string {
	return append([]string(nil), s.loadedFiles...)
}

func (s *ManifestSource) Load() (map[string]interface{}, error) {
	s.loadedFiles = nil
	result := make(map[string]interface{})
	baseKeys := make(map[string]string)
	baseRoots := make(map[string]string)

	for _, item := range s.imports {
		importPath, optional, err := s.resolveImport(item)
		if err != nil {
			return nil, err
		}
		baseData, exists, err := s.loadExistingFile(importPath, optional)
		if err != nil {
			return nil, err
		}
		if !exists {
			continue
		}

		roots := topLevelKeys(baseData)
		for root := range roots {
			if firstFile, ok := baseRoots[root]; ok {
				return nil, fmt.Errorf("duplicate config section %q in manifest imports: first=%s second=%s", root, firstFile, importPath)
			}
			baseRoots[root] = importPath
		}
		for key, value := range baseData {
			if firstFile, ok := baseKeys[key]; ok {
				return nil, fmt.Errorf("duplicate config key %q in manifest imports: first=%s second=%s", key, firstFile, importPath)
			}
			baseKeys[key] = importPath
			result[key] = value
		}

		for _, profile := range s.profiles {
			profilePath := profileVariantPath(importPath, profile)
			profileData, exists, err := s.loadExistingFile(profilePath, true)
			if err != nil {
				return nil, err
			}
			if !exists {
				continue
			}
			if err := validateProfileKeys(profilePath, profileData, roots); err != nil {
				return nil, err
			}
			for key, value := range profileData {
				result[key] = value
			}
		}
	}

	return result, nil
}

func (s *ManifestSource) resolveImport(item string) (string, bool, error) {
	value := strings.TrimSpace(item)
	optional := false
	if strings.HasPrefix(value, "optional:") {
		optional = true
		value = strings.TrimSpace(strings.TrimPrefix(value, "optional:"))
	}
	if value == "" {
		return "", false, fmt.Errorf("empty manifest import")
	}
	if filepath.IsAbs(value) {
		return "", false, fmt.Errorf("manifest import %q must be relative", item)
	}
	cleaned := filepath.Clean(value)
	if cleaned == "." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) || cleaned == ".." {
		return "", false, fmt.Errorf("manifest import %q cannot escape config directory", item)
	}
	if cleaned == "config.yaml" {
		return "", false, fmt.Errorf("manifest import %q cannot import config.yaml", item)
	}
	ext := filepath.Ext(cleaned)
	if ext != ".yaml" && ext != ".yml" {
		return "", false, fmt.Errorf("manifest import %q must be a yaml file", item)
	}
	return filepath.Join(s.configPath, cleaned), optional, nil
}

func (s *ManifestSource) loadExistingFile(path string, optional bool) (map[string]interface{}, bool, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) && optional {
			return nil, false, nil
		}
		if os.IsNotExist(err) {
			return nil, false, fmt.Errorf("manifest import %s does not exist", path)
		}
		return nil, false, err
	}
	data, err := NewFileSource(path, s.priority).Load()
	if err != nil {
		return nil, false, err
	}
	s.loadedFiles = append(s.loadedFiles, path)
	return data, true, nil
}

func profileVariantPath(basePath, profile string) string {
	profile = strings.TrimSpace(profile)
	if profile == "" {
		return basePath
	}
	ext := filepath.Ext(basePath)
	stem := strings.TrimSuffix(basePath, ext)
	return stem + "." + profile + ext
}

func splitProfiles(env string) []string {
	parts := strings.Split(env, ",")
	profiles := make([]string, 0, len(parts))
	for _, part := range parts {
		profile := strings.TrimSpace(part)
		if profile != "" {
			profiles = append(profiles, profile)
		}
	}
	return profiles
}

func topLevelKeys(data map[string]interface{}) map[string]bool {
	roots := make(map[string]bool)
	for key := range data {
		root := key
		if i := strings.IndexByte(key, '.'); i >= 0 {
			root = key[:i]
		}
		roots[root] = true
	}
	return roots
}

func validateProfileKeys(path string, data map[string]interface{}, allowed map[string]bool) error {
	for root := range topLevelKeys(data) {
		if !allowed[root] {
			return fmt.Errorf("profile config %s defines section %q outside its base import", path, root)
		}
	}
	return nil
}

func asString(value interface{}) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

func asStringSlice(value interface{}) []string {
	switch v := value.(type) {
	case []string:
		return v
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if text, ok := item.(string); ok {
				result = append(result, text)
			}
		}
		return result
	default:
		return nil
	}
}
