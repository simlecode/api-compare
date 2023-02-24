package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"math"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/lotus/api"
	ltypes "github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/lotus/chain/types/ethtypes"
	"github.com/filecoin-project/venus/pkg/constants"
	v1 "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs/go-cid"
	"github.com/sirupsen/logrus"
)

const (
	methodPrefix = "Compare"
)

func newAPICompare(ctx context.Context,
	vAPI v1.FullNode,
	lAPI api.FullNode,
	dp *dataProvider,
	concurrency int,
) *apiCompare {
	if concurrency <= 0 {
		concurrency = 5
	}
	return &apiCompare{
		ctx:     ctx,
		vAPI:    vAPI,
		lAPI:    lAPI,
		dp:      dp,
		handler: newHandler(ctx, vAPI, lAPI, concurrency),
	}
}

type apiCompare struct {
	ctx context.Context

	vAPI v1.FullNode
	lAPI api.FullNode

	dp      *dataProvider
	handler *handler
}

func (ac *apiCompare) sendAndWait(methodName string, args []interface{}, opts ...reqOpt) error {
	req := newReq(methodName, args, opts...)
	ac.handler.send(req)

	select {
	case <-ac.ctx.Done():
		return ac.ctx.Err()
	case err := <-req.err:
		return err
	}
}

func (ac *apiCompare) CompareStateAccountKey() error {
	addr := ac.dp.getSender()
	if addr.Empty() {
		return nil
	}

	req := newReq(stateAccountKey, toInterface(addr, ac.dp.currTS.Key()))
	ac.handler.send(req)

	return <-req.err
}

func (ac *apiCompare) CompareChainGetTipSet() error {
	req := newReq(chainGetTipSet, []interface{}{ac.ctx, ac.dp.currTS.Key()})
	ac.handler.send(req)

	return <-req.err
}

func (ac *apiCompare) CompareChainGetTipSetByHeight() error {
	ts := ac.dp.currTS
	height := ts.Height() - 10
	key := ts.Key()

	req := newReq(chainGetTipSetByHeight, []interface{}{ac.ctx, height, key})
	ac.handler.send(req)
	err := <-req.err
	if err != nil {
		return err
	}

	// Too high
	req = newReq(chainGetTipSetByHeight, []interface{}{ac.ctx, height + 100, key}, withExpectCallAPIError())
	ac.handler.send(req)

	return <-req.err
}

func (ac *apiCompare) CompareStateGetRandomnessFromBeacon() error {
	per := crypto.DomainSeparationTag_TicketProduction
	randEpoch := ac.dp.currTS.Height()
	emtropy := []byte("fixed-randomness")
	key := ac.dp.currTS.Key()

	for per < crypto.DomainSeparationTag_PoStChainCommit {
		req := newReq(stateGetRandomnessFromBeacon, []interface{}{ac.ctx, per, randEpoch, emtropy, key})
		ac.handler.send(req)
		if err := <-req.err; err != nil {
			return err
		}
		per++
	}

	return nil
}

func (ac *apiCompare) CompareStateGetBeaconEntry() error {
	height := ac.dp.currTS.Height()

	req := newReq(stateGetBeaconEntry, []interface{}{ac.ctx, height})
	ac.handler.send(req)

	return <-req.err
}

func (ac *apiCompare) CompareChainGetBlock() error {
	for _, blk := range ac.dp.currTS.Blocks() {
		req := newReq(chainGetBlock, []interface{}{ac.ctx, blk.Cid()})
		ac.handler.send(req)
		if err := <-req.err; err != nil {
			return fmt.Errorf("block: %s, error: %v", blk.Cid(), err)
		}
	}

	return nil
}

func (ac *apiCompare) CompareChainGetBlockMessages() error {
	for _, blk := range ac.dp.currTS.Blocks() {
		req := newReq(chainGetBlockMessages, []interface{}{ac.ctx, blk.Cid()}, withResultCheck(func(r1, r2 interface{}) error {
			// todo: compare all
			msgs, _ := r1.(*types.BlockMessages)
			msgs2, _ := r2.(*api.BlockMessages)
			cidMap := make(map[cid.Cid]struct{}, len(msgs.Cids))
			for _, c := range msgs.Cids {
				cidMap[c] = struct{}{}
			}
			for _, c := range msgs2.Cids {
				if _, ok := cidMap[c]; !ok {
					d, d2, _ := toJSON(r1, r2)
					return fmt.Errorf("not match %v != %v", d, d2)
				}
			}

			return nil
		}))
		ac.handler.send(req)
		if err := <-req.err; err != nil {
			return fmt.Errorf("block: %v, error %v", blk.Cid(), err)
		}
	}

	return nil
}

func (ac *apiCompare) CompareChainGetMessage() error {
	for _, blk := range ac.dp.currTS.Blocks() {
		blkMsgs, err := ac.vAPI.ChainGetBlockMessages(ac.ctx, blk.Cid())
		if err != nil {
			return fmt.Errorf("failed to get block %s messages: %v", blk.Cid(), err)
		}

		for _, msgCID := range blkMsgs.Cids {
			req := newReq(chainGetMessage, []interface{}{ac.ctx, msgCID}, withResultCheck(resultCheckWithEqual))
			ac.handler.send(req)

			if err := <-req.err; err != nil {
				return fmt.Errorf("msg: %s, error: %v", msgCID, err)
			}
		}
	}

	return nil
}

func (ac *apiCompare) CompareChainGetMessagesInTipset() error {
	key := ac.dp.currTS.Key()

	req := newReq(chainGetMessagesInTipset, []interface{}{ac.ctx, key}, withResultCheck(resultCheckWithEqual))
	ac.handler.send(req)
	if err := <-req.err; err != nil {
		return err
	}

	return nil
}

func (ac *apiCompare) CompareChainGetParentMessages() error {
	for _, blkCID := range ac.dp.currTS.Cids() {
		req := newReq(chainGetParentMessages, []interface{}{ac.ctx, blkCID}, withResultCheck(resultCheckWithEqual))
		ac.handler.send(req)
		if err := <-req.err; err != nil {
			return fmt.Errorf("block: %s, error: %v", blkCID, err)
		}
	}

	return nil
}

func (ac *apiCompare) CompareChainGetParentReceipts() error {
	for _, blkCID := range ac.dp.currTS.Cids() {
		req := newReq(chainGetParentReceipts, toInterface(ac.ctx, blkCID))
		ac.handler.send(req)
		if err := <-req.err; err != nil {
			return fmt.Errorf("block: %s, error: %v", blkCID, err)
		}
	}

	return nil
}

func (ac *apiCompare) CompareStateVerifiedRegistryRootKey() error {
	key := ac.dp.currTS.Key()
	req := newReq(stateVerifiedRegistryRootKey, toInterface(ac.ctx, key))
	ac.handler.send(req)

	return <-req.err
}

func (ac *apiCompare) CompareStateVerifierStatus() error {
	key := ac.dp.currTS.Key()
	req := newReq(stateVerifierStatus, toInterface(ac.ctx, ac.dp.defaultMiner(), key), withResultCheck(func(r1, r2 interface{}) error {
		o1, _ := r1.(*big.Int)
		o2, _ := r2.(*big.Int)
		return bigIntEqual(o1, o2)
	}))
	ac.handler.send(req)

	return <-req.err
}

func (ac *apiCompare) CompareStateNetworkName() error {
	req := newReq(stateNetworkName, toInterface(ac.ctx))
	ac.handler.send(req)

	return <-req.err
}

func (ac *apiCompare) CompareSearchWaitMessage() error {
	key := ac.dp.currTS.Key()
	searchMsg := func(msgCID cid.Cid) error {
		req := newReq(stateSearchMsg, toInterface(ac.ctx, key, msgCID, constants.LookbackNoLimit, true))
		ac.handler.send(req)

		return <-req.err
	}

	waitMsg := func(msgCID cid.Cid) error {
		req := newReq(stateWaitMsg, toInterface(ac.ctx, msgCID, constants.DefaultConfidence, constants.LookbackNoLimit, true))
		ac.handler.send(req)

		return <-req.err
	}

	for i, msg := range ac.dp.getMsgs() {
		if i >= 5 {
			break
		}
		if err := searchMsg(msg.Cid()); err != nil {
			return err
		}
		if err := waitMsg(msg.Cid()); err != nil {
			return err
		}
	}

	return nil
}

func (ac *apiCompare) CompareStateNetworkVersion() error {
	key := ac.dp.currTS.Key()
	req := newReq(stateNetworkVersion, toInterface(ac.ctx, key))
	ac.handler.send(req)

	return <-req.err
}

func (ac *apiCompare) CompareChainGetPath() error {
	ts := ac.dp.currTS
	from, err := ac.vAPI.ChainGetTipSetAfterHeight(ac.ctx, ts.Height()-5, ts.Key())
	if err != nil {
		return err
	}

	req := newReq(chainGetPath, toInterface(ac.ctx, from.Key(), ts.Key()))
	ac.handler.send(req)

	return <-req.err
}

func (ac *apiCompare) CompareStateGetNetworkParams() error {
	req := newReq(stateGetNetworkParams, toInterface(ac.ctx), withResultCheck(resultCheckWithEqual))
	ac.handler.send(req)

	return <-req.err
}

func (ac *apiCompare) CompareStateActorCodeCIDs() error {
	req := newReq(stateActorCodeCIDs, toInterface(ac.ctx, latestNetworkVersion))
	ac.handler.send(req)

	return <-req.err
}

func (ac *apiCompare) CompareChainGetGenesis() error {
	check := func(r1, r2 interface{}) error {
		o1, _ := r1.(*types.TipSet)
		o2, _ := r2.(*ltypes.TipSet)
		return tsEquals(o1, o2)
	}

	return ac.sendAndWait(chainGetGenesis, toInterface(ac.ctx), withResultCheck(check))
}

func (ac *apiCompare) CompareStateActorManifestCID() error {
	return ac.sendAndWait(stateActorManifestCID, toInterface(ac.ctx, latestNetworkVersion))
}

func (ac *apiCompare) CompareStateCall() error {
	msg := ac.dp.getMsg()
	if msg == nil {
		return nil
	}

	return ac.sendAndWait(stateCall, toInterface(ac.ctx, msg, types.EmptyTSK), withResultCheck(func(r1, r2 interface{}) error {
		return resultCheckWithInvocResult(msg.Cid(), r1, r2)
	}))
}

func (ac *apiCompare) CompareStateReplay() error {
	msg := ac.dp.getMsg()
	if msg == nil {
		return nil
	}
	c := msg.Cid()

	return ac.sendAndWait(stateReplay, toInterface(ac.ctx, types.EmptyTSK, c), withResultCheck(func(r1, r2 interface{}) error {
		return resultCheckWithInvocResult(c, r1, r2)
	}))
}

func (ac *apiCompare) CompareMinerGetBaseInfo() error {
	height := ac.dp.currTS.Height()
	key := ac.dp.currTS.Parents()

	return ac.sendAndWait(minerGetBaseInfo, toInterface(ac.ctx, ac.dp.defaultMiner(), height, key))
}

//// state ////

func (ac *apiCompare) CompareStateReadState() error {
	addr := ac.dp.getSender()
	if addr == address.Undef {
		addr = ac.dp.defaultMiner()
	}
	key := ac.dp.currTS.Key()

	return ac.sendAndWait(stateReadState, toInterface(ac.ctx, addr, key))
}

func (ac *apiCompare) CompareStateListMessages() error {
	from := ac.dp.getSender()
	if from.Empty() {
		return nil
	}
	height := ac.dp.currTS.Height() - 20

	return ac.sendAndWait(stateListMessages, toInterface(ac.ctx, &types.MessageMatch{From: from}, types.EmptyTSK, height))
}

func (ac *apiCompare) CompareStateDecodeParams() error {
	msgs := ac.dp.getMsgs()
	for _, msg := range msgs {
		if len(msg.Params) > 0 {
			return ac.sendAndWait(stateDecodeParams, toInterface(ac.ctx, msg.To, msg.Method, msg.Params, types.EmptyTSK))
		}
	}

	return nil
}

//// eth ////

func (ac *apiCompare) CompareEthAccounts() error {
	// EthAccounts will always return [] since we don't expect venus to manage private keys
	return ac.sendAndWait(ethAccounts, toInterface(ac.ctx))
}

func (ac *apiCompare) CompareEthAddressToFilecoinAddress() error {
	// todo: use rand address
	addr, err := address.NewFromString("t410flx24nnon3f4dexgt6dh4vtoai33caru6cphna2i")
	if err != nil {
		return err
	}
	vAddr, err := types.EthAddressFromFilecoinAddress(addr)
	if err != nil {
		return fmt.Errorf("venus convert filecoin address %s to eth address failed: %v", addr, err)
	}
	lAddr, err := ethtypes.EthAddressFromFilecoinAddress(addr)
	if err != nil {
		return fmt.Errorf("lotus convert filecoin address %s to eth address failed: %v", addr, err)
	}
	if vAddr != types.EthAddress(lAddr) {
		return fmt.Errorf("eth address not match: %v %v %v", vAddr, lAddr, addr)
	}

	return ac.sendAndWait(ethAddressToFilecoinAddress, toInterface(ac.ctx, vAddr))

	// ki, err := key.NewDelegatedKeyFromSeed(rand.Reader)
	// if err != nil {
	// 	return err
	// }
	// addr, err := ki.Address()
	// if err != nil {
	// 	return err
	// }
	// vAddr, err := types.EthAddressFromFilecoinAddress(addr)
	// if err != nil {
	// 	return fmt.Errorf("venus convert filecoin address %s to eth address failed: %v", addr, err)
	// }
	// lAddr, err := ethtypes.EthAddressFromFilecoinAddress(addr)
	// if err != nil {
	// 	return fmt.Errorf("lotus convert filecoin address %s to eth address failed: %v", addr, err)
	// }
	// if vAddr != types.EthAddress(lAddr) {
	// 	return fmt.Errorf("eth address not match %v != %v", vAddr, lAddr)
	// }

	// return ac.sendAndWait(ethAddressToFilecoinAddress, toInterface(ac.ctx, vAddr))
}

func (ac *apiCompare) CompareEthBlockNumber() error {
	check := func(r1, r2 interface{}) error {
		vnum, _ := r1.(types.EthUint64)
		lnum, _ := r2.(ethtypes.EthUint64)
		if math.Abs(float64(uint64(vnum)-uint64(lnum))) > 1 {
			return fmt.Errorf("not match %d != %d, may sync slow", vnum, lnum)
		}
		return nil
	}

	return ac.sendAndWait(ethBlockNumber, toInterface(ac.ctx), withResultCheck(check))
}

func (ac *apiCompare) CompareEthGetBlockTransactionCountByNumber() error {
	height := ac.dp.currTS.Height()

	return ac.sendAndWait(ethGetBlockTransactionCountByNumber, toInterface(ac.ctx, types.EthUint64(height)))
}

func (ac *apiCompare) CompareEthGetBlockTransactionCountByHash() error {
	blkHash, _, err := ac.dp.getBlockHash()
	if err != nil {
		return err
	}

	return ac.sendAndWait(ethGetBlockTransactionCountByHash, toInterface(ac.ctx, blkHash))
}

func (ac *apiCompare) CompareEthGetBlockByHash() error {
	blkHash, _, err := ac.dp.getBlockHash()
	if err != nil {
		return err
	}

	var fullTxInfo bool
	if err := ac.sendAndWait(ethGetBlockByHash, toInterface(ac.ctx, blkHash, fullTxInfo)); err != nil {
		return fmt.Errorf("fullTxInfo: false, blkhash %s, error: %v", blkHash.ToCid(), err)
	}

	fullTxInfo = true
	if err := ac.sendAndWait(ethGetBlockByHash, toInterface(ac.ctx, blkHash, fullTxInfo)); err != nil {
		return fmt.Errorf("fullTxInfo: true,  blkhash %s, error: %v", blkHash.ToCid(), err)
	}

	return nil
}

func (ac *apiCompare) CompareEthGetBlockByNumber() error {
	blkOpt, err := ac.dp.getBlkOptByHeight()
	if err != nil {
		return err
	}
	// blkParams := []string{blkOpt, blkParamsPending, blkParamsLatest}
	blkParams := []string{blkOpt}

	for _, blkParam := range blkParams {
		if err := ac.sendAndWait(ethGetBlockByNumber, toInterface(ac.ctx, blkParam, false)); err != nil {
			return fmt.Errorf("block param %s, error: %v", blkParam, err)
		}
	}

	return nil
}

func (ac *apiCompare) CompareEthGetTransactionByHash() error {
	msgHash, _, err := ac.dp.getTxHash()
	if err != nil {
		return err
	}

	return ac.sendAndWait(ethGetTransactionByHash, toInterface(ac.ctx, &msgHash))
}

func (ac *apiCompare) CompareEthGetTransactionCount() error {
	addr := ac.dp.getSender()
	if addr.Empty() {
		return nil
	}
	blkOpt, err := ac.dp.getBlkOptByHeight()
	if err != nil {
		return err
	}

	sender, err := types.EthAddressFromFilecoinAddress(addr)
	if err != nil {
		return err
	}

	return ac.sendAndWait(ethGetTransactionCount, toInterface(ac.ctx, sender, blkOpt))
}

func (ac *apiCompare) CompareEthGetTransactionReceipt() error {
	txHash, _, err := ac.dp.getTxHash()
	if err != nil {
		return err
	}

	return ac.sendAndWait(ethGetTransactionReceipt, toInterface(ac.ctx, txHash))
}

func (ac *apiCompare) CompareEthGetTransactionByBlockHashAndIndex() error {
	return ac.sendAndWait(ethGetTransactionByBlockHashAndIndex, toInterface(ac.ctx, emptyEthHash, types.EthUint64(0)))
}

func (ac *apiCompare) CompareEthGetTransactionByBlockNumberAndIndex() error {
	height := ac.dp.currTS.Height()
	return ac.sendAndWait(ethGetTransactionByBlockNumberAndIndex, toInterface(ac.ctx, types.EthUint64(height), types.EthUint64(0)))
}

func (ac *apiCompare) CompareEthGetCode() error {
	addr, _, err := ac.dp.getEthAddress()
	if err != nil {
		return err
	}

	return ac.sendAndWait(ethGetCode, toInterface(ac.ctx, addr, blkParamsLatest))
}

func (ac *apiCompare) CompareEthGetStorageAt() error {
	addr, _, err := ac.dp.getEthAddress()
	if err != nil {
		return err
	}

	return ac.sendAndWait(ethGetStorageAt, toInterface(ac.ctx, addr, types.EthBytes{}, blkParamsLatest))
}

func (ac *apiCompare) CompareEthGetBalance() error {
	addr, _, err := ac.dp.getEthAddress()
	if err != nil {
		return err
	}
	blkOpt, err := ac.dp.getBlkOptByHeight()
	if err != nil {
		return err
	}

	return ac.sendAndWait(ethGetBalance, toInterface(ac.ctx, addr, blkOpt))
}

func (ac *apiCompare) CompareEthChainId() error {
	return ac.sendAndWait(ethChainId, toInterface(ac.ctx))
}

func (ac *apiCompare) CompareNetVersion() error {
	return ac.sendAndWait(netVersion, toInterface(ac.ctx))
}

func (ac *apiCompare) CompareNetListening() error {
	return ac.sendAndWait(netListening, toInterface(ac.ctx))
}

func (ac *apiCompare) CompareEthProtocolVersion() error {
	return ac.sendAndWait(ethProtocolVersion, toInterface(ac.ctx))
}

func (ac *apiCompare) CompareEthGasPrice() error {
	return ac.sendAndWait(ethGasPrice, toInterface(ac.ctx), withResultCheck(func(r1, r2 interface{}) error {
		logrus.Infof("compare EthGasPrice: %d %d\n", r1, r2)
		return nil
	}))
}

func (ac *apiCompare) CompareEthFeeHistory() error {
	newestBlk, err := ac.dp.getBlkOptByHeight()
	if err != nil {
		return err
	}
	rewardPercentiles := make([]float64, 0)

	params := types.EthFeeHistoryParams{
		NewestBlkNum:      newestBlk,
		BlkCount:          10,
		RewardPercentiles: &rewardPercentiles,
	}

	data, err := json.Marshal(params)
	if err != nil {
		return err
	}

	return ac.sendAndWait(ethFeeHistory, toInterface(ac.ctx, data))
}

func (ac *apiCompare) CompareEthMaxPriorityFeePerGas() error {
	return ac.sendAndWait(ethMaxPriorityFeePerGas, toInterface(ac.ctx), withResultCheck(func(r1, r2 interface{}) error {
		logrus.Infof("compare EthMaxPriorityFeePerGas: %d %d\n", r1, r2)
		return nil
	}))
}

// todo: implement
func (ac *apiCompare) CompareEthEstimateGas() error {
	// if err := ac.sendAndWait(ethEstimateGas, toInterface(ac.ctx, types.EthCall{})); err != nil {
	// 	return err
	// }

	// vfrom, err := types.EthAddressFromFilecoinAddress(ac.dp.defaultMiner())
	// if err != nil {
	// 	return err
	// }
	// vcall := types.EthCall{
	// 	From:  &vfrom,
	// 	To:    &vfrom,
	// 	Value: types.EthBigInt(big.NewInt(10)),
	// }

	// return ac.sendAndWait(ethEstimateGas, toInterface(ac.ctx, vcall))
	return nil
}

// todo: implement
func (ac *apiCompare) CompareEthCall() error {
	// if err := ac.sendAndWait(ethCall, toInterface(ac.ctx, types.EthCall{}, blkParamsLatest)); err != nil {
	// 	return err
	// }

	// vfrom, err := types.EthAddressFromFilecoinAddress(ac.dp.defaultMiner())
	// if err != nil {
	// 	return err
	// }
	// vcall := types.EthCall{
	// 	From:  &vfrom,
	// 	To:    &vfrom,
	// 	Value: types.EthBigInt(big.NewInt(10)),
	// }

	// return ac.sendAndWait(ethCall, toInterface(ac.ctx, vcall, blkParamsLatest))
	return nil
}

func (ac *apiCompare) CompareWeb3ClientVersion() error {
	return ac.sendAndWait(web3ClientVersion, toInterface(ac.ctx), withResultCheck(func(r1, r2 interface{}) error {
		fmt.Printf("compare Web3ClientVersion: %v %v\n", r1, r2)
		return nil
	}))
}

func (ac *apiCompare) CompareEthGetTransactionHashByCid() error {
	msg := ac.dp.getMsg()
	if msg == nil {
		return nil
	}

	return ac.sendAndWait(ethGetTransactionHashByCid, toInterface(ac.ctx, msg.Cid()))
}

func (ac *apiCompare) CompareEthGetMessageCidByTransactionHash() error {
	msg := ac.dp.getMsg()
	if msg == nil {
		return nil
	}
	h, err := types.EthHashFromCid(msg.Cid())
	if err != nil {
		return err
	}

	return ac.sendAndWait(ethGetMessageCidByTransactionHash, toInterface(ac.ctx, &h))
}
