package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeTempConfig(t *testing.T, cfg map[string]interface{}) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "data"), 0755); err != nil {
		t.Fatalf("mkdir data: %v", err)
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	err = os.WriteFile(filepath.Join(dir, "data", "config.json"), data, 0644)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}
	return dir
}

func TestLoadConfigValid(t *testing.T) {
	dir := writeTempConfig(t, map[string]interface{}{
		"username":   "test",
		"password":   "pass",
		"secret_key": "key",
	})
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}
	if cfg.Username != "test" {
		t.Errorf("Username = %s, want test", cfg.Username)
	}
	if cfg.Database.Type != "sqlite3" {
		t.Errorf("Database.Type = %s, want sqlite3", cfg.Database.Type)
	}
	if cfg.Database.DBFile != "./treehole.db" {
		t.Errorf("Database.DBFile = %s, want ./treehole.db", cfg.Database.DBFile)
	}
}

func TestLoadConfigAllowsPartialAuthFields(t *testing.T) {
	dir := writeTempConfig(t, map[string]interface{}{
		"username":   "",
		"password":   "pass",
		"secret_key": "key",
	})
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}
	if cfg.HasPasswordLogin() {
		t.Fatal("HasPasswordLogin() = true, want false for missing username")
	}
}

func TestConfigAuthCapabilityHelpers(t *testing.T) {
	cfg := &Config{Username: "user", Password: "pass"}
	if !cfg.HasPasswordLogin() {
		t.Fatal("HasPasswordLogin() = false, want true")
	}
	if !cfg.HasAnyPasswordLoginInput() {
		t.Fatal("HasAnyPasswordLoginInput() = false, want true")
	}
	if cfg.HasTOTPSecret() {
		t.Fatal("HasTOTPSecret() = true, want false")
	}
	cfg.SecretKey = "secret"
	if !cfg.HasTOTPSecret() {
		t.Fatal("HasTOTPSecret() = false, want true")
	}
	cfg = &Config{Password: "pass"}
	if !cfg.HasAnyPasswordLoginInput() {
		t.Fatal("HasAnyPasswordLoginInput() = false, want true for password-only config")
	}
	if cfg.HasPasswordLogin() {
		t.Fatal("HasPasswordLogin() = true, want false for password-only config")
	}
}

func TestLoadConfigNoFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "data"), 0755); err != nil {
		t.Fatalf("mkdir data: %v", err)
	}
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}
	if cfg.Database.Type != "sqlite3" {
		t.Fatalf("Database.Type = %q, want sqlite3", cfg.Database.Type)
	}
	if cfg.Database.DBFile != "./treehole.db" {
		t.Fatalf("Database.DBFile = %q, want ./treehole.db", cfg.Database.DBFile)
	}
}

func TestEnsureRuntimeFilesCreatesDefaults(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	if err := EnsureRuntimeFiles(); err != nil {
		t.Fatalf("EnsureRuntimeFiles() error: %v", err)
	}

	configPath := filepath.Join(dir, "data", "config.json")
	configData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("stat data/config.json: %v", err)
	}
	var cfg Config
	if err := json.Unmarshal(configData, &cfg); err != nil {
		t.Fatalf("unmarshal data/config.json: %v", err)
	}
	if cfg.Database.Type != "sqlite3" || cfg.Database.DBFile != "./treehole.db" {
		t.Fatalf("default config = %+v", cfg.Database)
	}
	cookiesData, err := os.ReadFile(filepath.Join(dir, "data", "cookies.json"))
	if err != nil {
		t.Fatalf("read data/cookies.json: %v", err)
	}
	if string(cookiesData) != "[]\n" {
		t.Fatalf("cookies content = %q, want []\\n", string(cookiesData))
	}
}

func TestEnsureRuntimeFilesMigratesLegacyFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "data"), 0755); err != nil {
		t.Fatalf("mkdir data: %v", err)
	}
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("{\"username\":\"legacy-migrated\"}\n"), 0644); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cookies.json"), []byte("[{\"name\":\"token\"}]\n"), 0644); err != nil {
		t.Fatalf("write legacy cookies: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "crawler.log"), []byte("legacy log\n"), 0644); err != nil {
		t.Fatalf("write legacy crawler.log: %v", err)
	}

	if err := EnsureRuntimeFiles(); err != nil {
		t.Fatalf("EnsureRuntimeFiles() error: %v", err)
	}

	for _, name := range []string{"config.json", "cookies.json", "crawler.log"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Fatalf("legacy %s should be moved, stat err=%v", name, err)
		}
	}

	configData, err := os.ReadFile(filepath.Join(dir, "data", "config.json"))
	if err != nil {
		t.Fatalf("read migrated config: %v", err)
	}
	if string(configData) != "{\"username\":\"legacy-migrated\"}\n" {
		t.Fatalf("migrated config = %q", string(configData))
	}
	logData, err := os.ReadFile(filepath.Join(dir, "data", "crawler.log"))
	if err != nil {
		t.Fatalf("read migrated log: %v", err)
	}
	if string(logData) != "legacy log\n" {
		t.Fatalf("migrated log = %q", string(logData))
	}
}

func TestLoadConfigPostgresDefaults(t *testing.T) {
	dir := writeTempConfig(t, map[string]interface{}{
		"username":   "test",
		"password":   "pass",
		"secret_key": "key",
		"database": map[string]interface{}{
			"type":     "postgres",
			"user":     "myuser",
			"password": "mypass",
			"name":     "mydb",
		},
	})
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}
	if cfg.Database.Host != "localhost" {
		t.Errorf("Host = %s, want localhost", cfg.Database.Host)
	}
	if cfg.Database.Port != 5432 {
		t.Errorf("Port = %d, want 5432", cfg.Database.Port)
	}
	if cfg.Database.SSLMode != "disable" {
		t.Errorf("SSLMode = %s, want disable", cfg.Database.SSLMode)
	}
}

func TestGetDatabaseDSNSQLite(t *testing.T) {
	cfg := &Config{
		Username:  "test",
		Password:  "pass",
		SecretKey: "key",
		Database: DatabaseConfig{
			Type:   "sqlite3",
			DBFile: "./test.db",
		},
	}
	dsn, err := cfg.GetDatabaseDSN()
	if err != nil {
		t.Fatalf("GetDatabaseDSN() error: %v", err)
	}
	if dsn != "./test.db" {
		t.Errorf("DSN = %s, want ./test.db", dsn)
	}
}

func TestGetDatabaseDSNPostgres(t *testing.T) {
	cfg := &Config{
		Username:  "test",
		Password:  "pass",
		SecretKey: "key",
		Database: DatabaseConfig{
			Type:     "postgres",
			Host:     "localhost",
			Port:     5432,
			User:     "myuser",
			Password: "mypass",
			Name:     "mydb",
			SSLMode:  "disable",
		},
	}
	dsn, err := cfg.GetDatabaseDSN()
	if err != nil {
		t.Fatalf("GetDatabaseDSN() error: %v", err)
	}
	expected := "host=localhost port=5432 user=myuser password=mypass dbname=mydb sslmode=disable"
	if dsn != expected {
		t.Errorf("DSN = %s, want %s", dsn, expected)
	}
}

func TestGetDatabaseDSNPostgresMissingFields(t *testing.T) {
	cfg := &Config{
		Username:  "test",
		Password:  "pass",
		SecretKey: "key",
		Database: DatabaseConfig{
			Type: "postgres",
		},
	}
	_, err := cfg.GetDatabaseDSN()
	if err == nil {
		t.Fatal("GetDatabaseDSN() expected error for missing postgres fields")
	}
}

func TestGetDatabaseDSNCustomDSN(t *testing.T) {
	cfg := &Config{
		Username:  "test",
		Password:  "pass",
		SecretKey: "key",
		Database: DatabaseConfig{
			Type: "postgres",
			DSN:  "custom://connection-string",
		},
	}
	dsn, err := cfg.GetDatabaseDSN()
	if err != nil {
		t.Fatalf("GetDatabaseDSN() error: %v", err)
	}
	if dsn != "custom://connection-string" {
		t.Errorf("DSN = %s, want custom://connection-string", dsn)
	}
}

func TestGetDatabaseDSNUnsupportedType(t *testing.T) {
	cfg := &Config{
		Username:  "test",
		Password:  "pass",
		SecretKey: "key",
		Database: DatabaseConfig{
			Type: "mysql",
		},
	}
	_, err := cfg.GetDatabaseDSN()
	if err == nil {
		t.Fatal("GetDatabaseDSN() expected error for unsupported type")
	}
}

func TestGetDatabaseDSNSQLiteMissingFile(t *testing.T) {
	cfg := &Config{
		Username:  "test",
		Password:  "pass",
		SecretKey: "key",
		Database: DatabaseConfig{
			Type:   "sqlite3",
			DBFile: "",
		},
	}
	_, err := cfg.GetDatabaseDSN()
	if err == nil {
		t.Fatal("GetDatabaseDSN() expected error for empty DBFile")
	}
}
