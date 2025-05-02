package config

import (
	"fmt"
	"log"
	"strings"

	"github.com/lmnzx/slopify/pkg/middleware"
	"github.com/spf13/viper"
)

type ProductServiceConfig struct {
	Name               string `mapstructure:"name"`
	RestServerAddress  string `mapstructure:"restserveraddress"`
	AuthServiceAddress string `mapstructure:"authserviceaddress"`
	Postgres           struct {
		User     string `mapstructure:"user"`
		Password string `mapstructure:"password"`
		Host     string `mapstructure:"host"`
		Port     string `mapstructure:"port"`
		DBName   string `mapstructure:"dbname"`
		SSL      bool   `mapstructure:"ssl"`
	}
	Meilisearch struct {
		Url string `mapstructure:"url"`
		Key string `mapstructure:"key"`
	}
}

func GetConfig() ProductServiceConfig {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./product/config")

	viper.AutomaticEnv()
	viper.SetEnvPrefix("env")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	err := viper.ReadInConfig()
	if err != nil {
		log.Fatalf("Error reading config file, %s", err)
	}

	var config ProductServiceConfig
	err = viper.Unmarshal(&config)
	if err != nil {
		log.Fatalf("Unable to decode into struct, %v", err)
	}

	middleware.SetServiceName(config.Name)

	return config
}

func (c *ProductServiceConfig) GetDBConnectionString() string {
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
