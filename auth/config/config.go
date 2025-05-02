package config

import (
	"fmt"
	"log"
	"strings"

	"github.com/lmnzx/slopify/pkg/middleware"
	"github.com/spf13/viper"
)

type AuthServiceConfig struct {
	Name                  string `mapstructure:"name"`
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

	middleware.SetServiceName(config.Name)

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
