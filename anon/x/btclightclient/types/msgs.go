package types

import (
	"encoding/hex"
	"fmt"
	"math/big"

	anc "github.com/anon-org/anon/v4/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ sdk.Msg = (*MsgInsertHeaders)(nil)

func NewMsgInsertHeaders(signer sdk.AccAddress, headersHex string) (*MsgInsertHeaders, error) {
	if len(headersHex) == 0 {
		return nil, fmt.Errorf("empty headers list")
	}

	decoded, err := hex.DecodeString(headersHex)

	if err != nil {
		return nil, err
	}

	if len(decoded)%anc.BTCHeaderLen != 0 {
		return nil, fmt.Errorf("invalid length of encoded headers: %d", len(decoded))
	}
	numOfHeaders := len(decoded) / anc.BTCHeaderLen
	headers := make([]anc.BTCHeaderBytes, numOfHeaders)

	for i := 0; i < numOfHeaders; i++ {
		headerSlice := decoded[i*anc.BTCHeaderLen : (i+1)*anc.BTCHeaderLen]
		headerBytes, err := anc.NewBTCHeaderBytesFromBytes(headerSlice)
		if err != nil {
			return nil, err
		}
		headers[i] = headerBytes
	}
	return &MsgInsertHeaders{Signer: signer.String(), Headers: headers}, nil
}

func (msg *MsgInsertHeaders) ValidateHeaders(powLimit *big.Int) error {
	// TODO: Limit number of headers in message?
	for _, header := range msg.Headers {
		err := anc.ValidateBTCHeader(header.ToBlockHeader(), powLimit)
		if err != nil {
			return err
		}
	}

	return nil
}

func (msg *MsgInsertHeaders) ReporterAddress() sdk.AccAddress {
	sender, err := sdk.AccAddressFromBech32(msg.Signer)
	if err != nil {
		panic(err)
	}
	return sender
}

func (msg *MsgInsertHeaders) ValidateStateless() error {
	_, err := sdk.AccAddressFromBech32(msg.Signer)

	if err != nil {
		return err
	}

	if len(msg.Headers) == 0 {
		return fmt.Errorf("empty headers list")
	}

	return nil
}

// ValidateBasic performs stateless validation on MsgInsertHeaders
func (msg *MsgInsertHeaders) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Signer)

	if err != nil {
		return err
	}

	if len(msg.Headers) == 0 {
		return fmt.Errorf("empty headers list")
	}

	return nil
}
