# CoStaking Module

The CoStaking module enables users to earn aditional rewards by simultaneously
stake both Bitcoin (BTC) and Ntk tokens. This module tracks "costakers" who
have delegations in both systems.

## Quick Start

A **costaker** is a user who has:
1. **BTC delegations**: Active delegations to active finality providers via the
`x/btcstaking` module
2. **Ntk delegations**: Active delegations to validators via cosmos `x/staking`

The module dynamically tracks and updates costaker positions based on
delegation state changes through a comprehensive hook system.

## Key Components

### CostakerRewardsTracker

The core data structure tracking each costaker's position:

```go
// CostakerRewardsTracker represents the structure that holds information
// from the last time this staker withdraw the costaking rewards or modified
// his active staked amount of ntk or satoshis.
// The anon address of the staker is ommitted here but should be part of the
// key used to store this structure.
// Key: Prefix + costaker anon address.
type CostakerRewardsTracker struct {
  // StartPeriodCumulativeReward the starting period the costaker
  // made his last withdraw of costaking rewards or modified his active staking
  // amount of satoshis or ntk.
  StartPeriodCumulativeReward uint64 `protobuf:"varint,1,opt,name=start_period_cumulative_reward,json=startPeriodCumulativeReward,proto3" json:"start_period_cumulative_reward,omitempty"`
  // ActiveSatoshis is the total amount of active satoshi delegated
  // from this costaker anon address.
  ActiveSatoshis cosmossdk_io_math.Int `protobuf:"bytes,2,opt,name=active_satoshis,json=activeSatoshis,proto3,customtype=cosmossdk.io/math.Int" json:"active_satoshis"`
  // ActiveNtk is the total amount of active ntk delegated
  // from this costaker anon address.
  ActiveNtk cosmossdk_io_math.Int `protobuf:"bytes,3,opt,name=active_ntk,json=activeNtk,proto3,customtype=cosmossdk.io/math.Int" json:"active_ntk"`
  // TotalScore is the total amount of calculated score
  // of this costaker.
  TotalScore cosmossdk_io_math.Int `protobuf:"bytes,4,opt,name=total_score,json=totalScore,proto3,customtype=cosmossdk.io/math.Int" json:"total_score"`
}
```

### Hook System

The module responds to events from three other modules:

- **x/finality**: BTC delegation and finality providers lifecycle events
- **x/staking**: Ntk token delegation changes
- **x/incentive**: Triggers rewards withdraw

## Costaker State Logic

```mermaid
graph TD
    A[Costaker] --> B{Has BTC<br>Delegations?}
    A --> C{Has Ntk<br>Delegations?}

    B -->|Yes| D[BTC: hooks <br>x/finality]
    B -->|No| X[❌]

    C -->|No| X[❌]
    C -->|Yes| F[Ntk: hooks <br>x/staking]

    D --> J[🎯 CoStaker State<br/>Tracked by x/costaking]
    F --> J

    J --> M[CostakerRewardsTracker<br/>ActiveSatoshis + ActiveNtk + TotalScore]
    J --> N[Eligible for costaker rewards]

    style J fill:#e8f5e8,stroke:#4caf50,stroke-width:3px
    style M fill:#f3e5f5
    style N fill:#e1f5fe
```

## Hook Interactions

### x/finality Hooks

- `AfterBtcDelegationActivated`: Adds satoshis to costaker if the chosen fp was
in the active set.
- `AfterBtcDelegationUnbonded`: Removes satoshis from costaker if the chosen fp was active
in the previous anon block.
- `AfterAncFpEntersActiveSet`: Iterates over all the BTC delegations made for this fp and
add satoshi to the costaker structure.
- `AfterAncFpRemovedFromActiveSet`: Iterates over all the BTC delegations made for this fp and
removes satoshi in the costaker structure.

```mermaid
sequenceDiagram
    participant User
    participant BTCStaking as x/btcstaking
    participant Finality as x/finality
    participant CoStaking as x/costaking

    User->>BTCStaking: CreateBTCDelegation(fpAddr, delAddr, 1000 sats)

    BTCStaking->>Finality: BTCDelegationActivated Event
    Finality->>CoStaking: AfterBtcDelegationActivated(fpAddr, delAddr, prevActive, currActive, 1000)

    alt Fp in Active Set (prevActive=true)
      CoStaking->>CoStaking: costakerModified(delAddr, +1000 ActiveSatoshis)
      CoStaking-->>Finality:
    end

    User->>BTCStaking: UnbondDelegation
    BTCStaking->>Finality: BTCDelegationUnbonded Event
    Finality->>CoStaking: AfterBtcDelegationUnbonded(fpAddr, delAddr, prevActive, currActive, 1000)

    alt Fp Active in Prev AND Curr voting power distribution cache
      CoStaking->>CoStaking: costakerModified(delAddr, -1000 ActiveSatoshis)
      CoStaking-->>Finality:
    end

    Note over Finality, CoStaking: Fp Status Changes

    Finality->>CoStaking: AfterAncFpEntersActiveSet(fpAddr)
    loop For each BTC delegator to Fp
      CoStaking->>CoStaking: costakerModified(delegatorAddr, +sats ActiveSatoshis)
    end
    CoStaking-->>Finality:

    Finality->>CoStaking: AfterAncFpRemovedFromActiveSet(fpAddr)
    loop For each BTC delegator to Fp
      CoStaking->>CoStaking: costakerModified(delegatorAddr, -sats ActiveSatoshis)
    end
    CoStaking-->>Finality:
```

### x/staking Hooks

- `BeforeDelegationSharesModified`: Stores the amount of ntk staked for that
  validator in a map in memory to calculate the delta change.
- `AfterDelegationModified`: Updates Ntk token amount based on delegation
  delta change using the in memory cache.

```mermaid
sequenceDiagram
    participant User
    participant Staking as x/staking
    participant CoStaking as x/costaking
    participant Cache as Memory Cache

    User->>Staking: ModifyNtkDelegation(valAddr, 100→150 Ntk)

    Staking->>CoStaking: Hooks BeforeDelegationSharesModified(delAddr, valAddr)
    CoStaking->>Cache: setCacheStakedAmount(delAddr, valAddr, 100)
    Cache-->>CoStaking:
    CoStaking-->>Staking:


    Staking->>Staking: Process delegation modification

    Staking->>CoStaking: AfterDelegationModified(delAddr, valAddr)
    CoStaking->>Cache: GetCacheStakedAmount(delAddr, valAddr)
    Cache-->>CoStaking: cached: 100 tokens
    CoStaking->>CoStaking: Calculate delta: 150 - 100 = +50
    CoStaking->>CoStaking: costakerModified(delAddr, +50 ActiveNtk)
    CoStaking-->>Staking:
```

### x/incentive Hooks

- `BeforeRewardWithdraw`: Calculates and transfers the appropriate reward amounts from the
costaking module account to the incentive module account before reward distribution.

```mermaid
sequenceDiagram
    participant User
    participant Incentive as x/incentive
    participant CoStaking as x/costaking

    User->>Incentive: WithdrawRewards(COSTAKER, delAddr)

    Incentive->>CoStaking: Hooks BeforeRewardWithdraw(COSTAKER, delAddr)
    CoStaking->>CoStaking: costakerWithdrawRewards(delAddr)
    CoStaking-->>Incentive: Send funds from costaking <br>to incentive module account

    Incentive-->>User: Rewards distributed
```
