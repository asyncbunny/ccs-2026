package replay

import (
	"testing"

	anc "github.com/anon-org/anon/v4/types"
)

func (driver *AnonAppDriver) SendAndVerifyNDelegations(
	t *testing.T,
	staker *Staker,
	covSender *CovenantSender,
	keys []*anc.BIP340PubKey,
	n int,
) {
	for i := 0; i < n; i++ {
		staker.CreatePreApprovalDelegation(
			keys,
			1000,
			100000000,
		)
	}

	driver.GenerateNewBlockAssertExecutionSuccess()
	covSender.SendCovenantSignatures()
	driver.GenerateNewBlockAssertExecutionSuccess()
}
