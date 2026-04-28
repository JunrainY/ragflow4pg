package server

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFromEnvironmentsAcceptsPostgresDBType(t *testing.T) {
	t.Setenv("DB_TYPE", "postgres")

	original := globalConfig
	t.Cleanup(func() {
		globalConfig = original
	})
	globalConfig = &Config{}

	if err := FromEnvironments(); err != nil {
		t.Fatalf("FromEnvironments returned error: %v", err)
	}

	if globalConfig.Database.Driver != "postgres" {
		t.Fatalf("expected postgres driver, got %q", globalConfig.Database.Driver)
	}
}

func TestFromConfigFileMapsPostgresSection(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "service_conf.yaml")
	content := []byte(`
postgres:
  name: rag_flow
  user: rag_flow
  password: secret
  host: localhost
  port: 5432
`)
	if err := os.WriteFile(configPath, content, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	original := globalConfig
	t.Cleanup(func() {
		globalConfig = original
	})
	globalConfig = &Config{}

	if err := FromConfigFile(configPath); err != nil {
		t.Fatalf("FromConfigFile returned error: %v", err)
	}

	if globalConfig.Database.Driver != "postgres" {
		t.Fatalf("expected postgres driver, got %q", globalConfig.Database.Driver)
	}
	if globalConfig.Database.Host != "localhost" || globalConfig.Database.Port != 5432 {
		t.Fatalf("unexpected postgres config: %+v", globalConfig.Database)
	}
}
