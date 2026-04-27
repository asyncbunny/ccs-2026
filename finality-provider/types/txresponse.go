//nolint:revive
package types

import (
	"encoding/json"

	"github.com/anon-org/anon/v4/client/anonclient"
	"github.com/spf13/cobra"
)

// TxResponse handles the transaction response in the interface ConsumerController
// Not every consumer has Events thing in their response,
// so consumer client implementations need to care about Events field.
type TxResponse struct {
	TxHash string
	Events []anonclient.RelayerEvent
}

func NewAnonTxResponse(resp *anonclient.RelayerTxResponse) *anonclient.RelayerTxResponse {
	if resp == nil {
		return nil
	}

	events := make([]anonclient.RelayerEvent, len(resp.Events))
	for i, event := range resp.Events {
		events[i] = anonclient.RelayerEvent{
			EventType:  event.EventType,
			Attributes: event.Attributes,
		}
	}

	return &anonclient.RelayerTxResponse{
		Height:    resp.Height,
		TxHash:    resp.TxHash,
		Events:    events,
		Codespace: resp.Codespace,
		Code:      resp.Code,
		Data:      resp.Data,
	}
}

func PrintRespJSON(cmd *cobra.Command, resp interface{}) {
	jsonBytes, err := json.MarshalIndent(resp, "", "    ")
	if err != nil {
		cmd.Println("unable to marshal response: ", err)

		return
	}

	cmd.Printf("%s\n", jsonBytes)
}
