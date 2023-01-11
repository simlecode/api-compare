package cmd

import (
	"testing"

	"github.com/filecoin-project/go-state-types/abi"
	ltypes "github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/venus/venus-shared/testutil"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/stretchr/testify/require"
)

func TestEqual(t *testing.T) {
	var vmsg types.Message

	testutil.Provide(t, &vmsg)
	lmsg := ltypes.Message{
		Version:    vmsg.Version,
		To:         vmsg.To,
		From:       vmsg.From,
		Nonce:      vmsg.Nonce,
		Value:      vmsg.Value,
		GasLimit:   vmsg.GasLimit,
		GasFeeCap:  vmsg.GasFeeCap,
		GasPremium: vmsg.GasPremium,
		Method:     vmsg.Method,
		Params:     vmsg.Params,
	}

	require.True(t, equal(vmsg, lmsg))
	require.True(t, equal(&vmsg, &lmsg))
	require.True(t, equal([]*types.Message{&vmsg, &vmsg}, []*ltypes.Message{&lmsg, &lmsg}))

	vm := map[abi.MethodNum]*types.Message{}
	vm[vmsg.Method] = &vmsg
	lm := map[abi.MethodNum]*ltypes.Message{}
	lm[lmsg.Method] = &lmsg
	require.True(t, equal(vm, lm))
}
