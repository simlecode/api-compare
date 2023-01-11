package cmd

import (
	"context"
	"fmt"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/lotus/chain/types/ethtypes"
	v1 "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
)

func newDataProvider(ctx context.Context, api v1.FullNode) (*dataProvider, error) {
	defaultMiner, err := address.NewFromString("t01000")
	if err != nil {
		return nil, err
	}
	return &dataProvider{
		ctx: ctx,
		api: api,
		dataSet: &dataSet{
			defaultMiner: defaultMiner,
		},
	}, nil
}

type dataProvider struct {
	ctx context.Context
	api v1.FullNode

	ts *types.TipSet

	dataSet *dataSet
}

type dataSet struct {
	defaultMiner address.Address

	blockMsgs []*types.Message
	senders   []address.Address
	ids       []address.Address
}

func (dp *dataProvider) reset(ts *types.TipSet) error {
	if ts.Len() == 0 {
		return fmt.Errorf("ts is empty")
	}
	dp.ts = ts

	return dp.generateData()
}

func (dp *dataProvider) generateData() error {
	blkMsgs, err := dp.api.ChainGetBlockMessages(dp.ctx, dp.ts.Blocks()[0].Cid())
	if err != nil {
		return err
	}
	msgLen := len(blkMsgs.Cids)
	ids := make(map[address.Address]struct{}, msgLen)
	senders := make(map[address.Address]struct{}, msgLen)
	msgs := make([]*types.Message, 0, msgLen)
	for _, msg := range blkMsgs.BlsMessages {
		msgs = append(msgs, msg)
		if msg.To.Protocol() == address.ID {
			ids[msg.To] = struct{}{}
		}
		senders[msg.From] = struct{}{}
	}
	for _, signedMsg := range blkMsgs.SecpkMessages {
		msg := signedMsg.Message
		msgs = append(msgs, &msg)
		if msg.To.Protocol() == address.ID {
			ids[msg.To] = struct{}{}
		}
		senders[msg.From] = struct{}{}
	}

	if len(ids) != 0 {
		dp.dataSet.ids = make([]address.Address, len(ids))
		for addr := range ids {
			dp.dataSet.ids = append(dp.dataSet.ids, addr)
		}
	}
	if len(senders) != 0 {
		dp.dataSet.senders = make([]address.Address, len(senders))
		for addr := range senders {
			dp.dataSet.senders = append(dp.dataSet.senders, addr)
		}
	}
	if len(msgs) != 0 {
		dp.dataSet.blockMsgs = msgs
	}

	return nil
}

func (dp *dataProvider) getMsgs() []*types.Message {
	return dp.dataSet.blockMsgs
}

func (dp *dataProvider) getMsg() *types.Message {
	if len(dp.dataSet.blockMsgs) > 0 {
		return dp.dataSet.blockMsgs[0]
	}
	return nil
}

func (dp *dataProvider) getSenders() []address.Address {
	return dp.dataSet.senders
}

func (dp *dataProvider) getSender() address.Address {
	if len(dp.dataSet.senders) > 0 {
		return dp.dataSet.senders[0]
	}

	return address.Undef
}

func (dp *dataProvider) getIDAddress() address.Address {
	if len(dp.dataSet.ids) > 0 {
		return dp.dataSet.ids[0]
	}

	return dp.defaultMiner()
}

func (dp *dataProvider) defaultMiner() address.Address {
	return dp.dataSet.defaultMiner
}

func (dp *dataProvider) getBlkOptByHeight() (string, error) {
	d, err := types.EthUint64(dp.ts.Height()).MarshalJSON()
	if err != nil {
		return "", err
	}

	return string(d), nil
}

func (dp *dataProvider) getBlockHash() (types.EthHash, ethtypes.EthHash, error) {
	c, err := dp.ts.Key().Cid()
	if err != nil {
		return emptyEthHash, emptyLEthHash, err
	}

	blkHash, err := types.NewEthHashFromCid(c)
	if err != nil {
		return emptyEthHash, emptyLEthHash, err
	}

	blkHash2, err := ethtypes.NewEthHashFromCid(c)
	if err != nil {
		return emptyEthHash, emptyLEthHash, err
	}

	return blkHash, blkHash2, nil
}

func (dp *dataProvider) getTxHash() (types.EthHash, ethtypes.EthHash, error) {
	msg := dp.getMsg()

	if msg != nil {
		msgHash, err := types.NewEthHashFromCid(msg.Cid())
		if err != nil {
			return emptyEthHash, emptyLEthHash, err
		}
		msgHash2, err := ethtypes.NewEthHashFromCid(msg.Cid())

		return msgHash, msgHash2, err
	}

	return emptyEthHash, emptyLEthHash, nil
}

func (dp *dataProvider) getEthAddress() (types.EthAddress, ethtypes.EthAddress, error) {
	addr, err := types.EthAddressFromFilecoinAddress(dp.defaultMiner())
	if err != nil {
		return emptyEthAddress, emptyLEthAddress, err
	}
	addr2, err := ethtypes.EthAddressFromFilecoinAddress(dp.defaultMiner())

	return addr, addr2, err
}
