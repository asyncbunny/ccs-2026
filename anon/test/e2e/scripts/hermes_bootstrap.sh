#!/bin/bash

set -ex

RELAYER_CONF_DIR=/root/.hermes

# Initialize Hermes relayer configuration
mkdir -p $RELAYER_CONF_DIR
RELAYER_CONF=$RELAYER_CONF_DIR/config.toml

echo $ANC_A_E2E_VAL_MNEMONIC >$RELAYER_CONF_DIR/ANC_A_MNEMONIC.txt
echo $ANC_B_E2E_VAL_MNEMONIC >$RELAYER_CONF_DIR/ANC_B_MNEMONIC.txt

# Setup Hermes relayer configuration
cat <<EOF >$RELAYER_CONF
[global]
log_level = 'debug'
[mode]
[mode.clients]
enabled = true
refresh = true
misbehaviour = true
[mode.connections]
enabled = false
[mode.channels]
enabled = true
[mode.packets]
enabled = true
clear_interval = 100
clear_on_start = true
tx_confirmation = true
[rest]
enabled = true
host = '0.0.0.0'
port = 3031
[telemetry]
enabled = true
host = '127.0.0.1'
port = 3001
[[chains]]
type = "CosmosSdk"
id = '$ANC_A_E2E_CHAIN_ID'
rpc_addr = 'http://$ANC_A_E2E_VAL_HOST:26657'
grpc_addr = 'http://$ANC_A_E2E_VAL_HOST:9090'
event_source = { mode = 'push', url = 'ws://$ANC_A_E2E_VAL_HOST:26657/websocket', batch_delay = '500ms' }
rpc_timeout = '10s'
account_prefix = 'anc'
key_name = 'val01-anc-a'
store_prefix = 'ibc'
max_gas = 50000000
gas_price = { price = 0.1, denom = 'uanc' }
gas_multiplier = 1.5
clock_drift = '1m' # to accommodate docker containers
trusting_period = '14days'
trust_threshold = { numerator = '1', denominator = '3' }
[[chains]]
type = "CosmosSdk"
id = '$ANC_B_E2E_CHAIN_ID'
rpc_addr = 'http://$ANC_B_E2E_VAL_HOST:26657'
grpc_addr = 'http://$ANC_B_E2E_VAL_HOST:9090'
event_source = { mode = 'push', url = 'ws://$ANC_B_E2E_VAL_HOST:26657/websocket', batch_delay = '500ms' }
rpc_timeout = '10s'
account_prefix = 'anc'
key_name = 'val01-anc-b'
store_prefix = 'ibc'
max_gas = 50000000
gas_price = { price = 0.1, denom = 'uanc' }
gas_multiplier = 1.5
clock_drift = '1m' # to accommodate docker containers
trusting_period = '14days'
trust_threshold = { numerator = '1', denominator = '3' }
EOF

# Import keys
hermes keys add --chain ${ANC_A_E2E_CHAIN_ID} --key-name "val01-anc-a" --mnemonic-file $RELAYER_CONF_DIR/ANC_A_MNEMONIC.txt
hermes keys add --chain ${ANC_B_E2E_CHAIN_ID} --key-name "val01-anc-b" --mnemonic-file $RELAYER_CONF_DIR/ANC_B_MNEMONIC.txt

# Start Hermes relayer
hermes start
