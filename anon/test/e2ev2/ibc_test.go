package e2e2

import (
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	appparams "github.com/anon-org/anon/v4/app/params"
	"github.com/anon-org/anon/v4/test/e2ev2/tmanager"
	"github.com/anon-org/anon/v4/testutil/datagen"
	sdk "github.com/cosmos/cosmos-sdk/types"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	"github.com/stretchr/testify/require"
)

func TestIBCTransfer(t *testing.T) {
	t.Parallel()
	tm := tmanager.NewTmWithIbc(t)
	tm.Start()

	anc, csn := tm.ChainNodes()

	t.Log("Testing IBC transfer from CSN to ANC...")
	csnSender := csn.CreateWallet("csn_sender")
	csnSender.VerifySentTx = true

	ibcTransferCoin := sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(1_000000))
	senderCoins := sdk.NewCoins(ibcTransferCoin)

	csn.DefaultWallet().VerifySentTx = true
	csn.SendCoins(csnSender.Addr(), senderCoins.MulInt(sdkmath.NewInt(5)))

	csn.UpdateWalletAccSeqNumber(csnSender.KeyName)

	csnChannels := csn.QueryIBCChannels()
	csnChannel := csnChannels.Channels[0]
	ancRecipient := datagen.GenRandomAddress().String()

	csnSenderBalancesBefore := csn.QueryAllBalances(csnSender.Addr())
	ancRecipientBalancesBefore := anc.QueryAllBalances(ancRecipient)
	t.Logf("Before transfer - Sender balance: %s, Recipient balance: %s", csnSenderBalancesBefore.String(), ancRecipientBalancesBefore.String())

	ibcTxHash := csn.SendIBCTransfer(csnSender, ancRecipient, ibcTransferCoin, csnChannel.ChannelId, "test transfer")
	t.Logf("IBC transfer submitted successfully with tx hash: %s", ibcTxHash)

	// Compute the expected IBC denom on Anon side
	// When tokens are transferred from CSN to ANC, the denom gets prefixed with transfer/channel-X and latter hashed to ibc/
	hop := transfertypes.NewHop(csnChannel.Counterparty.PortId, csnChannel.Counterparty.ChannelId)
	denomTrace := transfertypes.NewDenom(ibcTransferCoin.Denom, hop)
	expectedIBCDenom := denomTrace.IBCDenom()
	t.Logf("Expected IBC denom on Anon: %s", expectedIBCDenom)

	// Wait for IBC transfer to complete and verify balance changes on both sides
	require.Eventually(t, func() bool {
		ancRecipientBalancesAfter := anc.QueryAllBalances(ancRecipient)
		expAfterBalances := ancRecipientBalancesBefore.Add(sdk.NewCoin(expectedIBCDenom, ibcTransferCoin.Amount))

		return ancRecipientBalancesAfter.Equal(expAfterBalances)
	}, 30*time.Second, 2*time.Second, "IBC transfer should complete within 30 seconds")

	csnSenderBalancesAfter := csn.QueryAllBalances(csnSender.Addr())
	ibcTxResp := csn.QueryTxByHash(ibcTxHash)

	fees := ibcTxResp.Tx.GetFee()
	expCsnSendBalances := csnSenderBalancesBefore.Sub(fees...).Sub(ibcTransferCoin)
	require.Equal(t, expCsnSendBalances.String(), csnSenderBalancesAfter.String(), "Sender should have %s, but has %s, fees %s, ibcTransferCoin: %s", expCsnSendBalances.String(), csnSenderBalancesAfter.String(), fees.String(), ibcTransferCoin.String())
}
