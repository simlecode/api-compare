package cmd

import (
	"context"
	"fmt"
	"strings"

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

	currentTS *types.TipSet

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
	dp.currentTS = ts

	return dp.generateData()
}

func (dp *dataProvider) generateData() error {
	blk := dp.currentTS.Blocks()[0].Cid()
	blkMsgs, err := dp.api.ChainGetParentMessages(dp.ctx, blk)
	if err != nil {
		return err
	}
	receipts, err := dp.api.ChainGetParentReceipts(dp.ctx, blk)
	if err != nil {
		return err
	}
	if len(blkMsgs) != len(receipts) {
		return fmt.Errorf("block %s message not match receipts, %d %d", blk, len(blkMsgs), len(receipts))
	}
	msgLen := len(blkMsgs)
	ids := make(map[address.Address]struct{}, msgLen)
	senders := make(map[address.Address]struct{}, msgLen)
	msgs := make([]*types.Message, 0, msgLen)
	msgWithEventRoot := make([]*types.Message, 0)
	for i, msg := range blkMsgs {
		receipt := receipts[i]
		if receipt.ExitCode.IsError() {
			continue
		}
		if msg.Message.To.Protocol() == address.ID {
			ids[msg.Message.To] = struct{}{}
		}
		senders[msg.Message.From] = struct{}{}
		if receipt.EventsRoot != nil {
			msgWithEventRoot = append(msgWithEventRoot, msg.Message)
			continue
		}
		msgs = append(msgs, msg.Message)
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
		dp.dataSet.blockMsgs = append(msgWithEventRoot, msgs...)
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

// nolint
func (dp *dataProvider) getSenders() []address.Address {
	return dp.dataSet.senders
}

func (dp *dataProvider) getSender() address.Address {
	if len(dp.dataSet.senders) > 0 {
		return dp.dataSet.senders[0]
	}

	return address.Undef
}

// nolint
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
	d, err := types.EthUint64(dp.currentTS.Height()).MarshalJSON()
	if err != nil {
		return "", err
	}
	h := strings.Replace(string(d), "\"", "", -1)

	return h, nil
}

func (dp *dataProvider) getBlockHash() (types.EthHash, ethtypes.EthHash, error) {
	c, err := dp.currentTS.Key().Cid()
	if err != nil {
		return emptyEthHash, emptyLEthHash, err
	}

	blkHash, err := types.EthHashFromCid(c)
	if err != nil {
		return emptyEthHash, emptyLEthHash, err
	}

	blkHash2, err := ethtypes.EthHashFromCid(c)
	if err != nil {
		return emptyEthHash, emptyLEthHash, err
	}

	if blkHash != types.EthHash(blkHash2) {
		return emptyEthHash, emptyLEthHash, fmt.Errorf("parse block hash not match %v %v %v", c, blkHash, blkHash2)
	}
	if !blkHash.ToCid().Equals(c) {
		return emptyEthHash, emptyLEthHash, fmt.Errorf("blkHash.ToCid() not match %v %v", c, blkHash.ToCid())
	}

	return blkHash, blkHash2, nil
}

func (dp *dataProvider) getTxHash() (types.EthHash, ethtypes.EthHash, error) {
	msg := dp.getMsg()

	if msg != nil {
		msgHash, err := types.EthHashFromCid(msg.Cid())
		if err != nil {
			return emptyEthHash, emptyLEthHash, err
		}
		msgHash2, err := ethtypes.EthHashFromCid(msg.Cid())
		if err != nil {
			return emptyEthHash, emptyLEthHash, err
		}
		if msgHash != types.EthHash(msgHash2) {
			return emptyEthHash, emptyLEthHash, fmt.Errorf("msg hash not match %v %v %v", msgHash, msgHash2, msg.Cid())
		}

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
