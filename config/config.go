package config

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var Conf = config{}

var (
	CfgPath string
	Env     string
)

type config struct {
	Postgresql       PostgresqlConfig `mapstructure:"postgresql" yaml:"postgresql"`
	HTTPServerConfig HTTPServerConfig `mapstructure:"httpserver" yaml:"httpserver"`
}

type HTTPServerConfig struct {
	Host string `mapstructure:"host" yaml:"host"`
	Port int    `mapstructure:"Port" yaml:"port"`
}

type PostgresqlConfig struct {
	User         string `mapstructure:"user" yaml:"user"`
	Password     string `mapstructure:"password" yaml:"password"`
	Database     string `mapstructure:"database" yaml:"database"`
	Host         string `mapstructure:"host" yaml:"host"`
	Port         int    `mapstructure:"port" yaml:"port"`
	LogMode      bool   `mapstructure:"log-mode" yaml:"log-mode"`
	MaxIdleConns int    `mapstructure:"max-idle-conns" yaml:"max-idle-conns"`
	MaxOpenConns int    `mapstructure:"max-open-conns" yaml:"max-open-conns"`
}

func SetupConfig() {
	if len(CfgPath) < 1 {
		panic(fmt.Errorf("failed to get config path %s", CfgPath))
	}

	viper.SetConfigName("config." + Env)
	viper.SetConfigType("yaml")
	viper.AddConfigPath(CfgPath)

	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("failed to read configuration file: %v", err))
	}
	// load config info to global Config variable
	if err = viper.Unmarshal(&Conf); err != nil {
		panic(fmt.Errorf("failed to unmarshal configuration file %v", err))
	}

	logrus.Infof("read configuration file successfully")
}
