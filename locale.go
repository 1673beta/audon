package main

import (
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
	yaml "gopkg.in/yaml.v3"
)

func initLocaleBundle() *i18n.Bundle {
	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("yaml", yaml.Unmarshal)
	bundle.LoadMessageFile("locales/active.ja.yaml")
	bundle.LoadMessageFile("locales/active.fr.yaml")

	return bundle
}
