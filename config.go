package main

import (
	"net/url"
	"os"

	"github.com/joho/godotenv"
)

type (
	AppConfig struct {
		AppConfigBase
		Livekit  *LivekitConfig
		MongoURL *url.URL
		Database *DBConfig
		Redis    *RedisConfig
	}

	AppConfigBase struct {
		SeesionSecret string `validate:"required,ascii"`
		LocalDomain   string `validate:"required,hostname|hostname_port"`
		Environment   string `validate:"printascii"`
	}

	LivekitConfig struct {
		APIKey      string `validate:"required,ascii"`
		APISecret   string `validate:"required,ascii"`
		Host        string `validate:"required,hostname|hostname_port"`
		LocalDomain string `validate:"required,hostname|hostname_port"`
		URL         *url.URL
	}

	DBConfig struct {
		User     string `validate:"required,alphanum"`
		Password string `validate:"required,ascii"`
		Host     string `validare:"required,hostname_port"`
		Name     string `validate:"required,alphanum"`
	}

	RedisConfig struct {
		Host     string `validate:"required,hostname_port"`
		User     string `validate:"printascii"`
		Password string `validate:"printascii"`
	}
)

const (
	SESSION_NAME           = "session-id"
	SESSION_DATASTORE_NAME = "data"
)

func loadConfig(envname string) (*AppConfig, error) {
	if envname == "" {
		envname = "development"
	}

	// Loads environment variables in .env files if they exist
	localEnv := ".env." + envname + ".local"
	if _, err := os.Stat(localEnv); err == nil {
		if err := godotenv.Load(localEnv); err != nil {
			return nil, err
		}
	}
	if _, err := os.Stat(".env"); err == nil {
		if err := godotenv.Load(".env"); err != nil {
			return nil, err
		}
	}

	var appConf AppConfig

	// Setup base config
	basicConf := AppConfigBase{
		SeesionSecret: os.Getenv("SESSION_SECRET"),
		LocalDomain:   os.Getenv("LOCAL_DOMAIN"),
		Environment:   envname,
	}
	if basicConf.SeesionSecret == "" {
		basicConf.SeesionSecret = "dev"
	}
	if err := mainValidator.Struct(&basicConf); err != nil {
		return nil, err
	}
	appConf.AppConfigBase = basicConf

	// Setup MongoDB config
	dbconf := &DBConfig{
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASS"),
		Host:     os.Getenv("DB_HOST"),
		Name:     os.Getenv("DB_NAME"),
	}
	if err := mainValidator.Struct(dbconf); err != nil {
		return nil, err
	}
	appConf.Database = dbconf
	mongoURL := &url.URL{
		Scheme: "mongodb",
		User:   url.UserPassword(dbconf.User, dbconf.Password),
		Host:   dbconf.Host,
	}
	appConf.MongoURL = mongoURL

	// Setup Redis config
	redisConf := &RedisConfig{
		Host:     os.Getenv("REDIS_HOST"),
		User:     os.Getenv("REDIS_USER"),
		Password: os.Getenv("REDIS_PASS"),
	}
	if err := mainValidator.Struct(redisConf); err != nil {
		return nil, err
	}
	appConf.Redis = redisConf

	// Setup LiveKit config
	lkConf := &LivekitConfig{
		APIKey:      os.Getenv("LIVEKIT_API_KEY"),
		APISecret:   os.Getenv("LIVEKIT_API_SECRET"),
		Host:        os.Getenv("LIVEKIT_HOST"),
		LocalDomain: os.Getenv("LIVEKIT_LOCAL_DOMAIN"),
	}
	if err := mainValidator.Struct(lkConf); err != nil {
		return nil, err
	}
	lkURL := &url.URL{
		Scheme: "wss",
		Host:   lkConf.LocalDomain,
	}
	lkConf.URL = lkURL
	appConf.Livekit = lkConf

	return &appConf, nil
}
