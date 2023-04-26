package config

import (
	"fmt"
	"strings"

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
	HTTPServerConfig HTTPServerConfig `mapstructure:"http_server" yaml:"http_server"`
	ETLConfig        ETLConfig        `mapstructure:"etl" yaml:"etl"`
	RedisConfig      RedisConfig      `mapstructure:"redis" yaml:"redis"`
}

type HTTPServerConfig struct {
	Host                  string   `mapstructure:"host" yaml:"host"`
	Port                  int      `mapstructure:"port" yaml:"port"`
	APIKey                string   `mapstructure:"apikey" yaml:"apikey"`
	EtherScanAPIKeyString string   `mapstructure:"etherscan_apikeys" yaml:"etherscan_apikeys"`
	EtherScanAPIKeys      []string `mapstructure:"-" yaml:"-"`
	ClientMaxConns        int      `mapstructure:"client_max_conns" yaml:"client_max_conns"`
	AddressNonceThreshold uint64   `mapstructure:"address_nonce_threshold" yaml:"address_nonce_threshold"`
	ReadSolidityCode      bool     `mapstructure:"read_solidity_code" yaml:"read_solidity_code"`
}

type PostgresqlConfig struct {
	Host         string `mapstructure:"host" yaml:"host"`
	Port         int    `mapstructure:"port" yaml:"port"`
	User         string `mapstructure:"user" yaml:"user"`
	Password     string `mapstructure:"password" yaml:"password"`
	Database     string `mapstructure:"database" yaml:"database"`
	LogMode      bool   `mapstructure:"log_mode" yaml:"log_mode"`
	MaxIdleConns int    `mapstructure:"max_idle_conns" yaml:"max_idle_conns"`
	MaxOpenConns int    `mapstructure:"max_open_conns" yaml:"max_open_conns"`
}

type RedisConfig struct {
	Addr         string `mapstructure:"addr" yaml:"host"`
	Password     string `mapstructure:"port" yaml:"port"`
	Database     int    `mapstructure:"database" yaml:"database"`
	MaxIdleConns int    `mapstructure:"max_idle_conns" yaml:"max_idle_conns"`
}

type ETLConfig struct {
	ProviderURL  string `mapstructure:"provider_url" yaml:"provider_url"`
	Chain        string `mapstructure:"chain" yaml:"chain"`
	Worker       int64  `mapstructure:"worker" yaml:"worker"`
	PreviousFile string `mapstructure:"previous_file" yaml:"previous_file"`
}

func SetupConfig(configPath string) {
	if configPath != "" {
		viper.SetConfigFile(configPath)
	} else {
		if len(CfgPath) < 1 {
			panic(fmt.Errorf("failed to get config path %s", CfgPath))
		}

		viper.SetConfigName("config." + Env)
		viper.SetConfigType("yaml")
		viper.AddConfigPath(CfgPath)
	}

	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("failed to read configuration file: %v", err))
	}
	// load config info to global Config variable
	if err = viper.Unmarshal(&Conf); err != nil {
		panic(fmt.Errorf("failed to unmarshal configuration file %v", err))
	}

	if Conf.HTTPServerConfig.EtherScanAPIKeyString != "" {
		Conf.HTTPServerConfig.EtherScanAPIKeys = strings.Split(Conf.HTTPServerConfig.EtherScanAPIKeyString, ",")
	}

	logrus.Infof("read configuration file successfully")
}
