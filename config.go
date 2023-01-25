package main

import (
	"image"
	"image/png"
	"net/url"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

type (
	AppConfig struct {
		AppConfigBase
		Livekit  *LivekitConfig
		MongoURL *url.URL
		Database *DBConfig
		Redis    *RedisConfig
		Bot      *BotConfig
	}

	AppConfigBase struct {
		LocalDomain        string `validate:"required,fqdn"`
		Environment        string `validate:"printascii"`
		StorageDir         string
		LogoImageBlueBack  image.Image
		LogoImageWhiteBack image.Image
		LogoImageFront     image.Image
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

	BotConfig struct {
		Enable       bool
		Server       *url.URL
		ClientID     string
		ClientSecret string
		AccessToken  string
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
	storageDir, err := filepath.Abs("public/storage")
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(storageDir, 0775); err != nil {
		return nil, err
	}
	publicDir, _ := filepath.Abs("public")
	logoBlueBack, err := os.Open(filepath.Join(publicDir, "logo_back_blue.png"))
	if err != nil {
		return nil, err
	}
	defer logoBlueBack.Close()
	logoBlueBackPng, err := png.Decode(logoBlueBack)
	if err != nil {
		return nil, err
	}
	logoWhiteBack, err := os.Open(filepath.Join(publicDir, "logo_back_white.png"))
	if err != nil {
		return nil, err
	}
	defer logoWhiteBack.Close()
	logoWhiteBackPng, err := png.Decode(logoWhiteBack)
	if err != nil {
		return nil, err
	}
	logoFront, err := os.Open(filepath.Join(publicDir, "logo_front.png"))
	if err != nil {
		return nil, err
	}
	defer logoFront.Close()
	logoFrontPng, err := png.Decode(logoFront)
	if err != nil {
		return nil, err
	}

	basicConf := AppConfigBase{
		LocalDomain:        os.Getenv("LOCAL_DOMAIN"),
		Environment:        envname,
		StorageDir:         storageDir,
		LogoImageBlueBack:  logoBlueBackPng,
		LogoImageWhiteBack: logoWhiteBackPng,
		LogoImageFront:     logoFrontPng,
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

	// Setup Notification Bot config
	botHost := os.Getenv("BOT_SERVER")
	botConf := &BotConfig{
		Enable:       botHost != "",
		ClientID:     os.Getenv("BOT_CLIENT_ID"),
		ClientSecret: os.Getenv("BOT_CLIENT_SECRET"),
		AccessToken:  os.Getenv("BOT_ACCESS_TOKEN"),
	}
	if botConf.Enable {
		botConf.Server = &url.URL{
			Host:   botHost,
			Scheme: "https",
			Path:   "/",
		}
	}
	appConf.Bot = botConf

	return &appConf, nil
}
