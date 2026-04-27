package types_test

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	"github.com/anon-org/anon/v4/x/costaking/types"
)

func TestParamsValidate(t *testing.T) {
	tests := []struct {
		name   string
		params types.Params
		expErr error
	}{
		{
			name:   "valid params with default values",
			params: types.DefaultParams(),
			expErr: nil,
		},
		{
			name: "valid params with custom values",
			params: types.Params{
				CostakingPortion:    math.LegacyMustNewDecFromStr("0.5"),
				ScoreRatioBtcByNtk: math.NewInt(100),
				ValidatorsPortion:   math.LegacyMustNewDecFromStr("0.001"),
			},
			expErr: nil,
		},
		{
			name: "valid params with minimum values",
			params: types.Params{
				CostakingPortion:    math.LegacyNewDec(0),
				ScoreRatioBtcByNtk: math.OneInt(),
				ValidatorsPortion:   math.LegacyNewDec(0),
			},
			expErr: nil,
		},
		{
			name: "valid params with maximum costaking portion",
			params: types.Params{
				CostakingPortion:    math.LegacyMustNewDecFromStr("0.5"),
				ScoreRatioBtcByNtk: math.NewInt(50),
				ValidatorsPortion:   math.LegacyMustNewDecFromStr("0.4"),
			},
			expErr: nil,
		},
		{
			name: "nil costaking portion",
			params: types.Params{
				CostakingPortion:    math.LegacyDec{},
				ScoreRatioBtcByNtk: types.DefaultScoreRatioBtcByNtk,
				ValidatorsPortion:   types.DefaultValidatorsPortion,
			},
			expErr: types.ErrInvalidPercentage,
		},
		{
			name: "costaking portion equal to 1",
			params: types.Params{
				CostakingPortion:    math.LegacyOneDec(),
				ScoreRatioBtcByNtk: types.DefaultScoreRatioBtcByNtk,
				ValidatorsPortion:   types.DefaultValidatorsPortion,
			},
			expErr: types.ErrPercentageTooHigh,
		},
		{
			name: "costaking portion greater than 1",
			params: types.Params{
				CostakingPortion:    math.LegacyMustNewDecFromStr("1.5"),
				ScoreRatioBtcByNtk: types.DefaultScoreRatioBtcByNtk,
				ValidatorsPortion:   types.DefaultValidatorsPortion,
			},
			expErr: types.ErrPercentageTooHigh,
		},
		{
			name: "negative costaking portion",
			params: types.Params{
				CostakingPortion:    math.LegacyMustNewDecFromStr("-0.1"),
				ScoreRatioBtcByNtk: types.DefaultScoreRatioBtcByNtk,
				ValidatorsPortion:   types.DefaultValidatorsPortion,
			},
			expErr: types.ErrInvalidPercentage.Wrap("lower than zero"),
		},
		{
			name: "nil score ratio btc by ntk",
			params: types.Params{
				CostakingPortion:    types.DefaultCostakingPortion,
				ScoreRatioBtcByNtk: math.Int{},
				ValidatorsPortion:   types.DefaultValidatorsPortion,
			},
			expErr: types.ErrInvalidScoreRatioBtcByNtk,
		},
		{
			name: "score ratio btc by ntk less than 1",
			params: types.Params{
				CostakingPortion:    types.DefaultCostakingPortion,
				ScoreRatioBtcByNtk: math.ZeroInt(),
				ValidatorsPortion:   types.DefaultValidatorsPortion,
			},
			expErr: types.ErrScoreRatioTooLow,
		},
		{
			name: "negative score ratio btc by ntk",
			params: types.Params{
				CostakingPortion:    types.DefaultCostakingPortion,
				ScoreRatioBtcByNtk: math.NewInt(-10),
				ValidatorsPortion:   types.DefaultValidatorsPortion,
			},
			expErr: types.ErrScoreRatioTooLow,
		},
		{
			name: "both fields invalid",
			params: types.Params{
				CostakingPortion:    math.LegacyDec{},
				ScoreRatioBtcByNtk: math.Int{},
				ValidatorsPortion:   types.DefaultValidatorsPortion,
			},
			expErr: types.ErrInvalidPercentage,
		},
		{
			name: "nil validators portion",
			params: types.Params{
				CostakingPortion:    types.DefaultCostakingPortion,
				ScoreRatioBtcByNtk: types.DefaultScoreRatioBtcByNtk,
				ValidatorsPortion:   math.LegacyDec{},
			},
			expErr: types.ErrInvalidPercentage,
		},
		{
			name: "validators portion equal to 1",
			params: types.Params{
				CostakingPortion:    types.DefaultCostakingPortion,
				ScoreRatioBtcByNtk: types.DefaultScoreRatioBtcByNtk,
				ValidatorsPortion:   math.LegacyOneDec(),
			},
			expErr: types.ErrPercentageTooHigh,
		},
		{
			name: "validators portion greater than 1",
			params: types.Params{
				CostakingPortion:    types.DefaultCostakingPortion,
				ScoreRatioBtcByNtk: types.DefaultScoreRatioBtcByNtk,
				ValidatorsPortion:   math.LegacyMustNewDecFromStr("1.5"),
			},
			expErr: types.ErrPercentageTooHigh,
		},
		{
			name: "negative validators portion",
			params: types.Params{
				CostakingPortion:    types.DefaultCostakingPortion,
				ScoreRatioBtcByNtk: types.DefaultScoreRatioBtcByNtk,
				ValidatorsPortion:   math.LegacyMustNewDecFromStr("-0.01"),
			},
			expErr: types.ErrInvalidPercentage.Wrap("lower than zero"),
		},
		{
			name: "costaking + validators portion equal to 1",
			params: types.Params{
				CostakingPortion:    math.LegacyMustNewDecFromStr("0.5"),
				ScoreRatioBtcByNtk: types.DefaultScoreRatioBtcByNtk,
				ValidatorsPortion:   math.LegacyMustNewDecFromStr("0.5"),
			},
			expErr: types.ErrPercentageTooHigh,
		},
		{
			name: "costaking + validators portion greater than 1",
			params: types.Params{
				CostakingPortion:    math.LegacyMustNewDecFromStr("0.6"),
				ScoreRatioBtcByNtk: types.DefaultScoreRatioBtcByNtk,
				ValidatorsPortion:   math.LegacyMustNewDecFromStr("0.5"),
			},
			expErr: types.ErrPercentageTooHigh,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.params.Validate()

			if tc.expErr != nil {
				require.ErrorContains(t, err, tc.expErr.Error())
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestDefaultParams(t *testing.T) {
	params := types.DefaultParams()

	require.Equal(t, types.DefaultCostakingPortion, params.CostakingPortion)
	require.Equal(t, types.DefaultScoreRatioBtcByNtk, params.ScoreRatioBtcByNtk)
	require.Equal(t, types.DefaultValidatorsPortion, params.ValidatorsPortion)

	err := params.Validate()
	require.NoError(t, err)
}
