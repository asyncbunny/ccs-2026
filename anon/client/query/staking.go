package query

import (
	"context"

	"github.com/cosmos/cosmos-sdk/client"
	sdkquerytypes "github.com/cosmos/cosmos-sdk/types/query"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// QueryStaking queries the Staking module of the Anon node
// according to the given function
func (c *QueryClient) QueryStaking(f func(ctx context.Context, queryClient stakingtypes.QueryClient) error) error {
	ctx, cancel := c.getQueryContext()
	defer cancel()

	clientCtx := client.Context{Client: c.RPCClient}
	queryClient := stakingtypes.NewQueryClient(clientCtx)

	return f(ctx, queryClient)
}

// StakingParams queries staking module's parameters via ChainClient
func (c *QueryClient) StakingParams() (*stakingtypes.QueryParamsResponse, error) {
	var resp *stakingtypes.QueryParamsResponse
	err := c.QueryStaking(func(ctx context.Context, queryClient stakingtypes.QueryClient) error {
		var err error
		req := &stakingtypes.QueryParamsRequest{}
		resp, err = queryClient.Params(ctx, req)
		return err
	})

	return resp, err
}

// QueryNtkValidators queries the active ntk validators
func (c *QueryClient) QueryNtkValidators(pagination *sdkquerytypes.PageRequest, status stakingtypes.BondStatus) (*stakingtypes.QueryValidatorsResponse, error) {
	var resp *stakingtypes.QueryValidatorsResponse
	err := c.QueryStaking(func(ctx context.Context, queryClient stakingtypes.QueryClient) error {
		var err error
		req := &stakingtypes.QueryValidatorsRequest{
			Pagination: pagination,
			Status:     status.String(),
		}
		resp, err = queryClient.Validators(ctx, req)
		return err
	})

	return resp, err
}

// QueryNtkValidatorsBonded queries the bonded ntk validators
func (c *QueryClient) QueryNtkValidatorsBonded(pagination *sdkquerytypes.PageRequest) (*stakingtypes.QueryValidatorsResponse, error) {
	return c.QueryNtkValidators(pagination, stakingtypes.Bonded)
}

// QueryAllNtkValidatorsBonded queries all bonded validators by paginating through all pages
func (c *QueryClient) QueryAllNtkValidatorsBonded() ([]stakingtypes.Validator, error) {
	var allValidators []stakingtypes.Validator

	pagination := &sdkquerytypes.PageRequest{Limit: 100}
	for {
		resp, err := c.QueryNtkValidatorsBonded(pagination)
		if err != nil {
			return nil, err
		}

		allValidators = append(allValidators, resp.Validators...)

		if resp.Pagination == nil || resp.Pagination.NextKey == nil {
			break
		}
		pagination.Key = resp.Pagination.NextKey
	}

	return allValidators, nil
}
