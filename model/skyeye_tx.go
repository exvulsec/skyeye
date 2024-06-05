package model

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/sirupsen/logrus"
	"github.com/slack-go/slack"

	"github.com/exvulsec/skyeye/client"
	"github.com/exvulsec/skyeye/config"
	"github.com/exvulsec/skyeye/datastore"
	"github.com/exvulsec/skyeye/utils"
)

var skyEyeTableName = utils.ComposeTableName(datastore.SchemaPublic, datastore.TableSkyEyeTransaction)

const (
	Dedaub  = "Dedaub"
	Phalcon = "Phalcon"
)

type SkyEyeTransaction struct {
	ID                  *int64            `json:"id" gorm:"column:id"`
	Chain               string            `json:"chain" gorm:"column:chain"`
	BlockTimestamp      int64             `json:"block_timestamp" gorm:"column:block_timestamp"`
	BlockNumber         int64             `json:"block_number" gorm:"column:blknum"`
	TxHash              string            `json:"txhash" gorm:"column:txhash"`
	TxPos               int64             `json:"txpos" gorm:"column:txpos"`
	FromAddress         string            `json:"from_address" gorm:"column:from_address"`
	ContractAddress     string            `json:"contract_address" gorm:"column:contract_address"`
	MultiContracts      []string          `json:"multi_contracts" gorm:"-"`
	MultiContractString string            `json:"-" gorm:"column:multi_contracts"`
	Nonce               uint64            `json:"nonce" gorm:"column:nonce"`
	Score               int               `json:"score" gorm:"column:score"`
	Scores              []string          `json:"-" gorm:"-"`
	SplitScores         string            `json:"split_scores" gorm:"column:split_scores"`
	Values              []byte            `json:"-" gorm:"column:skyeye_values"`
	Trace               *TransactionTrace `json:"-" gorm:"-"`
	ByteCode            []byte            `json:"-" gorm:"-"`
	Push4Args           []string          `json:"-" gorm:"-"`
	Push20Args          []string          `json:"-" gorm:"-"`
	Push32Args          []string          `json:"-" gorm:"-"`
	PushStringLogs      []string          `json:"-" gorm:"-"`
	Fund                string            `json:"fund" gorm:"-"`
	MonitorAddrs        *MonitorAddrs     `json:"-" gorm:"-"`
	Skip                bool              `json:"-" gorm:"-"`
	Input               string            `json:"input" gorm:"-"`
}

func (st *SkyEyeTransaction) ConvertFromTransaction(tx Transaction) {
	st.Chain = config.Conf.ETL.Chain
	st.BlockTimestamp = tx.BlockTimestamp
	st.BlockNumber = tx.BlockNumber
	st.TxHash = tx.TxHash
	st.TxPos = tx.TxPos
	st.FromAddress = tx.FromAddress
	st.ContractAddress = tx.ContractAddress
	st.Nonce = tx.Nonce
	st.Trace = tx.Trace
}

func (st *SkyEyeTransaction) HasFlashLoan(flashLoanFuncNames []string) bool {
	for _, push4 := range st.Push4Args {
		for _, funcName := range flashLoanFuncNames {
			if push4 == funcName {
				return true
			}
		}
	}
	return false
}

func (st *SkyEyeTransaction) HasStart() bool {
	skipFuncNames := []string{"tokenName", "tokenSymbol"}
	for _, push4 := range st.Push4Args {
		for _, funcName := range skipFuncNames {
			if strings.EqualFold(push4, funcName) {
				return false
			}
		}
	}

	for _, push4 := range st.Push4Args {
		if strings.EqualFold(push4, "start") {
			return true
		}
	}
	return false
}

func (st *SkyEyeTransaction) HasRiskAddress(addrs []string) bool {
	for _, push20 := range st.Push20Args {
		for _, addr := range addrs {
			if push20 == addr {
				return true
			}
		}
	}
	return false
}

func (st *SkyEyeTransaction) ComposeSkyEyeTXValues() map[string]string {
	values := map[string]string{
		"Chain":      utils.ConvertChainToDeFiHackLabChain(config.Conf.ETL.Chain),
		"Block":      fmt.Sprintf("%d", st.BlockNumber),
		"TXhash":     st.TxHash,
		"CreateTime": fmt.Sprintf("%s UTC", time.Unix(st.BlockTimestamp, 0).Format(time.DateTime)),
		"Contract":   st.ContractAddress,
		"Deployer":   st.FromAddress,
		"Fund":       st.Fund,
	}

	keys := []string{"Func", "AddrLabels", "CodeSize", "Score", "SplitScores", "EmitLogs"}
	byteCodeValues := st.ComposeSkyEyeTXValuesFromByteCode()
	for _, key := range keys {
		value := byteCodeValues[key]
		values[key] = value
	}
	return values
}

func (st *SkyEyeTransaction) ComposeSkyEyeTXValuesFromByteCode() map[string]string {
	codeSize := 0
	if len(st.ByteCode) != 0 {
		codeSize = len(st.ByteCode[2:])
	}

	return map[string]string{
		"Func":        strings.Join(st.Push4Args, ","),
		"AddrLabels":  strings.Join(st.Push20Args, ","),
		"CodeSize":    fmt.Sprintf("%d", codeSize),
		"Score":       fmt.Sprintf("%d", st.Score),
		"SplitScores": st.SplitScores,
		"EmitLogs":    strings.Join(st.PushStringLogs, ","),
	}
}

func (st *SkyEyeTransaction) Insert() error {
	var err error
	if len(st.Values) == 0 {
		st.Values, err = json.Marshal(st.ComposeSkyEyeTXValues())
		if err != nil {
			return fmt.Errorf("marhsal nastiffValues is err %v", err)
		}
	}
	return datastore.DB().Table(skyEyeTableName).Create(st).Error
}

func (st *SkyEyeTransaction) GetInfoByContract(chain, contract string) error {
	return datastore.DB().Table(skyEyeTableName).Where("chain = ? AND contract_address = ?", chain, contract).Find(st).Error
}

func (st *SkyEyeTransaction) Analysis(chain string) {
	code, ok := utils.Retry(func() (any, error) {
		retryContextTimeout, retryCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer retryCancel()
		return client.MultiEvmClient()[chain].CodeAt(retryContextTimeout, common.HexToAddress(st.ContractAddress), big.NewInt(st.BlockNumber))
	}).([]byte)
	if ok {
		st.ByteCode = code
	}
	if st.analysisContractByPolicies() {
		st.Skip = true
		return
	}
	st.SplitScores = strings.Join(st.Scores, ",")
}

func (st *SkyEyeTransaction) alert() {
	if st.Score > config.Conf.ETL.ScoreAlertThreshold {
		if err := st.SendMessageToSlack(); err != nil {
			logrus.Errorf("send txhash %s's contract %s message to slack is err %v", st.TxHash, st.ContractAddress, err)
		}
		logrus.Infof("monitor contract %s on chain %s", st.ContractAddress, st.Chain)
		if err := st.MonitorContractAddress(); err != nil {
			logrus.Error(err)
			return
		}
		if err := st.Insert(); err != nil {
			logrus.Errorf("insert txhash %s's contract %s to %s is err %v", st.TxHash, st.ContractAddress, skyEyeTableName, err)
			return
		}
	}
}

func (st *SkyEyeTransaction) analysisContractByPolicies() bool {
	policies := []PolicyCalc{
		&ByteCodePolicyCalc{},
		&ContractTypePolicyCalc{},
		&Push4PolicyCalc{
			FlashLoanFuncNames: FuncNameList,
		},
		&Push20PolicyCalc{},
		&HeimdallPolicyCalc{},
	}
	for _, p := range policies {
		if p.Filter(st) {
			return true
		}
		score := p.Calc(st)
		st.Scores = append(st.Scores, fmt.Sprintf("%s: %d", p.Name(), score))
		st.Score += score
	}
	return false
}

func (st *SkyEyeTransaction) MonitorContractAddress() error {
	if st.MonitorAddrs != nil {
		now := time.Now()
		monitorAddr := MonitorAddr{
			Chain:       strings.ToLower(config.Conf.ETL.Chain),
			Address:     strings.ToLower(st.ContractAddress),
			Description: "SkyEye Monitor",
			CreatedTime: &now,
		}
		if err := monitorAddr.Create(); err != nil {
			return fmt.Errorf("create monitor address chain %s address %s is err %v", config.Conf.ETL.Chain, st.ContractAddress, err)
		}
		if !st.MonitorAddrs.Existed([]string{monitorAddr.Address}) {
			*st.MonitorAddrs = append(*st.MonitorAddrs, monitorAddr)
		}
	}
	return nil
}

func (st *SkyEyeTransaction) RemoveMonitorContractAddress() error {
	monitorAddr := MonitorAddr{
		Chain:   strings.ToLower(config.Conf.ETL.Chain),
		Address: strings.ToLower(st.ContractAddress),
	}
	if err := monitorAddr.Delete(); err != nil {
		return fmt.Errorf("remove monitor address on chain %s address %s is err %v", config.Conf.ETL.Chain, st.ContractAddress, err)
	}
	return nil
}

func (st *SkyEyeTransaction) ComposeMessage() string {
	chain := config.Conf.ETL.Chain
	scanURL := utils.GetScanURL(chain)
	text := fmt.Sprintf("*Chain:* `%s`\n", strings.ToUpper(chain))
	text += fmt.Sprintf("*Score:* `%d`\n", st.Score)
	text += fmt.Sprintf("*Funcs:* `%s`\n", strings.Join(st.Push4Args, ","))
	text += fmt.Sprintf("*Address Labels:* `%s`\n", strings.Join(st.Push20Args, ","))
	text += fmt.Sprintf("*Emit Logs:* `%s`\n", strings.Join(st.PushStringLogs, ","))
	text += fmt.Sprintf("*Block:* `%d`\n", st.BlockNumber)
	text += fmt.Sprintf("*TXhash:* <%s|%s>\n", fmt.Sprintf("%s/tx/%s", scanURL, st.TxHash), st.TxHash)
	text += fmt.Sprintf("*DateTime:* `%s UTC`\n", time.Unix(st.BlockTimestamp, 0).Format(time.DateTime))
	text += fmt.Sprintf("*Contract:* <%s|%s>\n", fmt.Sprintf("%s/address/%s", utils.GetScanURL(chain), st.ContractAddress), st.ContractAddress)
	text += fmt.Sprintf("*Fund:* `%s`\n", st.Fund)
	text += fmt.Sprintf("*Deployer:* <%s|%s>\n", fmt.Sprintf("%s/address/%s", utils.GetScanURL(chain), st.FromAddress), st.FromAddress)
	text += fmt.Sprintf("*CodeSize:* `%d`\n", len(st.ByteCode))
	text += fmt.Sprintf("*Split Scores:* `%s`\n", st.SplitScores)
	return text
}

func (st *SkyEyeTransaction) ComposeSlackAction() []slack.AttachmentAction {
	actions := []slack.AttachmentAction{}
	var actionURL string
	for key, url := range config.Conf.ETL.LinkURLs {
		switch {
		case strings.EqualFold(key, Dedaub):
			var dedaubMD5String DeDaubResponseString
			err := dedaubMD5String.GetCodeMD5(st.ByteCode)
			if err != nil {
				logrus.Errorf("get md5 for contract %s is err:", err)
				continue
			}
			actionURL = fmt.Sprintf(url, dedaubMD5String)
		case strings.EqualFold(key, Phalcon):
			actionURL = fmt.Sprintf(url, utils.ConvertChainToBlockSecChainID(config.Conf.ETL.Chain), st.TxHash)
		}
		actions = append(actions, slack.AttachmentAction{
			Name: utils.FirstUpper(key),
			Text: utils.FirstUpper(key),
			Type: "button",
			URL:  actionURL,
		})
	}
	return actions
}

func (st *SkyEyeTransaction) SendMessageToSlack() error {
	summary := fmt.Sprintf("⚠️Detected a suspected risk transactionon %s, score %d ⚠️\n", strings.ToUpper(config.Conf.ETL.Chain), st.Score)
	attachment := slack.Attachment{
		Color:      "warning",
		AuthorName: "EXVul",
		Fallback:   summary,
		Text:       summary + st.ComposeMessage(),
		Footer:     fmt.Sprintf("skyeye-on-%s", config.Conf.ETL.Chain),
		Ts:         json.Number(strconv.FormatInt(time.Now().Unix(), 10)),
		Actions:    st.ComposeSlackAction(),
	}
	msg := slack.WebhookMessage{
		Attachments: []slack.Attachment{attachment},
	}
	return slack.PostWebhook(config.Conf.ETL.SlackContractWebHook, &msg)
}
