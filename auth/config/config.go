package config

import (
	"fmt"
	"log"
	"strings"

	"github.com/lmnzx/slopify/pkg/logger"

	"github.com/spf13/viper"
)

type AuthServiceConfig struct {
	Name                  string `mapstructure:"name"`
	Version               string `mapstructure:"version"`
	RestServerAddress     string `mapstructure:"restserveraddress"`
	GrpcServerAddress     string `mapstructure:"grpcserveraddress"`
	AccountServiceAddress string `mapstructure:"accountserviceaddress"`
	Valkey                struct {
		User     string `mapstructure:"user"`
		Password string `mapstructure:"password"`
		Host     string `mapstructure:"host"`
		Port     string `mapstructure:"port"`
		DBNumber string `mapstructure:"dbnumber"`
	}
	Secrets struct {
		AccessTokenSecret  string `mapstructure:"accesstoken"`
		RefreshTokenSecret string `mapstructure:"refreshtoken"`
	}
	OtelCollectorURL string `mapstructure:"otelcollectorurl"`
}

func GetConfig() AuthServiceConfig {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./auth/config")

	viper.AutomaticEnv()
	viper.SetEnvPrefix("env")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	err := viper.ReadInConfig()
	if err != nil {
		log.Fatalf("Error reading config file, %s", err)
	}

	var config AuthServiceConfig
	err = viper.Unmarshal(&config)
	if err != nil {
		log.Fatalf("Unable to decode into struct, %v", err)
	}

	logger.SetServiceName(config.Name)

	return config
}

func (c *AuthServiceConfig) GetDBConnectionString() string {
	return fmt.Sprintf("valkey://%s:%s@%s:%s/%s",
		c.Valkey.User,
		c.Valkey.Password,
		c.Valkey.Host,
		c.Valkey.Port,
		c.Valkey.DBNumber)
}
