# ETL Read Me


## ETL
### Quick Start
#### ETL Export Transaction
```shell
# export contract creation transaction
etl export txs --tx_nonce <nonce> \
    --creation_contract true \
    --config <config_path> \
    --batch_size <batch> \
    --chain <chain> \
    --openapi_server <openapi_server>
```
#### ETL HTTP Server
```shell
# shell
etl http --config <config_file_dir>

# docker
docker run -d -p 8080:8080 etl:<tag>

# docker-compose
doker-compose up -d

```

### 命令
#### Export
导出数据, 当前支持 txs

| 参数                  | 描述                                                       |
|---------------------|----------------------------------------------------------|
| tx_nonce            | 设置 transaction nonce 的值, 大于该值则会被过滤, 0 则不做过滤, 默认值为0       |
| creation_contract   | 只筛选出创建合约的交易, 默认为 false                                   |
| config | 设置 config 文件的路径, config 配置详见 [config](#config)           |
| batch_size | 使用 BatchCall 时,一个 Batch 中最大的对象数量, 默认为 50                 |
| chain | 指定链, 默认为 `ethereum`                                      |
| openapi_server | openapi_server 地址, 可使用 `etl http` 命令启动, 详见 [HTTP](#http) |




#### HTTP
启动 HTTP 服务

| 参数                  | 描述                                                       |
|---------------------|----------------------------------------------------------|
| config | 设置 config 文件的路径, config 配置详见 [config](#config)           |

### 从 RPC 批量获取数据
- 使用 `go-ethereum` RPC 的 `BatchElem` 构建了 `BatchCall` 用于批量从 `RPC Node` 上获取数据
- `BatchCall` 能批量获取的数据包括 `Block` 数据, `Transaction` 数据, `Receipt` 数据

### 区块
- 最新区块使用 `SubscribeNewHead`
- 具有区块记录功能,记录当前运行到哪个区块,下次启动时从已执行完区块的下一个区块开始运行
- 记录的区块高度写到文件上,文件可以在配置文件中配置


### 交易
- 从区块中获取的所有 `Transaction`
- 根据 `Transaction` 的 `Transaction Hash`, 从 `RPC Node` 获取到 `Transaction` 的 `Receipt` 数据

#### 过滤 Contract Creation 的交易
根据交易的 `To` 字段是否为空则去获取该交易的 `Receipt` 数据

##### 推送 Redis 的策略
根据如下策略判断是否需要过滤合约地址:
- **策略1: 过滤失败交易**
  - 根据 `Receipt` 数据后,查看 `Status` 是否为 `1`
  - 若为 `Status` 为 `1` 则过滤该交易
- **策略2: 过滤 Nonce 大于 10 的交易**
  - 根据 `Transaction Nonce` 是否小于指定的 `Threshold(暂定 10)`
  - 若 `Transaction Nonce` 大于 `10` 则过滤掉该合约地址

推送到 Redis `HSET` 的 `Key` 为:
- `<chain>:txs_associated:addrs`

推送到 Redis `HSET` 的 `Value` 为:
- `<from_address>`: `<contract_address>,<contract_address>,<contract_address>`

##### 推送 Nastiff 监控策略
根据如下策略判断是否需要过滤合约地址:
- **前置条件: 过滤失败交易**
  - 根据 `Receipt` 数据后,查看 `Status` 是否为 `1`
  - 若为 `Status` 为 `1` 则过滤该交易
- **策略1: 过滤 Nonce 大于 10 的交易**
  - 根据 `Transaction Nonce` 是否小于指定的 `Threshold(暂定 10)`
  - 若 `Transaction Nonce` 大于 `10` 则过滤掉该合约地址
- **策略2: 过滤 ByteCode < 500 的合约**
  - 根据合约的 `ByteCode` 大小, 判断 `ByteCode` Size除去 `0x` 是否小于 `500` 或为 `0`
  - 若 `ByteCode` Size 小于 `500` 或为 `0` 则过滤合约
- **策略3: 过滤合约为 Erc20 或 Erc721的合约**
  - 从 `RPC` 获取到该 `Contract Address` 的 `ByteCode`
  - 判断该 `ByteCode` 是否含有 `ERC20` 和 `ERC721` 的 `63{Signature Code}`
  - 若含有 `ERC20` 和 `ERC721` 的 `63{Signature Code}` 的特征则过滤掉该合约地址
- **策略4: 过滤开源合约**
  - 等待十分钟后, 向 `EtherScan` 请求该合约是否开源
  - 若合约已开源, 则过滤掉该合约地址

推送到 Redis MQ 的 `Key` 为:
- `<chain>:contract_address:stream:v2`

推送到 Redis MQ 的 `Value` 为:
```json
{
  "chain": <chain>,
  "txhash": <hash>,
  "contract": <contract_address>,
  "fund":  <depth> - <label>,
  "codeSize": <code size>,
  "push4": <push4 args>,
  "push20": <push20 args>,
}
```


## Config
```yaml
# http server 配置
http_server:
  # httt server ip
  host: localhost
  # http server 端口
  port: 8080
  # http api key, 设置后将通过 api key 访问 api
  apikey:
  # http 客户端最大的链接数
  client_max_conns: 5
  # scan 的 apikey, 调用 scan api 使用, 支持多个 apikey, 以 ',' 分割
  scan_apikeys:
  # 过滤 fund 源头地址时, 地址 transaction nonce 的阈值, 大于该值时停止查询
  address_nonce_threshold: 200000
  # solidity 合约源码的路径, 设置该值则会启用 /api/v1/address/:address/solidity 接口, 默认关闭
  solidity_code_path:

# 配置该值, 则会在使用 etl export txs 时导出到数据库
postgresql:
  user: postgres
  password: postgres
  database: postgres
  host: localhost
  port: 5432
  log_mode: true
  max_idle_conns: 5
  max_open_conns: 10

# etl 配置
etl:
  # rpc provider url
  provider_url:
  # 链名, 支持 ethereum
  chain: ethereum
  # 最大并发数
  worker: 10
  # 上一次执行到的区块高度的文件
  previous_file:
  # 中心化交易所列表, 与 label 前缀匹配时则, 过滤合约数据, 支持多个交易所前缀, 以 ',' 分割
  cex_list: Binance,Coinbase

# 配置该值, 则会在使用 etl export txs 时导出到 redis
redis:
  addr: localhost:6379
  database: 1
  max_idle_conns: 5
```