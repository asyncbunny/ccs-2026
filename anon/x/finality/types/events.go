package types

import "github.com/anon-org/anon/v4/types"

func NewEventSlashedFinalityProvider(evidence *Evidence) *EventSlashedFinalityProvider {
	return &EventSlashedFinalityProvider{
		Evidence: evidence,
	}
}

func NewEventJailedFinalityProvider(fpPk *types.BIP340PubKey) *EventJailedFinalityProvider {
	return &EventJailedFinalityProvider{PublicKey: fpPk.MarshalHex()}
}
