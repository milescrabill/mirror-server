package config

import (
	"log"

	"github.com/kelseyhightower/envconfig"
)

// Config is config for perm
type Config struct {
	Debug         bool
	Port          int
	ForecastToken string `required:"true"`
	PlaidClientID string `required:"true"`
	PlaidSecret   string `required:"true"`
	NewsToken   	string `required:"true"`

	// for securecookie
	// generate w/ openssl rand -hex 12 or similar
	SecureCookieHashKey  string `required:"true"`
	SecureCookieBlockKey string `required:"true"`
}

// GetConfig gets Config from env
// vars are prefixed with MIRROR_ and are all caps
func GetConfig() (c Config) {
	err := envconfig.Process("mirror", &c)
	if err != nil {
		log.Fatal(err.Error())
	}
	return
}
