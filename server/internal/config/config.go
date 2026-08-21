package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server struct {
		Port int    `yaml:"port"`
		Mode string `yaml:"mode"`
		// StaticDir 前端构建产物（dist）磁盘目录；目录存在时由 Go 托管前端，
		// 留空或目录不存在则纯 API 模式（前端可由 Nginx 独立托管）
		StaticDir string `yaml:"static_dir"`
	} `yaml:"server"`

	Database struct {
		DSN string `yaml:"dsn"`
	} `yaml:"database"`

	JWT struct {
		Secret      string `yaml:"secret"`
		ExpireHours int    `yaml:"expire_hours"`
	} `yaml:"jwt"`

	Casdoor struct {
		Enabled      bool   `yaml:"enabled"`
		Endpoint     string `yaml:"endpoint"`
		ClientID     string `yaml:"client_id"`
		ClientSecret string `yaml:"client_secret"`
		Organization string `yaml:"organization"`
		Application  string `yaml:"application"`
		RedirectURI  string `yaml:"redirect_uri"`
	} `yaml:"casdoor"`

	Feishu struct {
		Enabled   bool   `yaml:"enabled"`
		AppID     string `yaml:"app_id"`
		AppSecret string `yaml:"app_secret"`
	} `yaml:"feishu"`

	App struct {
		// 平台对外访问地址，用于飞书消息中的任务链接
		URL string `yaml:"url"`
	} `yaml:"app"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Server.StaticDir == "" {
		cfg.Server.StaticDir = "../web/dist"
	}
	if cfg.JWT.ExpireHours == 0 {
		cfg.JWT.ExpireHours = 168
	}
	if cfg.App.URL == "" {
		cfg.App.URL = fmt.Sprintf("http://localhost:%d", cfg.Server.Port)
	}
	return &cfg, nil
}
