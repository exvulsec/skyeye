# EXVUL on chain security monitor


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
docker-compose up -d
```

### Commands

#### Export

Exports data, currently supports `txs` (transactions).

| Parameter         | Description                                                                                |
|-------------------|--------------------------------------------------------------------------------------------|
| `tx_nonce`        | Sets the transaction nonce value; transactions with nonces greater than this value will be filtered. A value of 0 disables filtering. Default is 0. |
| `creation_contract` | Filters only contract creation transactions. Defaults to `false`.                                 |
| `config`          | Sets the path to the config file. See [config](#config) for config details.                   |
| `batch_size`      | When using BatchCall, the maximum number of objects in a batch. Defaults to 50.              |
| `chain`           | Specifies the chain. Defaults to `ethereum`.                                                   |
| `openapi_server`  | The address of the openapi server. Can be started with the `etl http` command. See [HTTP](#http). |

#### HTTP

Starts the HTTP service.

| Parameter | Description                                               |
|-----------|-----------------------------------------------------------|
| `config`  | Sets the path to the config file. See [config](#config) for config details. |

### Batch Data Retrieval from RPC

- Uses `BatchElem` from `go-ethereum` RPC to construct `BatchCall` for batch data retrieval from the `RPC Node`.
- `BatchCall` can batch retrieve data including `Block` data, `Transaction` data, and `Receipt` data.

### Blocks

- The latest block is obtained using `SubscribeNewHead`.
- Has block record functionality, recording which block it has currently processed, and starting from the next block after the last processed block on next startup.
- The recorded block height is written to a file, which can be configured in the config file.

### Transactions

- All `Transactions` are obtained from blocks.
- Based on the `Transaction Hash` of a `Transaction`, the `Receipt` data of the `Transaction` is obtained from the `RPC Node`.

#### Filtering Contract Creation Transactions

Checks if the `To` field of the transaction is empty to determine if the `Receipt` data for that transaction should be retrieved.

##### Redis Push Strategy

The following strategies are used to determine whether contract addresses need to be filtered:

- **Strategy 1: Filter Failed Transactions**
  - After obtaining the `Receipt` data, check if the `Status` is `1`.
  - If the `Status` is `1`, the transaction is filtered.
- **Strategy 2: Filter Transactions with Nonce Greater Than 10**
  - Check if the `Transaction Nonce` is less than the specified `Threshold` (currently set to 10).
  - If the `Transaction Nonce` is greater than `10`, the contract address is filtered out.

The `Key` for the `HSET` pushed to Redis is:

- `<chain>:txs_associated:addrs`

The `Value` for the `HSET` pushed to Redis is:

- `<from_address>`: `<contract_address>,<contract_address>,<contract_address>`

##### Nastiff Alerting Strategy

The following strategy is used to determine whether contract addresses need to be filtered:

- **Prerequisite: Filter Failed Transactions**
  - After obtaining the `Receipt` data, check if the `Status` is `1`.
- If the `Status` is `0`, the transaction is filtered.

Each contract will undergo independent score calculation. Currently, an alert is pushed when the score threshold is met (>= 50 points).

Score Settings

| Name          | Conditions       | Score            |
|---------------|-------------------|-------------------|
| nonce         | 0 <= nonce < 10   | 10 - nonce       |
|               | 10 <= nonce < 50  | 5 - (nonce-10)/10|
|               | 50 <= nonce       | 0                |
| bytecode      | 0 < bytecode < 500| 0                |
|               | bytecode >= 500   | 12               |
| isERC20/721   | isERC20/721      | 0                |
|               | ~isERC20/721     | 20               |
| push20        | len(push20) == 0  | 0                |
|               | len(push20) != 0  | 2                |
| push4 no flashloan| True             | 0                |
|               | False            | 50               |
| fund          | Tornado          | 40               |
|               | ChangeNow        | 13               |

The `Key` for the Redis MQ is:

- `evm:contract_address:stream`

An example of the `Value` pushed to the Redis MQ is:

```json
{
  "chain": "eth",
  "codeSize": 3218,
  "contract": "0xe911c2fd491931db7ee683ecb8ebb0b9a37332c5",
  "createTime": "2023-05-31 03:15:59",
  "func": "_buy,withdraw,withdrawToken,0x{1}",
  "fund": "2-KuCoin_0xcad6",
  "push20": "",
  "score": 61,
  "split_scores": [11, 12, 13, 25, 0, 0, 0],
  "txhash": "0x7464c7dad2859bec2f03f9f231fe63b66570499454fb1bc414d093eba67e98a3"
}
```

## Config

```yaml
# http server configuration
http_server:
  # http server ip
  host: localhost
  # http server port
  port: 8080
  # http api key, access to api will require api key if set
  apikey:
  # maximum number of connections for http client
  client_max_conns: 5
  # apikeys for scan, used for calling scan api, supports multiple apikeys separated by ','
  scan_apikeys:
  # Threshold of transaction nonce for filtering fund source addresses, querying stops when greater than this value
  address_nonce_threshold: 200000
  # Path to solidity contract source code, enable /api/v1/address/:address/solidity api if set, disabled by default
  solidity_code_path:

# Configure this to export to database when using etl export txs
postgresql:
  user: postgres
  password: postgres
  database: postgres
  host: localhost
  port: 5432
  log_mode: true
  max_idle_conns: 5
  max_open_conns: 10

# etl configuration
etl:
  # rpc provider url
  provider_url:
  # chain name, supports ethereum
  chain: ethereum
  # Maximum concurrency
  worker: 10
  # File of the block height processed last time
  previous_file:
  # File of all flash loan functions
  flash_loan_file: ./config/flashloan.txt
  # Score threshold for pushing monitoring alerts
  score_alert_threshold: 50

# Configure this to export to redis when using etl export txs
redis:
  addr: localhost:6379
  database: 1
  max_idle_conns: 5

