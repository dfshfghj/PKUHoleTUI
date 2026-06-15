package config

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

var (
	Conf *Config
)

const (
	dataDirName     = "data"
	configFileName  = "config.json"
	cookiesFileName = "cookies.json"
	logFileName     = "crawler.log"
)

type runtimePaths struct {
	dataDir          string
	configPath       string
	cookiesPath      string
	logPath          string
	legacyConfig     string
	legacyCookies    string
	legacyCrawlerLog string
}

type DatabaseConfig struct {
	Type     string `json:"type"`     // "sqlite3" or "postgres"
	Host     string `json:"host"`     // PostgreSQL host
	Port     int    `json:"port"`     // PostgreSQL port
	User     string `json:"user"`     // PostgreSQL user
	Password string `json:"password"` // PostgreSQL password
	Name     string `json:"name"`     // PostgreSQL database name
	DBFile   string `json:"db_file"`  // SQLite file path
	SSLMode  string `json:"ssl_mode"` // PostgreSQL SSL mode
	DSN      string `json:"dsn"`      // Custom DSN (optional)
}

type CorsConfig struct {
	AllowOrigins []string `json:"allow_origins"`
	AllowMethods []string `json:"allow_methods"`
	AllowHeaders []string `json:"allow_headers"`
}

type Config struct {
	Username   string         `json:"username"`
	Password   string         `json:"password"`
	SecretKey  string         `json:"secret_key"`
	DeviceUUID string         `json:"device_uuid"` // 设备标识符，用于API请求的uuid header
	Database   DatabaseConfig `json:"database"`
	Cors       CorsConfig     `json:"cors"`
}

func resolveRuntimePaths() (runtimePaths, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return runtimePaths{}, fmt.Errorf("获取工作目录失败: %w", err)
	}

	dataDir := filepath.Join(currentDir, dataDirName)
	return runtimePaths{
		dataDir:          dataDir,
		configPath:       filepath.Join(dataDir, configFileName),
		cookiesPath:      filepath.Join(dataDir, cookiesFileName),
		logPath:          filepath.Join(dataDir, logFileName),
		legacyConfig:     filepath.Join(currentDir, configFileName),
		legacyCookies:    filepath.Join(currentDir, cookiesFileName),
		legacyCrawlerLog: filepath.Join(currentDir, logFileName),
	}, nil
}

func ConfigPath() (string, error) {
	paths, err := resolveRuntimePaths()
	if err != nil {
		return "", err
	}
	return paths.configPath, nil
}

func CookiesPath() (string, error) {
	paths, err := resolveRuntimePaths()
	if err != nil {
		return "", err
	}
	return paths.cookiesPath, nil
}

func LogPath() (string, error) {
	paths, err := resolveRuntimePaths()
	if err != nil {
		return "", err
	}
	return paths.logPath, nil
}

func EnsureRuntimeFiles() error {
	paths, err := resolveRuntimePaths()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(paths.dataDir, 0755); err != nil {
		return fmt.Errorf("创建 data 目录失败: %w", err)
	}

	if err := moveLegacyFile(paths.legacyConfig, paths.configPath); err != nil {
		return err
	}
	if err := moveLegacyFile(paths.legacyCookies, paths.cookiesPath); err != nil {
		return err
	}
	if err := moveLegacyFile(paths.legacyCrawlerLog, paths.logPath); err != nil {
		return err
	}

	if _, err := os.Stat(paths.configPath); errors.Is(err, os.ErrNotExist) {
		if err := writeDefaultConfig(paths.configPath); err != nil {
			return fmt.Errorf("初始化默认配置失败: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("检查配置文件失败: %w", err)
	}

	if _, err := os.Stat(paths.cookiesPath); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(paths.cookiesPath, []byte("[]\n"), 0644); err != nil {
			return fmt.Errorf("初始化 cookies 文件失败: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("检查 cookies 文件失败: %w", err)
	}

	return nil
}

func LoadConfig() (*Config, error) {
	if err := EnsureRuntimeFiles(); err != nil {
		return nil, err
	}
	configPath, err := ConfigPath()
	if err != nil {
		return nil, err
	}

	file, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var config Config
	err = json.NewDecoder(file).Decode(&config)
	if err != nil {
		return nil, err
	}

	// 设置默认数据库配置
	if config.Database.Type == "" {
		config.Database.Type = "sqlite3"
	}
	if config.Database.Type == "sqlite3" && config.Database.DBFile == "" {
		config.Database.DBFile = "./treehole.db"
	}
	if config.Database.Type == "postgres" {
		if config.Database.Host == "" {
			config.Database.Host = "localhost"
		}
		if config.Database.Port == 0 {
			config.Database.Port = 5432
		}
		if config.Database.SSLMode == "" {
			config.Database.SSLMode = "disable"
		}
	}

	// 生成并保存 device_uuid（如果为空）
	if config.DeviceUUID == "" {
		config.DeviceUUID = generateDeviceUUID()
		if err := saveDeviceUUID(configPath, config.DeviceUUID); err != nil {
			log.Printf("[Config] 保存 device_uuid 失败: %v", err)
		} else {
			log.Printf("[Config] 已自动生成 device_uuid: %s", config.DeviceUUID)
		}
	}

	Conf = &config

	return &config, nil
}

func (c *Config) HasPasswordLogin() bool {
	if c == nil {
		return false
	}
	return c.Username != "" && c.Password != ""
}

func (c *Config) HasAnyPasswordLoginInput() bool {
	if c == nil {
		return false
	}
	return c.Username != "" || c.Password != ""
}

func (c *Config) HasTOTPSecret() bool {
	if c == nil {
		return false
	}
	return c.SecretKey != ""
}

func DefaultConfig() Config {
	return Config{
		Username:   "",
		Password:   "",
		SecretKey:  "",
		DeviceUUID: "",
		Database: DatabaseConfig{
			Type:     "sqlite3",
			Host:     "localhost",
			Port:     5432,
			User:     "",
			Password: "",
			Name:     "",
			DBFile:   "./treehole.db",
			SSLMode:  "disable",
			DSN:      "",
		},
		Cors: CorsConfig{
			AllowOrigins: []string{"*"},
			AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization"},
		},
	}
}

func generateDeviceUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("Web_PKUHOLE_2.0.0_WEB_UUID_%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func saveDeviceUUID(configPath, uuid string) error {
	file, err := os.Open(configPath)
	if err != nil {
		return err
	}
	defer file.Close()

	var config map[string]interface{}
	if err := json.NewDecoder(file).Decode(&config); err != nil {
		return err
	}

	config["device_uuid"] = uuid

	data, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

func moveLegacyFile(src, dst string) error {
	if _, err := os.Stat(dst); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("检查目标文件失败 %s: %w", dst, err)
	}

	if _, err := os.Stat(src); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("检查旧文件失败 %s: %w", src, err)
	}

	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	if err := copyFileContents(src, dst); err != nil {
		return fmt.Errorf("迁移文件 %s -> %s 失败: %w", src, dst, err)
	}
	if err := os.Remove(src); err != nil {
		return fmt.Errorf("删除旧文件 %s 失败: %w", src, err)
	}
	return nil
}

func copyFileContents(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}

	if _, err := out.ReadFrom(in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func writeDefaultConfig(path string) error {
	data, err := json.MarshalIndent(DefaultConfig(), "", "    ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}

func (c *Config) GetDatabaseDSN() (string, error) {
	if c.Database.DSN != "" {
		return c.Database.DSN, nil
	}

	switch c.Database.Type {
	case "sqlite3":
		if c.Database.DBFile == "" {
			return "", fmt.Errorf("sqlite3 database file path is required")
		}
		return c.Database.DBFile, nil
	case "postgres":
		if c.Database.User == "" || c.Database.Password == "" || c.Database.Name == "" {
			return "", fmt.Errorf("postgres database requires user, password, and name")
		}
		return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			c.Database.Host, c.Database.Port, c.Database.User, c.Database.Password, c.Database.Name, c.Database.SSLMode), nil
	default:
		return "", fmt.Errorf("unsupported database type: %s", c.Database.Type)
	}
}
