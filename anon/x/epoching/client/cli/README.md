## CLI of the epoching module

### Delegating anc

```shell
$ANON_PATH/build/anond --home $TESTNET_PATH/node0/anond --chain-id chain-test \
         --keyring-backend test --fees 1anc \
         --from node0 --broadcast-mode block \
         tx epoching delegate <val_addr> <amount_of_anc>
```

### Undelegating anc

```shell
$ANON_PATH/build/anond --home $TESTNET_PATH/node0/anond --chain-id chain-test \
         --keyring-backend test --fees 3anc \
         --from node0 --broadcast-mode block \
         tx epoching unbond <val_addr> <amount_of_anc>
```

### Redelegating anc

```shell
$ANON_PATH/build/anond --home $TESTNET_PATH/node0/anond --chain-id chain-test \
         --keyring-backend test --fees 3anc \
         --from node0 --broadcast-mode block \
         tx epoching redelegate <from_val_addr> <to_val_addr> <amount_of_anc>
```
