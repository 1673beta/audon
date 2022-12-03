package main

import (
	"fmt"
	"net/url"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type (
	AppConfig struct {
		AppConfigBase
		Livekit  *LivekitConfig
		MongoURL *url.URL
	}

	AppConfigBase struct {
		DBName        string `validate:"required,alphabum"`
		SeesionSecret string `validate:"required,ascii"`
		LocalDomain   string `validate:"required,hostname|hostname_port`
	}

	LivekitConfig struct {
		LivekitAPIKey    string `validate:"required,alphanum"`
		LivekitAPISecret string `validate:"required,alphanum"`
	}

	DBConfig struct {
		User     string `validate:"required,alphanum"`
		Password string `validate:"required,ascii"`
		Host     string `validare:"required,hostname"`
		Port     int    `validate:"required,gt=20000"`
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
	if err := godotenv.Load(".env"); err != nil {
		return nil, err
	}

	var appConf AppConfig

	// Setup base config
	basicConf := AppConfigBase{
		DBName:        os.Getenv("DB_NAME"),
		SeesionSecret: os.Getenv("SESSION_SECRET"),
		LocalDomain:   os.Getenv("LOCAL_DOMAIN"),
	}
	if basicConf.SeesionSecret == "" {
		basicConf.SeesionSecret = "dev"
	}
	if err := mainValidator.Struct(&basicConf); err != nil {
		return nil, err
	}
	appConf.AppConfigBase = basicConf

	// Setup MongoDB config
	dbport, err := strconv.Atoi(os.Getenv("DB_PORT"))
	if err != nil {
		return nil, err
	}
	dbconf := &DBConfig{
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASS"),
		Host:     os.Getenv("DB_HOST"),
		Port:     dbport,
	}
	if err := mainValidator.Struct(dbconf); err != nil {
		return nil, err
	}
	mongoURL := &url.URL{
		Scheme: "mongodb",
		User:   url.UserPassword(dbconf.User, dbconf.Password),
		Host:   fmt.Sprintf("%s:%d", dbconf.Host, dbconf.Port),
	}
	appConf.MongoURL = mongoURL

	// Setup LiveKit config
	lkConf := &LivekitConfig{
		LivekitAPIKey:    os.Getenv("LIVEKIT_API_KEY"),
		LivekitAPISecret: os.Getenv("LIVEKIT_API_SECRET"),
	}
	if err := mainValidator.Struct(lkConf); err != nil {
		return nil, err
	}
	appConf.Livekit = lkConf

	return &appConf, nil
}
