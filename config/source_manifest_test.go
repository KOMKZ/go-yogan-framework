package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadManifest(t *testing.T) {
	dir := t.TempDir()
	writeConfigFile(t, filepath.Join(dir, "config.yaml"), `
yogan:
  config:
    mode: manifest
    imports:
      - runtime.yaml
      - database.yaml
`)

	manifest, err := LoadManifest(filepath.Join(dir, "config.yaml"))

	require.NoError(t, err)
	assert.True(t, manifest.Enabled())
	assert.Equal(t, []string{"runtime.yaml", "database.yaml"}, manifest.Imports)
}

func TestManifestSourceLoadsImportsAndProfiles(t *testing.T) {
	dir := t.TempDir()
	writeConfigFile(t, filepath.Join(dir, "runtime.yaml"), `
api_server:
  port: 8080
logger:
  level: info
`)
	require.NoError(t, os.Mkdir(filepath.Join(dir, "test"), 0755))
	writeConfigFile(t, filepath.Join(dir, "test", "runtime.yaml"), `
api_server:
  port: 18080
`)
	writeConfigFile(t, filepath.Join(dir, "database.yaml"), `
database:
  connections:
    master:
      driver: mysql
`)
	writeConfigFile(t, filepath.Join(dir, "test", "database.yaml"), `
database:
  connections:
    master:
      driver: sqlite
`)

	source := NewManifestSource(dir, []string{"runtime.yaml", "database.yaml"}, []string{"test"}, 20)
	data, err := source.Load()

	require.NoError(t, err)
	assert.Equal(t, 18080, data["api_server.port"])
	assert.Equal(t, "info", data["logger.level"])
	assert.Equal(t, "sqlite", data["database.connections.master.driver"])
	assert.Equal(t, []string{
		filepath.Join(dir, "runtime.yaml"),
		filepath.Join(dir, "test", "runtime.yaml"),
		filepath.Join(dir, "database.yaml"),
		filepath.Join(dir, "test", "database.yaml"),
	}, source.LoadedFiles())
}

func TestManifestSourceAllowsOptionalMissingImport(t *testing.T) {
	dir := t.TempDir()
	writeConfigFile(t, filepath.Join(dir, "runtime.yaml"), `
logger:
  level: info
`)

	source := NewManifestSource(dir, []string{"runtime.yaml", "optional:local.yaml"}, nil, 20)
	data, err := source.Load()

	require.NoError(t, err)
	assert.Equal(t, "info", data["logger.level"])
}

func TestManifestSourceRejectsMissingRequiredImport(t *testing.T) {
	dir := t.TempDir()

	source := NewManifestSource(dir, []string{"missing.yaml"}, nil, 20)
	_, err := source.Load()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

func TestManifestSourceRejectsDuplicateBaseKeys(t *testing.T) {
	dir := t.TempDir()
	writeConfigFile(t, filepath.Join(dir, "runtime.yaml"), `
logger:
  level: info
`)
	writeConfigFile(t, filepath.Join(dir, "logging.yaml"), `
logger:
  level: debug
`)

	source := NewManifestSource(dir, []string{"runtime.yaml", "logging.yaml"}, nil, 20)
	_, err := source.Load()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate config section")
	assert.Contains(t, err.Error(), "logger")
}

func TestManifestSourceRejectsDuplicateBaseSections(t *testing.T) {
	dir := t.TempDir()
	writeConfigFile(t, filepath.Join(dir, "database.yaml"), `
database:
  connections:
    master:
      driver: mysql
`)
	writeConfigFile(t, filepath.Join(dir, "database-extra.yaml"), `
database:
  max_idle_conns: 5
`)

	source := NewManifestSource(dir, []string{"database.yaml", "database-extra.yaml"}, nil, 20)
	_, err := source.Load()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate config section")
	assert.Contains(t, err.Error(), "database")
}

func TestManifestSourceRejectsProfileSectionsOutsideBase(t *testing.T) {
	dir := t.TempDir()
	writeConfigFile(t, filepath.Join(dir, "database.yaml"), `
database:
  connections: {}
`)
	require.NoError(t, os.Mkdir(filepath.Join(dir, "test"), 0755))
	writeConfigFile(t, filepath.Join(dir, "test", "database.yaml"), `
logger:
  level: debug
`)

	source := NewManifestSource(dir, []string{"database.yaml"}, []string{"test"}, 20)
	_, err := source.Load()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside its base import")
}

func TestManifestSourceRejectsEscapingImports(t *testing.T) {
	dir := t.TempDir()

	tests := []string{
		"../secret.yaml",
		"/tmp/secret.yaml",
		"config.yaml",
		"database.json",
		"",
	}

	for _, item := range tests {
		t.Run(item, func(t *testing.T) {
			source := NewManifestSource(dir, []string{item}, nil, 20)
			_, err := source.Load()
			require.Error(t, err)
		})
	}
}

func TestSplitProfiles(t *testing.T) {
	assert.Equal(t, []string{"test", "local"}, splitProfiles("test, local,,"))
	assert.Empty(t, splitProfiles(""))
}

func TestProfileVariantPathRejectsUnsafeProfiles(t *testing.T) {
	dir := t.TempDir()
	for _, profile := range []string{"../test", "/test", "test/local", `test\local`, "."} {
		t.Run(profile, func(t *testing.T) {
			_, err := profileVariantPath(dir, filepath.Join(dir, "runtime.yaml"), profile)
			require.Error(t, err)
		})
	}
}

func TestLoaderReloadResetsLoadedFiles(t *testing.T) {
	dir := t.TempDir()
	writeConfigFile(t, filepath.Join(dir, "config.yaml"), `
yogan:
  config:
    mode: manifest
    imports:
      - runtime.yaml
`)
	writeConfigFile(t, filepath.Join(dir, "runtime.yaml"), `
logger:
  level: info
`)

	loader, err := NewLoaderBuilder().WithConfigPath(dir).Build()
	require.NoError(t, err)
	require.Len(t, loader.GetLoadedFiles(), 2)

	require.NoError(t, loader.Reload())
	assert.Len(t, loader.GetLoadedFiles(), 2)
}

func writeConfigFile(t *testing.T, path string, body string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(strings.TrimSpace(body)+"\n"), 0644))
}
