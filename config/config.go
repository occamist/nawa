package config

import (
	"github.com/caarlos0/env/v11"
)

type Config struct {
	JWTSecret     string `env:"JWT_SECRET,required"`
	CookieSecure  bool   `env:"COOKIE_SECURE" envDefault:"false"`
	AdminUsername string `env:"ADMIN_USERNAME" envDefault:"admin"`
	AdminPassword string `env:"ADMIN_PASSWORD"`
	Host          string `env:"HOST"`
	Port          string `env:"PORT" envDefault:"5555"`
}

func Load() (Config, error) {
	return env.ParseAs[Config]()
}
