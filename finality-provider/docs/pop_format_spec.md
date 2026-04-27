# Proof of Possession (PoP) Specification

## Overview

The Proof of Possession (PoP) structured specification outlined in this
document allows for the verification of the mutual ownership of a Anon
key pair and an EOTS key pair. In the following, we outline the five essential
attributes exposed by the `PoPExport` structure and provide examples and
validation procedures.

## Attributes

The `PoPExport` structure is defined bellow:

```go
// PoPExport the data needed to prove ownership of the eots and anon key pairs.
type PoPExport struct {
  // Btc public key is the EOTS PK *anctypes.BIP340PubKey marshal hex
  EotsPublicKey string `json:"eotsPublicKey"`
  // Anon public key is the *secp256k1.PubKey marshal hex
  NtkPublicKey string `json:"ntkPublicKey"`

  // Anon key pair signs EOTS public key as hex
  NtkSignEotsPk string `json:"ntkSignEotsPk"`
  // Schnorr signature of EOTS private key over the SHA256(Ntk address)
  EotsSignNtk string `json:"eotsSignNtk"`

  // Anon address ex.: anc1f04czxeqprn0s9fe7kdzqyde2e6nqj63dllwsm
  NtkAddress string `json:"ntkAddress"`
}
```

Detailed specification of each field:

- `EotsPublicKey`: The EOTS public key of the finality provider in hexadecimal format.
- `NtkPublicKey` – The Anon secp256k1 public key in base64 format.
- `NtkAddress` – The Anon account address (`anc` prefix). The address is
derived from the `NtkPublicKey` and used as the primary identifier on the
Anon network.
- `EotsSignNtk` – A Schnorr signature in base64 format, created by signing the
`sha256(NtkAddress)` with the EOTS private key.
- `NtkSignEotsPk` – A signature of the `EotsPublicKey`, created by the Anon
private key. This signature follows the Cosmos
[ADR-036](https://github.com/cosmos/cosmos-sdk/blob/main/docs/architecture/adr-036-arbitrary-signature.md)
specification and is encoded in base64.

## Example

Below is an example JSON representation of the PoPExport structure:

```json
{
  "eotsPublicKey": "3d0bebcbe800236ce8603c5bb1ab6c2af0932e947db4956a338f119797c37f1e",
  "ntkPublicKey": "A0V6yw74EdvoAWVauFqkH/GVM9YIpZitZf6bVEzG69tT",
  "ntkSignEotsPk": "AOoIG2cwC2IMiJL3OL0zLEIUY201X1qKumDr/1qDJ4oQvAp78W1nb5EnVasRPQ/XrKXqudUDnZFprLd0jaRJtQ==",
  "eotsSignNtk": "pR6vxgU0gXq+VqO+y7dHpZgHTz3zr5hdqXXh0WcWNkqUnRjHrizhYAHDMV8gh4vks4PqzKAIgZ779Wqwf5UrXQ==",
  "ntkAddress": "anc1f04czxeqprn0s9fe7kdzqyde2e6nqj63dllwsm"
}
```

## Validation

The function responsible for validating the `PoPExport` is `VerifyPopExport`,
which can be found [here](#).

`VerifyPopExport` ensures the authenticity and integrity of the `PoPExport`
by cross-verifying the provided signatures and public keys. This process
consists of two core validation steps:

- `ValidEotsSignNtk` – This function checks the validity of the Schnorr
signature `(EotsSignNtk)` by verifying that the EOTS private key has correctly
signed the SHA256 hash of the NTK address.
- `ValidNtkSignEots` – This function confirms that the NTK private key has
signed the EOTS public key `(EotsPublicKey)`, ensuring mutual validation
between the key pairs.

If both signatures pass verification, the export is deemed valid, confirming
that the finality provider holds both key pairs. This function plays a critical
role in maintaining trust and security in the finality provider's key
management process.
