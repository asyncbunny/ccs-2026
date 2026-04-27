//nolint:revive
package types

import "github.com/anon-org/anon/v4/client/anonclient"

type TxResponse struct {
	TxHash string
	Events []anonclient.RelayerEvent
}
