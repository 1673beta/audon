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
	}

	AppConfigBase struct {
		SeesionSecret string `validate:"required,ascii"`
		LocalDomain   string `validate:"required,hostname|hostname_port"`
		Environment   string `validate:"printascii"`
	}

	LivekitConfig struct {
		APIKey    string `validate:"required,ascii"`
		APISecret string `validate:"required,ascii"`
		Host      string `validate:"required,hostname_port"`
	}

	DBConfig struct {
		User     string `validate:"required,alphanum"`
		Password string `validate:"required,ascii"`
		Host     string `validare:"required,hostname_port"`
		Name     string `validate:"required,alphanum"`
	}
)

const (
	SESSION_NAME           = "session"
	SESSION_DATASTORE_NAME = "data"
)

func loadConfig(envname string) (*AppConfig, error) {
	if envname == "" {
		envname = "development"
	}

	// Set values in .env files to environment variables
	if err := godotenv.Load(".env." + envname); err != nil {
		return nil, err
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

	// Setup LiveKit config
	lkConf := &LivekitConfig{
		APIKey:    os.Getenv("LIVEKIT_API_KEY"),
		APISecret: os.Getenv("LIVEKIT_API_SECRET"),
		Host:      os.Getenv("LIVEKIT_HOST"),
	}
	if err := mainValidator.Struct(lkConf); err != nil {
		return nil, err
	}
	appConf.Livekit = lkConf

	return &appConf, nil
}
