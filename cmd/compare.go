package cmd

import (
	"context"
	"fmt"
	"log"

	"github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/lotus/api"
	ltypes "github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/venus/pkg/constants"
	v1 "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs/go-cid"
)

const latestNetworkVersion = network.Version17

func newCompareMgr(ctx context.Context,
	vAPI v1.FullNode,
	lAPI api.FullNode,
	dp *dataProvider,
	currTS *types.TipSet,
) *compareMgr {
	return &compareMgr{
		ctx:    ctx,
		vAPI:   vAPI,
		lAPI:   lAPI,
		dp:     dp,
		currTS: currTS,
		next:   make(chan struct{}, 10),
	}
}

type compareMgr struct {
	ctx context.Context

	vAPI v1.FullNode
	lAPI api.FullNode

	dp *dataProvider

	currTS   *types.TipSet
	latestTS *types.TipSet

	next chan struct{}
}

func (cmgr *compareMgr) start() {
	if err := cmgr.chainNotify(); err != nil {
		log.Fatalf("chain notify error: %v\n", err)
	}

	first := true
	cmgr.next <- struct{}{}

	for {
		select {
		case <-cmgr.ctx.Done():
			fmt.Println("context done")
			return
		case <-cmgr.next:
			if first {
				first = false
			} else {
				ts, err := cmgr.findNextTS(cmgr.currTS)
				if err != nil {
					panic(err)
				}
				cmgr.currTS = ts
			}
			if err := cmgr.compareAPI(); err != nil {
				fmt.Printf("compare api error: %v\n", err)
			}
		}
	}
}

func (cmgr *compareMgr) chainNotify() error {
	ctx, cancel := context.WithCancel(cmgr.ctx)
	notifs, err := cmgr.vAPI.ChainNotify(ctx)
	if err != nil {
		cancel()
		return err
	}

	select {
	case noti := <-notifs:
		if len(noti) != 1 {
			cancel()
			return fmt.Errorf("expect hccurrent length 1 but for %d", len(noti))
		}

		if noti[0].Type != types.HCCurrent {
			cancel()
			return fmt.Errorf("expect hccurrent event but got %s ", noti[0].Type)
		}
		cmgr.latestTS = noti[0].Val
	case <-ctx.Done():
		cancel()
		return ctx.Err()
	}

	go func() {
		defer cancel()

		for notif := range notifs {
			var apply []*types.TipSet

			for _, change := range notif {
				switch change.Type {
				case types.HCApply:
					apply = append(apply, change.Val)
				}
			}
			if apply[0].Height() > cmgr.latestTS.Height() {
				cmgr.latestTS = apply[0]

				cmgr.next <- struct{}{}
			}
		}
	}()

	return nil
}

func (cmgr *compareMgr) findNextTS(currTS *types.TipSet) (*types.TipSet, error) {
	vts, err := cmgr.vAPI.ChainGetTipSetAfterHeight(cmgr.ctx, currTS.Height()+1, types.EmptyTSK)
	if err != nil {
		return nil, err
	}
	lts, err := cmgr.lAPI.ChainGetTipSetAfterHeight(cmgr.ctx, currTS.Height()+1, ltypes.EmptyTSK)
	if err != nil {
		return nil, err
	}

	if vts.Height() != lts.Height() {
		return nil, fmt.Errorf("height not match %d != %d", vts.Height(), lts.Height())
	}
	if !vts.Key().Equals(types.NewTipSetKey(lts.Cids()...)) {
		return nil, fmt.Errorf("cids not match %v != %v", vts.Cids(), lts.Cids())
	}

	return vts, nil
}

func (cmgr *compareMgr) compareAPI() error {
	if err := cmgr.dp.reset(cmgr.currTS); err != nil {
		return err
	}

	cmgr.printResult("StateAccountKey", cmgr.compareStateAccountKey())
	cmgr.printResult("StateGetActor", cmgr.compareStateGetActor())
	cmgr.printResult("ChainGetTipSet", cmgr.compareChainGetTipSet())
	cmgr.printResult("ChainGetTipSetByHeight", cmgr.compareChainGetTipSetByHeight())
	cmgr.printResult("StateGetRandomnessFromBeacon", cmgr.compareStateGetRandomnessFromBeacon())
	cmgr.printResult("StateGetRandomnessFromTickets", cmgr.compareStateGetRandomnessFromTickets())
	cmgr.printResult("StateGetBeaconEntry", cmgr.compareStateGetBeaconEntry())
	cmgr.printResult("ChainGetBlock", cmgr.compareChainGetBlock())
	cmgr.printResult("ChainGetBlockMessages", cmgr.compareChainGetBlockMessages())
	cmgr.printResult("ChainGetMessage", cmgr.compareChainGetMessage())
	cmgr.printResult("ChainGetMessagesInTipset", cmgr.compareChainGetMessagesInTipset())
	cmgr.printResult("ChainGetParentMessages", cmgr.compareChainGetParentMessages())
	cmgr.printResult("ChainGetParentReceipts", cmgr.compareChainGetParentReceipts())
	cmgr.printResult("StateVerifiedRegistryRootKey", cmgr.compareStateVerifiedRegistryRootKey())
	cmgr.printResult("StateVerifierStatus", cmgr.compareStateVerifierStatus())
	cmgr.printResult("StateNetworkName", cmgr.compareStateNetworkName())
	cmgr.printResult("SearchWaitMessage", cmgr.compareSearchWaitMessage())
	cmgr.printResult("StateNetworkVersion", cmgr.compareStateNetworkVersion())
	cmgr.printResult("ChainGetPath", cmgr.compareChainGetPath())
	cmgr.printResult("StateGetNetworkParams", cmgr.compareStateGetNetworkParams())
	cmgr.printResult("StateActorCodeCIDs", cmgr.compareStateActorCodeCIDs())
	cmgr.printResult("ChainGetGenesis", cmgr.compareChainGetGenesis())
	cmgr.printResult("StateActorManifestCID", cmgr.compareStateActorManifestCID())
	cmgr.printResult("MinerGetBaseInfo", cmgr.compareMinerGetBaseInfo())

	// eth api
	cmgr.printResult("EthAccounts", cmgr.compareEthAccounts())
	cmgr.printResult("EthBlockNumber", cmgr.compareEthBlockNumber())
	cmgr.printResult("EthGetBlockTransactionCountByNumber", cmgr.compareEthGetBlockTransactionCountByNumber())
	cmgr.printResult("EthGetBlockTransactionCountByHash", cmgr.compareEthGetBlockTransactionCountByHash())
	cmgr.printResult("EthGetBlockByHash", cmgr.compareEthGetBlockByHash())
	cmgr.printResult("EthGetBlockByNumber", cmgr.compareEthGetBlockByNumber())
	cmgr.printResult("EthGetTransactionByHash", cmgr.compareEthGetTransactionByHash())
	cmgr.printResult("EthGetTransactionCount", cmgr.compareEthGetTransactionCount())
	cmgr.printResult("EthGetTransactionReceipt", cmgr.compareEthGetTransactionReceipt())
	cmgr.printResult("EthGetTransactionByBlockHashAndIndex", cmgr.compareEthGetTransactionByBlockHashAndIndex())
	cmgr.printResult("EthGetTransactionByBlockNumberAndIndex", cmgr.compareEthGetTransactionByBlockNumberAndIndex())
	cmgr.printResult("EthGetCode", cmgr.compareEthGetCode())
	// cmgr.printResult("EthGetStorageAt", cmgr.compareEthGetStorageAt())
	cmgr.printResult("EthGetBalance", cmgr.compareEthGetBalance())
	cmgr.printResult("EthChainId", cmgr.compareEthChainId())
	cmgr.printResult("NetVersion", cmgr.compareNetVersion())
	cmgr.printResult("NetListening", cmgr.compareNetListening())
	cmgr.printResult("EthProtocolVersion", cmgr.compareEthProtocolVersion())
	cmgr.printResult("EthGasPrice", cmgr.compareEthGasPrice())
	cmgr.printResult("EthFeeHistory", cmgr.compareEthFeeHistory())
	cmgr.printResult("EthMaxPriorityFeePerGas", cmgr.compareEthMaxPriorityFeePerGas())
	cmgr.printResult("EthEstimateGas", cmgr.compareEthEstimateGas())
	cmgr.printResult("EthCall", cmgr.compareEthCall())
	cmgr.printResult("EthSendRawTransaction", cmgr.compareEthSendRawTransaction())

	return nil
}

func (cmgr *compareMgr) compareStateAccountKey() error {
	addr := cmgr.dp.getSender()
	if addr.Empty() {
		return nil
	}
	vaddr, err := cmgr.vAPI.StateAccountKey(cmgr.ctx, addr, cmgr.currTS.Key())
	if err != nil {
		return err
	}
	laddr, err := cmgr.lAPI.StateAccountKey(cmgr.ctx, addr, toLoutsTipsetKey(cmgr.currTS.Key()))
	if err != nil {
		return err
	}
	if vaddr != laddr {
		return fmt.Errorf("address not match %s != %s, origin address %s", vaddr, laddr, addr)
	}

	vaddr2, err := cmgr.vAPI.StateAccountKey(cmgr.ctx, vaddr, cmgr.currTS.Key())
	if err != nil {
		return err
	}
	laddr, err = cmgr.lAPI.StateAccountKey(cmgr.ctx, vaddr, toLoutsTipsetKey(cmgr.currTS.Key()))
	if err != nil {
		return err
	}
	if vaddr2 != laddr {
		return fmt.Errorf("address not match %s != %s, origin address %s", vaddr2, laddr, vaddr)
	}

	return nil
}

func (cmgr *compareMgr) compareStateGetActor() error {
	vactor, err := cmgr.vAPI.StateGetActor(cmgr.ctx, cmgr.dp.defaultMiner(), cmgr.currTS.Key())
	if err != nil {
		return err
	}
	lactor, err := cmgr.lAPI.StateGetActor(cmgr.ctx, cmgr.dp.defaultMiner(), toLoutsTipsetKey(cmgr.currTS.Key()))
	if err != nil {
		return err
	}

	return checkByJSON(vactor, lactor)
}

func (cmgr *compareMgr) compareChainGetTipSet() error {
	vts, err := cmgr.vAPI.ChainGetTipSet(cmgr.ctx, cmgr.currTS.Key())
	if err != nil {
		return err
	}
	lts, err := cmgr.lAPI.ChainGetTipSet(cmgr.ctx, toLoutsTipsetKey(cmgr.currTS.Key()))
	if err != nil {
		return err
	}

	return checkByJSON(vts, lts)
}

func (cmgr *compareMgr) compareChainGetTipSetByHeight() error {
	height := cmgr.currTS.Height() - 10
	key := cmgr.currTS.Key()

	vts, err := cmgr.vAPI.ChainGetTipSetByHeight(cmgr.ctx, height, key)
	if err != nil {
		return err
	}
	lts, err := cmgr.lAPI.ChainGetTipSetByHeight(cmgr.ctx, height, toLoutsTipsetKey(key))
	if err != nil {
		return err
	}
	if err := checkByJSON(vts, lts); err != nil {
		return err
	}

	// Too high
	_, err = cmgr.vAPI.ChainGetTipSetByHeight(cmgr.ctx, height+100, key)
	if err == nil {
		return fmt.Errorf("expect error but found nil")
	}
	_, err = cmgr.lAPI.ChainGetTipSetByHeight(cmgr.ctx, height+100, toLoutsTipsetKey(key))
	if err == nil {
		return fmt.Errorf("expect error but found nil")
	}

	return nil
}

func (cmgr *compareMgr) compareStateGetRandomnessFromBeacon() error {
	per := crypto.DomainSeparationTag_TicketProduction
	randEpoch := cmgr.currTS.Height()
	emtropy := []byte("fixed-randomness")
	tsk := cmgr.currTS.Key()

	for per < crypto.DomainSeparationTag_PoStChainCommit {
		vrandomness, err := cmgr.vAPI.StateGetRandomnessFromBeacon(cmgr.ctx, per, randEpoch, emtropy, tsk)
		if err != nil {
			return err
		}
		lrandomness, err := cmgr.lAPI.StateGetRandomnessFromBeacon(cmgr.ctx, per, randEpoch, emtropy, toLoutsTipsetKey(tsk))
		if err != nil {
			return err
		}
		if err := checkByJSON(vrandomness, lrandomness); err != nil {
			return err
		}

		per++
	}

	return nil
}

func (cmgr *compareMgr) compareStateGetRandomnessFromTickets() error {
	per := crypto.DomainSeparationTag_TicketProduction
	randEpoch := cmgr.currTS.Height()
	emtropy := []byte("fixed-randomness")
	tsk := cmgr.currTS.Key()

	for per < crypto.DomainSeparationTag_PoStChainCommit {
		vrandomness, err := cmgr.vAPI.StateGetRandomnessFromTickets(cmgr.ctx, per, randEpoch, emtropy, tsk)
		if err != nil {
			return err
		}
		lrandomness, err := cmgr.lAPI.StateGetRandomnessFromTickets(cmgr.ctx, per, randEpoch, emtropy, toLoutsTipsetKey(tsk))
		if err != nil {
			return err
		}
		if err := checkByJSON(vrandomness, lrandomness); err != nil {
			return err
		}

		per++
	}

	return nil
}

func (cmgr *compareMgr) compareStateGetBeaconEntry() error {
	height := cmgr.currTS.Height()

	vbe, err := cmgr.vAPI.StateGetBeaconEntry(cmgr.ctx, height)
	if err != nil {
		return err
	}
	lbe, err := cmgr.lAPI.StateGetBeaconEntry(cmgr.ctx, height)
	if err != nil {
		return err
	}

	return checkByJSON(vbe, lbe)
}

func (cmgr *compareMgr) compareChainGetBlock() error {
	for _, blk := range cmgr.currTS.Blocks() {
		vbh, err := cmgr.vAPI.ChainGetBlock(cmgr.ctx, blk.Cid())
		if err != nil {
			return err
		}
		lbh, err := cmgr.lAPI.ChainGetBlock(cmgr.ctx, blk.Cid())
		if err != nil {
			return err
		}
		if err := checkByJSON(vbh, lbh); err != nil {
			return fmt.Errorf("block: %s, error: %v", blk.Cid(), err)
		}
	}

	return nil
}

func (cmgr *compareMgr) compareChainGetBlockMessages() error {
	for _, blk := range cmgr.currTS.Blocks() {
		vmsgs, err := cmgr.vAPI.ChainGetBlockMessages(cmgr.ctx, blk.Cid())
		if err != nil {
			return err
		}
		lmsgs, err := cmgr.lAPI.ChainGetBlockMessages(cmgr.ctx, blk.Cid())
		if err != nil {
			return err
		}

		if !equal(vmsgs, lmsgs) {
			return fmt.Errorf("block: %v, not match %+v != %+v", blk.Cid(), vmsgs, lmsgs)
		}
	}

	return nil
}

func (cmgr *compareMgr) compareChainGetMessage() error {
	for _, blk := range cmgr.currTS.Blocks() {
		blkMsgs, err := cmgr.vAPI.ChainGetBlockMessages(cmgr.ctx, blk.Cid())
		if err != nil {
			return fmt.Errorf("failed to get block %s messages: %v", blk.Cid(), err)
		}
		for _, msgCID := range blkMsgs.Cids {
			vmsg, err := cmgr.vAPI.ChainGetMessage(cmgr.ctx, msgCID)
			if err != nil {
				return err
			}
			lmsg, err := cmgr.lAPI.ChainGetMessage(cmgr.ctx, msgCID)
			if err != nil {
				return err
			}

			if !equal(vmsg, lmsg) {
				return fmt.Errorf("msg: %s, not match %+v != %+v", msgCID, vmsg, lmsg)
			}
		}
	}

	return nil
}

func (cmgr *compareMgr) compareChainGetMessagesInTipset() error {
	key := cmgr.currTS.Key()
	vmsgs, err := cmgr.vAPI.ChainGetMessagesInTipset(cmgr.ctx, key)
	if err != nil {
		return err
	}
	lmsgs, err := cmgr.lAPI.ChainGetMessagesInTipset(cmgr.ctx, toLoutsTipsetKey(key))
	if err != nil {
		return err
	}

	if !equal(vmsgs, lmsgs) {
		return fmt.Errorf("not match %+v != %+v", vmsgs, lmsgs)
	}

	return nil
}

func (cmgr *compareMgr) compareChainGetParentMessages() error {
	for _, blkCID := range cmgr.currTS.Cids() {
		vmsg, err := cmgr.vAPI.ChainGetParentMessages(cmgr.ctx, blkCID)
		if err != nil {
			return err
		}
		lmsg, err := cmgr.lAPI.ChainGetParentMessages(cmgr.ctx, blkCID)
		if err != nil {
			return err
		}

		if !equal(vmsg, lmsg) {
			return fmt.Errorf("block: %s, not match %+v != %+v", blkCID, vmsg, lmsg)
		}
	}

	return nil
}

func (cmgr *compareMgr) compareChainGetParentReceipts() error {
	for _, blkCID := range cmgr.currTS.Cids() {
		vreceipts, err := cmgr.vAPI.ChainGetParentReceipts(cmgr.ctx, blkCID)
		if err != nil {
			return err
		}
		lreceipts, err := cmgr.lAPI.ChainGetParentReceipts(cmgr.ctx, blkCID)
		if err != nil {
			return err
		}
		if err := checkByJSON(vreceipts, lreceipts); err != nil {
			return fmt.Errorf("block: %s, error: %v", blkCID, err)
		}
	}

	return nil
}

func (cmgr *compareMgr) compareStateVerifiedRegistryRootKey() error {
	key := cmgr.currTS.Key()
	vaddr, err := cmgr.vAPI.StateVerifiedRegistryRootKey(cmgr.ctx, key)
	if err != nil {
		return err
	}
	laddr, err := cmgr.lAPI.StateVerifiedRegistryRootKey(cmgr.ctx, toLoutsTipsetKey(key))
	if err != nil {
		return err
	}

	if vaddr != laddr {
		return fmt.Errorf("address not match %s != %s", vaddr, laddr)
	}

	return nil
}

func (cmgr *compareMgr) compareStateVerifierStatus() error {
	key := cmgr.currTS.Key()
	vres, err := cmgr.vAPI.StateVerifierStatus(cmgr.ctx, cmgr.dp.defaultMiner(), key)
	if err != nil {
		return err
	}
	lres, err := cmgr.lAPI.StateVerifierStatus(cmgr.ctx, cmgr.dp.defaultMiner(), toLoutsTipsetKey(key))
	if err != nil {
		return err
	}

	return bigIntEqual(vres, lres)
}

func (cmgr *compareMgr) compareStateNetworkName() error {
	vname, err := cmgr.vAPI.StateNetworkName(cmgr.ctx)
	if err != nil {
		return err
	}
	lname, err := cmgr.lAPI.StateNetworkName(cmgr.ctx)
	if err != nil {
		return err
	}
	if vname != types.NetworkName(lname) {
		return fmt.Errorf("not match %s != %s", vname, lname)
	}

	return nil
}

func (cmgr *compareMgr) compareSearchWaitMessage() error {
	key := cmgr.currTS.Key()
	searchMsg := func(msgCID cid.Cid) error {
		vmsg, err := cmgr.vAPI.StateSearchMsg(cmgr.ctx, key, msgCID, constants.LookbackNoLimit, true)
		if err != nil {
			return fmt.Errorf("search msg %s faield: %v", msgCID, err)
		}
		lmsg, err := cmgr.lAPI.StateSearchMsg(cmgr.ctx, toLoutsTipsetKey(key), msgCID, constants.LookbackNoLimit, true)
		if err != nil {
			return fmt.Errorf("search msg %s faield: %v", msgCID, err)
		}

		return checkByJSON(vmsg, lmsg)
	}

	waitMsg := func(msgCID cid.Cid) error {
		vMsgLookup, err := cmgr.vAPI.StateWaitMsg(cmgr.ctx, msgCID, constants.DefaultConfidence, constants.LookbackNoLimit, true)
		if err != nil {
			return fmt.Errorf("wait msg %s faield: %v", msgCID, err)
		}
		lMsgLookup, err := cmgr.lAPI.StateWaitMsg(cmgr.ctx, msgCID, constants.DefaultConfidence, constants.LookbackNoLimit, true)
		if err != nil {
			return fmt.Errorf("wait msg %s faield: %v", msgCID, err)
		}

		return checkByJSON(vMsgLookup, lMsgLookup)
	}

	for i, msg := range cmgr.dp.getMsgs() {
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

func (cmgr *compareMgr) compareStateNetworkVersion() error {
	key := cmgr.currTS.Key()
	vv, err := cmgr.vAPI.StateNetworkVersion(cmgr.ctx, key)
	if err != nil {
		return err
	}
	lv, err := cmgr.lAPI.StateNetworkVersion(cmgr.ctx, toLoutsTipsetKey(key))
	if err != nil {
		return err
	}
	if vv != lv {
		return fmt.Errorf("not match %d != %d", vv, lv)
	}

	return nil
}

func (cmgr *compareMgr) compareChainGetPath() error {
	ts := cmgr.currTS
	from, err := cmgr.vAPI.ChainGetTipSetAfterHeight(cmgr.ctx, ts.Height()-5, ts.Key())
	if err != nil {
		return err
	}

	vhc, err := cmgr.vAPI.ChainGetPath(cmgr.ctx, from.Key(), ts.Key())
	if err != nil {
		return err
	}

	lhc, err := cmgr.lAPI.ChainGetPath(cmgr.ctx, toLoutsTipsetKey(from.Key()), toLoutsTipsetKey(ts.Key()))
	if err != nil {
		return err
	}

	return checkByJSON(vhc, lhc)
}

func (cmgr *compareMgr) compareStateGetNetworkParams() error {
	vparams, err := cmgr.vAPI.StateGetNetworkParams(cmgr.ctx)
	if err != nil {
		return err
	}
	lparams, err := cmgr.lAPI.StateGetNetworkParams(cmgr.ctx)
	if err != nil {
		return err
	}

	if !equal(vparams, lparams) {
		return fmt.Errorf("not match %+v != %+v", vparams, lparams)
	}

	return nil
}

func (cmgr *compareMgr) compareStateActorCodeCIDs() error {
	vactorcode, err := cmgr.vAPI.StateActorCodeCIDs(cmgr.ctx, latestNetworkVersion)
	if err != nil {
		return err
	}
	lactorcode, err := cmgr.lAPI.StateActorCodeCIDs(cmgr.ctx, latestNetworkVersion)
	if err != nil {
		return err
	}

	return checkByJSON(vactorcode, lactorcode)
}

func (cmgr *compareMgr) compareChainGetGenesis() error {
	vts, err := cmgr.vAPI.ChainGetGenesis(cmgr.ctx)
	if err != nil {
		return err
	}
	lts, err := cmgr.lAPI.ChainGetGenesis(cmgr.ctx)
	if err != nil {
		return err
	}

	return tsEquals(vts, lts)
}

func (cmgr *compareMgr) compareStateActorManifestCID() error {
	vres, err := cmgr.vAPI.StateActorManifestCID(cmgr.ctx, latestNetworkVersion)
	if err != nil {
		return err
	}
	lres, err := cmgr.lAPI.StateActorManifestCID(cmgr.ctx, latestNetworkVersion)
	if err != nil {
		return err
	}
	if vres != lres {
		return fmt.Errorf("not match %s != %s", vres, lres)
	}

	return nil
}

func (cmgr *compareMgr) compareMinerGetBaseInfo() error {
	height := cmgr.currTS.Height()
	tsk := cmgr.currTS.Parents()
	vinfo, err := cmgr.vAPI.MinerGetBaseInfo(cmgr.ctx, cmgr.dp.defaultMiner(), height, tsk)
	if err != nil {
		return err
	}

	linfo, err := cmgr.lAPI.MinerGetBaseInfo(cmgr.ctx, cmgr.dp.defaultMiner(), height, toLoutsTipsetKey(tsk))
	if err != nil {
		return err
	}

	// return checkByJSON(vinfo, linfo)
	if !equal(vinfo, linfo) {
		return fmt.Errorf("not match %+v != %+v", vinfo, linfo)
	}

	return nil
}

func (cmgr *compareMgr) printResult(method string, err error) {
	height := cmgr.currTS.Height()
	if err != nil {
		fmt.Printf("height: %d, compare %s failed, reason: %v \n", height, method, err)
	} else {
		fmt.Printf("height: %d, compare %s success \n", height, method)
	}
}
