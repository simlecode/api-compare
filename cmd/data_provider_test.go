package cmd

import (
	"testing"

	"github.com/filecoin-project/venus/venus-shared/testutil"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/stretchr/testify/require"
)

func TestGetBlockHash(t *testing.T) {
	var ts types.TipSet
	testutil.Provide(t, &ts)
	dp := &dataProvider{
		currTS: &ts,
	}
	blkHash, blkHash2, err := dp.getBlockHash()

	require.NoError(t, err)
	require.Equal(t, blkHash.ToCid(), blkHash2.ToCid())
}
