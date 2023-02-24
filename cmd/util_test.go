package cmd

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/lotus/api"
	ltypes "github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/lotus/chain/types/ethtypes"
	"github.com/filecoin-project/venus/venus-shared/testutil"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/stretchr/testify/assert"
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

func TestLogsBloomMarshal(t *testing.T) {
	vReceipt := &types.EthTxReceipt{
		LogsBloom: types.EmptyEthBloom[:],
	}
	lReceipt := &api.EthTxReceipt{
		LogsBloom: ethtypes.EmptyEthBloom[:],
	}

	d, err := json.Marshal(vReceipt)
	assert.NoError(t, err)

	d2, err := json.Marshal(lReceipt)
	assert.NoError(t, err)

	fmt.Printf("%s \n %s\n", d, d2)
	assert.Equal(t, d, d2)
}
