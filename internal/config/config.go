package config

type Config struct {
	AdminUser string
	AdminPass string
}

func Load() *Config {
	return &Config{
		AdminUser: "admin",
		AdminPass: "secret",
	}
}
