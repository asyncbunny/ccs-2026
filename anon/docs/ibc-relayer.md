# IBC relaying guide

Anon uses [IBC](https://ibcprotocol.dev/)
(Inter-Blockchain Communication protocol) to enable cross-chain
communication with other IBC enabled chains. This guide focuses on the specific
configurations needed
when relaying with Anon, particularly around its unique unbonding period
mechanism.

> **Note**: Anon supports IBC Transfer and IBC-Wasm. It does not support ICA,
> ICQ, IBC Hooks, or IBC Middleware yet.

## Important Note on Anon's Unbonding Period

Anon has a unique unbonding mechanism that differs from standard Cosmos SDK
chains. The Anon Genesis chain disables the standard `x/staking` module and
wraps it with the [x/epoching module](../x/epoching/README.md) module,
introducing secure, fast unbonding
through Bitcoin timestamping.

> **Important**: The standard `x/staking` module's unbonding time parameter
> remains at the default 21 days, but **this value should be ignored** when
> configuring the relayer's trusting period.

1. **Epoched Staking Mechanism**:
    - All staking operations and voting power adjustments are processed at the
      last block of each epoch
    - The AppHash of the last block of each epoch is checkpointed onto the
      Bitcoin blockchain
      (this AppHash is derived from the entire execution trace prior to that
      block)
    - On Anon mainnet, each epoch spans 360 blocks (defined
      by `epoch_interval` parameter
      of [x/epoching module](../x/epoching/README.md)) with 10s block times,
      resulting in 1 hour epoch duration

2. **Finalization Process**:
    - After an epoch is timestamped on a Bitcoin block, it becomes finalized
      once the block reaches a certain depth
    - This is defined by the `checkpoint_finalization_timeout` parameter
      of [x/btccheckpoint module](../x/btccheckpoint/README.md)
    - Any unbonding requests from that checkpointed epoch are then matured
    - On Anon mainnet, the block must be 300-deep, and given Bitcoin's
      average block time of ~10 minutes, the average unbonding
      time is about 50 hours

3. **IBC Light Client Configuration**:
    - IBC light clients for Anon Genesis on other chains should have a lower
      trusting period
    - This is about 2/3 of the unbonding period, following standard IBC security
      practices
    - This configuration only affects light clients of Anon Genesis on other
      chains
    - The trusting period of other chains' light clients on Anon Genesis
      remains unaffected

Due to these unique characteristics, special attention is required when
configuring the relayer's trusting period and client refresh rate.

## Network-Specific Parameters

The values mentioned above are specific to Anon mainnet. For other networks (
testnet, etc.), you can retrieve these values using:

```bash
# Query epoch interval
anond query epoching params

# Query checkpoint finalization timeout
anond query btccheckpoint params
```

For RPC and LCD endpoints for different networks, refer to
the [Anon Networks Repository](https://github.com/anon-org/networks/tree/main/anc-test-6).

## Relayer Configuration

When setting up a relayer for Anon, pay special attention to these
parameters. The following values are specific to Anon mainnet:

1. **Trusting Period**: Should be set to approximately 2/3 of the network's
   unbonding period
    - On Anon mainnet, the unbonding period is ~50 hours (based on ~300 BTC
      blocks), therefore the trusting period should be set to ~33 hours

2. **Client Refresh Rate**: A higher refresh rate is recommended (1/5 of
   trusting period)
    - On Anon mainnet, this is ~6.6 hours

Example Hermes configuration:

```
[mode.clients]
refresh = true

[[chains]]
trusting_period = "33 hours"
client_refresh_rate = "1/5"
```

For complete setup instructions, including wallet configuration, connection
setup, and channel creation, refer to:

- [Celestia's IBC Relayer Guide](https://docs.celestia.org/how-to-guides/ibc-relayer)
- [Osmosis's Relayer Guide](https://docs.osmosis.zone/osmosis-core/relaying/relayer-guide)

## Monitoring and Maintenance

Regular monitoring of your IBC clients is crucial. For example, if using Hermes,
you can monitor the `client_updates_submitted_total` metric, which counts the
number of client update messages submitted between chains. This metric should
increase over time as your relayer submits updates to keep the IBC clients
synchronized. For detailed information about this metric as well as other
important metrics, refer
to [Hermes metrics documentation](https://hermes.informal.systems/documentation/telemetry/operators.html#what-is-the-overall-ibc-status-of-each-network).

## Handling Expired/Frozen IBC Clients

If an IBC client expires or becomes frozen, you'll need to submit a governance
proposal to recover the client. This proposal needs to be submitted on the chain
that maintains the light client of the counterparty chain.

For example, if you're relaying between Anon and another chain:

- If the light client of the other chain (maintained on Anon) expires, submit
  the proposal on Anon
- If the light client of Anon (maintained on the other chain) expires, submit
  the proposal on the other chain

For detailed steps on how to submit an IBC client recovery proposal, refer to
the [IBC Governance Proposals Guide](https://ibc.cosmos.network/main/ibc/proposals.html#steps).
For more information about submitting governance proposals on Anon, including
parameters and requirements, see
the [Anon Governance Guide](https://docs.anon.io/guides/governance/).
