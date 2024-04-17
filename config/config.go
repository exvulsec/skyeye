package config

import (
	"fmt"
	"os"
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
	Postgresql PostgresqlConfig          `mapstructure:"postgresql" yaml:"postgresql"`
	ScanInfos  map[string]ScanInfoConfig `mapstructure:"scan_infos" yaml:"scanInfos"`
	HTTPServer HTTPServerConfig          `mapstructure:"http_server" yaml:"http_server"`
	ETL        ETLConfig                 `mapstructure:"etl" yaml:"etl"`
	Redis      RedisConfig               `mapstructure:"redis" yaml:"redis"`
}

type HTTPServerConfig struct {
	Host           string                     `mapstructure:"host" yaml:"host"`
	Port           int                        `mapstructure:"port" yaml:"port"`
	APIKey         string                     `mapstructure:"apikey" yaml:"apikey"`
	APIKeyForCMC   string                     `mapstructure:"apikey_for_cmc" yaml:"apikey_for_cmc"`
	ClientMaxConns int                        `mapstructure:"client_max_conns" yaml:"client_max_conns"`
	MultiEvmClient map[string]EvmClientConfig `mapstructure:"multi_evm_clients" yaml:"multi_evm_clients"`
	LogPath        string                     `mapstructure:"log_path" yaml:"log_path"`
}

type EvmClientConfig struct {
	ProviderURL string `mapstructure:"provider_url" yaml:"provider_url"`
}

type ScanInfoConfig struct {
	APIKeyString string   `mapstructure:"apikeys" yaml:"apikeys"`
	APIKeys      []string `mapstructure:"-" yaml:"-"`
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
	ProviderURL            string            `mapstructure:"provider_url" yaml:"provider_url"`
	Chain                  string            `mapstructure:"chain" yaml:"chain"`
	RetryTimes             int64             `mapstructure:"retry_times" yaml:"retry_times"`
	PreviousFile           string            `mapstructure:"previous_file" yaml:"previous_file"`
	FlashLoanFile          string            `mapstructure:"flash_loan_file" yaml:"flash_loan_file"`
	FilterContracts        string            `mapstructure:"filter_contracts" yaml:"filter_contracts"`
	ScoreAlertThreshold    int               `mapstructure:"score_alert_threshold" yaml:"score_alert_threshold"`
	AssetUSDAlertThreshold string            `mapstructure:"asset_usd_alert_threshold" yaml:"asset_usd_alert_threshold"`
	LogPath                string            `mapstructure:"log_path" yaml:"log_path"`
	SlackContractWebHook   string            `mapstructure:"slack_contract_webhook" yaml:"slack_contract_webhook"`
	SlackTransferWebHook   string            `mapstructure:"slack_transfer_webhook" yaml:"slack_transfer_webhook"`
	HeimdallServer         string            `mapstructure:"heidmall_server" yaml:"heimdall_server"`
	CGCAPIKey              string            `mapstructure:"cgc_apikey" yaml:"cgc_apikey"`
	LinkURLs               map[string]string `mapstructure:"link_urls" yaml:"link_urls"`
}

func init() {
	SetupConfig(os.Getenv("CONFIG_PATH"))
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

	for chain, value := range Conf.ScanInfos {
		if value.APIKeyString != "" {
			value.APIKeys = strings.Split(value.APIKeyString, ",")
			Conf.ScanInfos[chain] = value
		}
	}

	logrus.Info("read configuration file successfully")
}
