package cmd

import (
	"context"
	"testing"

	lapi "github.com/filecoin-project/lotus/api"
	ltypes "github.com/filecoin-project/lotus/chain/types"
	v1 "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
)

type fullNode struct {
	vAPI v1.FullNode
	lAPI lapi.FullNode
}

func TestSingleMethod(t *testing.T) {
	vToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoiYWRtaW4iLCJwZXJtIjoiYWRtaW4iLCJleHQiOiIifQ.6MY0durlQKAl6dNn4_MVRTcn1Bd34Ip_3aGXgEJVV2k"
	vURL := "/ip4/127.0.0.1/tcp/3453"
	lToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoiYWRtaW4iLCJwZXJtIjoiYWRtaW4iLCJleHQiOiIifQ.6MY0durlQKAl6dNn4_MVRTcn1Bd34Ip_3aGXgEJVV2k"
	lURL := "/ip4/127.0.0.1/tcp/1234"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	vAPI, vClose, err := v1.DialFullNodeRPC(ctx, vURL, vToken, nil)
	assert.NoError(t, err)
	defer vClose()

	lAPI, lClose, err := newLotusFullNodeRPCV1(ctx, lURL, lToken)
	assert.NoError(t, err)
	defer lClose()

	full := fullNode{
		vAPI: vAPI,
		lAPI: lAPI,
	}

	testStateCall(ctx, t, full)
}

func testStateCall(ctx context.Context, t *testing.T, full fullNode) {
	t.SkipNow()

	c, err := cid.Decode("bafy2bzacedrh52c7nucli6owxp6sygn66aj4lezs4ju4zruwl2rfpyli4budc")
	assert.NoError(t, err)
	msg, err := full.vAPI.ChainGetMessage(ctx, c)
	assert.NoError(t, err)

	vReplay, err := full.vAPI.StateCall(ctx, msg, types.EmptyTSK)
	assert.NoError(t, err)
	lReplay, err := full.lAPI.StateCall(ctx, toLotusMsg(msg), ltypes.EmptyTSK)
	assert.NoError(t, err)

	// Nonce may be different
	assert.Equal(t, vReplay.MsgCid, lReplay.MsgCid)
}
