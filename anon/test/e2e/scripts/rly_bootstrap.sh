#!/bin/bash

set -ex

RELAYER_CONF_DIR=/root/.rly

# Initialize Cosmos relayer configuration
mkdir -p $RELAYER_CONF_DIR
rly --home $RELAYER_CONF_DIR config init
RELAYER_CONF=$RELAYER_CONF_DIR/config/config.yaml

# Setup Cosmos relayer configuration
cat <<EOF >$RELAYER_CONF
global:
    api-listen-addr: :5183
    timeout: 20s
    memo: ""
    light-cache-size: 10
chains:
    anc-a:
        type: cosmos
        value:
            key-directory: $RELAYER_CONF_DIR/keys/$ANC_A_E2E_CHAIN_ID
            key: val01-anc-a
            chain-id: $ANC_A_E2E_CHAIN_ID
            rpc-addr: http://$ANC_A_E2E_VAL_HOST:26657
            account-prefix: anc
            keyring-backend: test
            gas-adjustment: 1.5
            gas-prices: 0.002uanc
            min-gas-amount: 1
            debug: true
            timeout: 10s
            output-format: json
            sign-mode: direct
            extra-codecs: []
    anc-b:
        type: cosmos
        value:
            key-directory: $RELAYER_CONF_DIR/keys/$ANC_B_E2E_CHAIN_ID
            key: val01-anc-b
            chain-id: $ANC_B_E2E_CHAIN_ID
            rpc-addr: http://$ANC_B_E2E_VAL_HOST:26657
            account-prefix: anc
            keyring-backend: test
            gas-adjustment: 1.5
            gas-prices: 0.002uanc
            min-gas-amount: 1
            debug: true
            timeout: 10s
            output-format: json
            sign-mode: direct
            extra-codecs: []
paths:
    anca-ancb:
        src:
            chain-id: $ANC_A_E2E_CHAIN_ID
        dst:
            chain-id: $ANC_B_E2E_CHAIN_ID
EOF

# Import keys
rly -d --home $RELAYER_CONF_DIR keys restore anc-a val01-anc-a "$ANC_A_E2E_VAL_MNEMONIC"
rly -d --home $RELAYER_CONF_DIR keys restore anc-b val01-anc-b "$ANC_B_E2E_VAL_MNEMONIC"
sleep 3

# Start Cosmos relayer
echo "Creating IBC light clients, connection, and channel between the two CZs"
rly -d --home $RELAYER_CONF_DIR tx link anca-ancb --src-port ${CHAIN_A_IBC_PORT} --dst-port ${CHAIN_B_IBC_PORT} --order ordered --version zoneconcierge-1
echo "Created IBC channel successfully!"
sleep 10

rly -d --home $RELAYER_CONF_DIR start anca-ancb --debug-addr ""
