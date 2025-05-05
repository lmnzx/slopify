package config

import (
	"fmt"
	"log"
	"strings"

	"github.com/lmnzx/slopify/pkg/logger"
	"github.com/spf13/viper"
)

type AccountServiceConfig struct {
	Name               string `mapstructure:"name"`
	Version            string `mapstructure:"version"`
	RestServerAddress  string `mapstructure:"restserveraddress"`
	GrpcServerAddress  string `mapstructure:"grpcserveraddress"`
	AuthServiceAddress string `mapstructure:"authserviceaddress"`
	Postgres           struct {
		User     string `mapstructure:"user"`
		Password string `mapstructure:"password"`
		Host     string `mapstructure:"host"`
		Port     string `mapstructure:"port"`
		DBName   string `mapstructure:"dbname"`
		SSL      bool   `mapstructure:"ssl"`
	}
	OtelCollectorURL string `mapstructure:"otelcollectorurl"`
}

func GetConfig() AccountServiceConfig {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./account/config")

	viper.AutomaticEnv()
	viper.SetEnvPrefix("env")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	err := viper.ReadInConfig()
	if err != nil {
		log.Fatalf("Error reading config file, %s", err)
	}

	var config AccountServiceConfig
	err = viper.Unmarshal(&config)
	if err != nil {
		log.Fatalf("Unable to decode into struct, %v", err)
	}

	logger.SetServiceName(config.Name)

	return config
}

func (c *AccountServiceConfig) GetDBConnectionString() string {
	sslMode := "disable"
	if c.Postgres.SSL {
		sslMode = "require"
	}

	return fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?sslmode=%s",
		c.Postgres.User,
		c.Postgres.Password,
		c.Postgres.Host,
		c.Postgres.Port,
		c.Postgres.DBName,
		sslMode)
}
