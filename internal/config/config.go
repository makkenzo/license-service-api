package config

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	Log      LogConfig
	OIDC     OIDCConfig
}

type ServerConfig struct {
	Port           string        `mapstructure:"port"`
	ReadTimeout    time.Duration `mapstructure:"readTimeout"`
	WriteTimeout   time.Duration `mapstructure:"writeTimeout"`
	IdleTimeout    time.Duration `mapstructure:"idleTimeout"`
	ShutdownPeriod time.Duration `mapstructure:"shutdownPeriod"`
}

type DatabaseConfig struct {
	URL             string        `mapstructure:"url"`
	MaxOpenConns    int           `mapstructure:"maxOpenConns"`
	MaxIdleConns    int           `mapstructure:"maxIdleConns"`
	ConnMaxLifetime time.Duration `mapstructure:"connMaxLifetime"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type LogConfig struct {
	Level string `mapstructure:"level"`
}

type JWTConfig struct {
	SecretKey string        `mapstructure:"secretKey"`
	TokenTTL  time.Duration `mapstructure:"tokenTTL"`
}

type OIDCConfig struct {
	IssuerURL string `mapstructure:"issuerUrl"`
	ClientID  string `mapstructure:"clientId"`
}

func LoadConfig(configPath string) (*Config, error) {
	err := godotenv.Load()
	if err != nil {
		log.Println("Info: .env file not found or error loading it. Proceeding without it.")
	} else {
		log.Println("Info: Loaded environment variables from .env file")
	}

	viper.SetDefault("server.port", "8080")
	viper.SetDefault("server.readTimeout", 5*time.Second)
	viper.SetDefault("server.writeTimeout", 10*time.Second)
	viper.SetDefault("server.idleTimeout", 120*time.Second)
	viper.SetDefault("server.shutdownPeriod", 15*time.Second)

	viper.SetDefault("database.maxOpenConns", 25)
	viper.SetDefault("database.maxIdleConns", 25)
	viper.SetDefault("database.connMaxLifetime", 5*time.Minute)

	viper.SetDefault("redis.db", "0")

	viper.SetDefault("log.level", "info")

	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AllowEmptyEnv(true)

	if configPath != "" {
		viper.SetConfigFile(configPath)
		if err := viper.ReadInConfig(); err != nil {
			log.Printf("Warning: could not read config file: %s. Error: %v\n", configPath, err)
		} else {
			log.Printf("Info: Loaded configuration from file: %s\n", configPath)
		}
	}

	if err := viper.BindEnv("database.url", "DATABASE_URL"); err != nil {
		log.Printf("Warning: could not bind DATABASE_URL: %v\n", err)
	}
	if err := viper.BindEnv("redis.addr", "REDIS_ADDR"); err != nil {
		log.Printf("Warning: could not bind REDIS_ADDR: %v\n", err)
	}
	if err := viper.BindEnv("redis.password", "REDIS_PASSWORD"); err != nil {
		log.Printf("Warning: could not bind REDIS_PASSWORD: %v\n", err)
	}
	if err := viper.BindEnv("redis.db", "REDIS_DB"); err != nil {
		log.Printf("Warning: could not bind REDIS_DB: %v\n", err)
	}
	if err := viper.BindEnv("server.port", "SERVER_PORT"); err != nil {
		log.Printf("Warning: could not bind SERVER_PORT: %v\n", err)
	}
	if err := viper.BindEnv("log.level", "LOG_LEVEL"); err != nil {
		log.Printf("Warning: could not bind LOG_LEVEL: %v\n", err)
	}
	if err := viper.BindEnv("jwt.secretKey", "JWT_SECRET_KEY"); err != nil {
		log.Printf("Warning: could not bind JWT_SECRET_KEY: %v\n", err)
	}
	if err := viper.BindEnv("oidc.issuerUrl", "ZITADEL_ISSUER_URL"); err != nil {
		log.Printf("Warning: could not bind ZITADEL_ISSUER_URL: %v\n", err)
	}
	if err := viper.BindEnv("oidc.clientId", "ZITADEL_CLIENT_ID"); err != nil {
		log.Printf("Warning: could not bind ZITADEL_CLIENT_ID: %v\n", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal configuration: %w", err)
	}

	return &cfg, nil
}
