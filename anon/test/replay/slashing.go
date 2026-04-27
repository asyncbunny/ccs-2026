package replay

import (
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
)

func (d *AnonAppDriver) TxUnjailValidator(operator *SenderInfo, valAddr string) {
	msgUnjail := slashingtypes.NewMsgUnjail(valAddr)
	d.SendTxWithMessagesSuccess(d.t, operator, DefaultGasLimit, defaultFeeCoin, msgUnjail)
	operator.IncSeq()
}
