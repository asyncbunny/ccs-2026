package e2etest

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	ancclient "github.com/anon-org/anon/v4/client/client"
	"github.com/cosmos/cosmos-sdk/client"
	sdkquery "github.com/cosmos/cosmos-sdk/types/query"
	"os"

	"strings"
)

func StoreWasmCode(ctx context.Context, ancClient *ancclient.Client, wasmFile string) error {
	wasmCode, err := os.ReadFile(wasmFile) // #nosec G304
	if err != nil {
		return err
	}
	if strings.HasSuffix(wasmFile, "wasm") { // compress for gas limit
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		_, err = gz.Write(wasmCode)
		if err != nil {
			return err
		}
		err = gz.Close()
		if err != nil {
			return err
		}
		wasmCode = buf.Bytes()
	}

	storeMsg := &wasmtypes.MsgStoreCode{
		Sender:       ancClient.MustGetAddr(),
		WASMByteCode: wasmCode,
	}
	_, err = ancClient.ReliablySendMsg(ctx, storeMsg, nil, nil)
	if err != nil {
		return err
	}

	return nil
}

func InstantiateContract(ancClient *ancclient.Client, ctx context.Context, codeID uint64, initMsg []byte) error {
	instantiateMsg := &wasmtypes.MsgInstantiateContract{
		Sender: ancClient.MustGetAddr(),
		Admin:  ancClient.MustGetAddr(),
		CodeID: codeID,
		Label:  "cw",
		Msg:    initMsg,
		Funds:  nil,
	}

	_, err := ancClient.ReliablySendMsg(ctx, instantiateMsg, nil, nil)
	if err != nil {
		return err
	}

	return nil
}

func ListCodes(ctx context.Context, ancClient *ancclient.Client, pagination *sdkquery.PageRequest) (*wasmtypes.QueryCodesResponse, error) {
	clientCtx := client.Context{Client: ancClient.RPCClient}
	queryClient := wasmtypes.NewQueryClient(clientCtx)

	resp, err := queryClient.Codes(ctx, &wasmtypes.QueryCodesRequest{
		Pagination: pagination,
	})

	return resp, err
}

func GetLatestCodeID(ctx context.Context, ancClient *ancclient.Client) (uint64, error) {
	pagination := &sdkquery.PageRequest{
		Limit:   1,
		Reverse: true,
	}
	resp, err := ListCodes(ctx, ancClient, pagination)
	if err != nil {
		return 0, err
	}

	if len(resp.CodeInfos) == 0 {
		return 0, fmt.Errorf("no codes found")
	}

	return resp.CodeInfos[0].CodeID, nil
}

func ListContractsByCode(ctx context.Context, ancClient *ancclient.Client, codeID uint64, pagination *sdkquery.PageRequest) (*wasmtypes.QueryContractsByCodeResponse, error) {
	clientCtx := client.Context{Client: ancClient.RPCClient}
	queryClient := wasmtypes.NewQueryClient(clientCtx)

	resp, err := queryClient.ContractsByCode(ctx, &wasmtypes.QueryContractsByCodeRequest{
		CodeId:     codeID,
		Pagination: pagination,
	})

	return resp, err
}
