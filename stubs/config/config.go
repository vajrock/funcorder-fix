package config

// Config представляет конфигурацию приложения.
type Config struct {
	Server ServerConfig
}

// ServerConfig содержит настройки сервера.
type ServerConfig struct {
	AutoUpdateCRLAfterRevoke bool
}
