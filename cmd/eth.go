package cmd

import (
	"bytes"
	"fmt"
	"math"
	"math/big"
	"strings"

	"github.com/filecoin-project/lotus/chain/types/ethtypes"
	"github.com/filecoin-project/venus/venus-shared/types"
)

const (
	blkParamsEarliest = "earliest"
	blkParamsPending  = "pending"
	blkParamsLatest   = "latest"
)

var (
	emptyEthHash  = types.EthHash{}
	emptyLEthHash = ethtypes.EthHash{}

	emptyEthAddress  = types.EthAddress{}
	emptyLEthAddress = ethtypes.EthAddress{}
)

func (cmgr *compareMgr) compareEthAccounts() error {
	// EthAccounts will always return [] since we don't expect venus to manage private keys
	vaccount, err := cmgr.vAPI.EthAccounts(cmgr.ctx)
	if err != nil {
		return err
	}
	laccount, err := cmgr.lAPI.EthAccounts(cmgr.ctx)
	if err != nil {
		return err
	}

	if len(vaccount) == 0 && len(laccount) == 0 {
		return nil
	}

	return fmt.Errorf("not match %d != %d", len(vaccount), len(laccount))
}

func (cmgr *compareMgr) compareEthBlockNumber() error {
	vnum, err := cmgr.vAPI.EthBlockNumber(cmgr.ctx)
	if err != nil {
		return err
	}
	lnum, err := cmgr.lAPI.EthBlockNumber(cmgr.ctx)
	if err != nil {
		return err
	}

	if math.Abs(float64(uint64(vnum)-uint64(lnum))) > 1 {
		return fmt.Errorf("not match %d != %d, may sync slow", vnum, lnum)
	}

	return nil
}

func (cmgr *compareMgr) compareEthGetBlockTransactionCountByNumber() error {
	height := cmgr.currTS.Height()
	vtc, err := cmgr.vAPI.EthGetBlockTransactionCountByNumber(cmgr.ctx, types.EthUint64(height))
	if err != nil {
		return err
	}
	ltc, err := cmgr.lAPI.EthGetBlockTransactionCountByNumber(cmgr.ctx, ethtypes.EthUint64(height))
	if err != nil {
		return err
	}

	if vtc != types.EthUint64(ltc) {
		return fmt.Errorf("not match %d != %d", vtc, ltc)
	}

	return nil
}

func (cmgr *compareMgr) compareEthGetBlockTransactionCountByHash() error {
	blkHash, blkHash2, err := cmgr.dp.getBlockHash()
	if err != nil {
		return err
	}
	vtc, err := cmgr.vAPI.EthGetBlockTransactionCountByHash(cmgr.ctx, blkHash)
	if err != nil {
		return fmt.Errorf("venus %v", err)
	}

	ltc, err := cmgr.lAPI.EthGetBlockTransactionCountByHash(cmgr.ctx, blkHash2)
	if err != nil {
		return fmt.Errorf("lotus %v", err)
	}

	if vtc != types.EthUint64(ltc) {
		return fmt.Errorf("not match %d != %d", vtc, ltc)
	}

	return nil
}

func (cmgr *compareMgr) compareEthGetBlockByHash() error {
	blkHash, blkHash2, err := cmgr.dp.getBlockHash()
	if err != nil {
		return err
	}

	doCheck := func(fullTxInfo bool) error {
		vblk, err := cmgr.vAPI.EthGetBlockByHash(cmgr.ctx, blkHash, fullTxInfo)
		if err != nil {
			return fmt.Errorf("venus %v", err)
		}
		lblk, err := cmgr.lAPI.EthGetBlockByHash(cmgr.ctx, blkHash2, fullTxInfo)
		if err != nil {
			return fmt.Errorf("lotus %v", err)
		}

		return checkByJSON(vblk, lblk)
	}

	if err := doCheck(true); err != nil {
		return fmt.Errorf("fullTxInfo: true, error: %v", err)
	}

	if err := doCheck(false); err != nil {
		return fmt.Errorf("fullTxInfo: false, error: %v", err)
	}

	return nil
}

func (cmgr *compareMgr) compareEthGetBlockByNumber() error {
	blkOpt, err := cmgr.dp.getBlkOptByHeight()
	if err != nil {
		return err
	}
	blkOpt = strings.Replace(blkOpt, "\"", "", -1)
	blkParams := []string{blkOpt, blkParamsEarliest, blkParamsPending, blkParamsLatest}

	doCheck := func(blkParam string) error {
		vblk, err := cmgr.vAPI.EthGetBlockByNumber(cmgr.ctx, blkParam, false)
		if err != nil && blkParam != blkParamsEarliest {
			return err
		}
		lblk, err := cmgr.lAPI.EthGetBlockByNumber(cmgr.ctx, blkParam, false)
		if err != nil && blkParam != blkParamsEarliest {
			return err
		}

		return checkByJSON(vblk, lblk)
	}

	for _, blkParam := range blkParams {
		if err := doCheck(blkParam); err != nil {
			return fmt.Errorf("block param %s, error: %v", blkParam, err)
		}
	}

	return nil
}

func (cmgr *compareMgr) compareEthGetTransactionByHash() error {
	msgHash, msgHash2, err := cmgr.dp.getTxHash()
	if err != nil {
		return err
	}

	vt, err := cmgr.vAPI.EthGetTransactionByHash(cmgr.ctx, &msgHash)
	if err != nil {
		return err
	}
	lt, err := cmgr.lAPI.EthGetTransactionByHash(cmgr.ctx, &msgHash2)
	if err != nil {
		return err
	}

	return checkByJSON(vt, lt)
}

func (cmgr *compareMgr) compareEthGetTransactionCount() error {
	addr := cmgr.dp.getSender()
	if addr.Empty() {
		return nil
	}
	blkOpt, err := cmgr.dp.getBlkOptByHeight()
	if err != nil {
		return err
	}
	blkOpt = strings.Replace(blkOpt, "\"", "", -1)
	sender, err := types.EthAddressFromFilecoinAddress(addr)
	if err != nil {
		return err
	}
	sender2, err := ethtypes.EthAddressFromFilecoinAddress(addr)
	if err != nil {
		return err
	}

	vt, err := cmgr.vAPI.EthGetTransactionCount(cmgr.ctx, sender, blkOpt)
	if err != nil {
		return err
	}
	lt, err := cmgr.lAPI.EthGetTransactionCount(cmgr.ctx, sender2, blkOpt)
	if err != nil {
		return err
	}

	if vt != types.EthUint64(lt) {
		return fmt.Errorf("not match %d != %d", vt, lt)
	}

	return nil
}

func (cmgr *compareMgr) compareEthGetTransactionReceipt() error {
	txHash, txHash2, err := cmgr.dp.getTxHash()
	if err != nil {
		return err
	}

	vr, err := cmgr.vAPI.EthGetTransactionReceipt(cmgr.ctx, txHash)
	if err != nil {
		return err
	}
	lr, err := cmgr.lAPI.EthGetTransactionReceipt(cmgr.ctx, txHash2)
	if err != nil {
		return err
	}

	return checkByJSON(vr, lr)
}

func (cmgr *compareMgr) compareEthGetTransactionByBlockHashAndIndex() error {
	vt, err := cmgr.vAPI.EthGetTransactionByBlockHashAndIndex(cmgr.ctx, emptyEthHash, 0)
	if err != nil {
		return err
	}
	lt, err := cmgr.lAPI.EthGetTransactionByBlockHashAndIndex(cmgr.ctx, emptyLEthHash, 0)
	if err != nil {
		return err
	}

	if vt.ChainID != types.EthUint64(lt.ChainID) || vt.From != types.EthAddress(lt.From) {
		return fmt.Errorf("expect empty %v != %v", vt, lt)
	}

	return nil
}

func (cmgr *compareMgr) compareEthGetTransactionByBlockNumberAndIndex() error {
	height := cmgr.currTS.Height()
	vt, err := cmgr.vAPI.EthGetTransactionByBlockNumberAndIndex(cmgr.ctx, types.EthUint64(height), 0)
	if err != nil {
		return err
	}
	lt, err := cmgr.lAPI.EthGetTransactionByBlockNumberAndIndex(cmgr.ctx, ethtypes.EthUint64(height), 0)
	if err != nil {
		return err
	}

	if vt.ChainID != types.EthUint64(lt.ChainID) || vt.From != types.EthAddress(lt.From) {
		return fmt.Errorf("expect empty %v != %v", vt, lt)
	}

	return nil
}

func (cmgr *compareMgr) compareEthGetCode() error {
	addr, addr2, err := cmgr.dp.getEthAddress()
	if err != nil {
		return err
	}

	vcode, err := cmgr.vAPI.EthGetCode(cmgr.ctx, addr, blkParamsLatest)
	if err != nil {
		return err
	}
	lcode, err := cmgr.lAPI.EthGetCode(cmgr.ctx, addr2, blkParamsLatest)
	if err != nil {
		return err
	}

	return checkByJSON(vcode, lcode)
}

func (cmgr *compareMgr) compareEthGetStorageAt() error {
	addr, addr2, err := cmgr.dp.getEthAddress()
	if err != nil {
		return err
	}

	vres, err := cmgr.vAPI.EthGetStorageAt(cmgr.ctx, addr, types.EthBytes{}, blkParamsLatest)
	if err != nil {
		return err
	}
	lres, err := cmgr.lAPI.EthGetStorageAt(cmgr.ctx, addr2, ethtypes.EthBytes{}, blkParamsLatest)
	if err != nil {
		return err
	}

	return checkByJSON(vres, lres)
}

func (cmgr *compareMgr) compareEthGetBalance() error {
	addr, addr2, err := cmgr.dp.getEthAddress()
	if err != nil {
		return err
	}

	vb, err := cmgr.vAPI.EthGetBalance(cmgr.ctx, addr, blkParamsLatest)
	if err != nil {
		return err
	}
	lb, err := cmgr.lAPI.EthGetBalance(cmgr.ctx, addr2, blkParamsLatest)
	if err != nil {
		return err
	}

	if vb.Cmp(lb.Int) != 0 {
		return fmt.Errorf("not match %v != %v", vb, lb)
	}

	return nil
}

func (cmgr *compareMgr) compareEthChainId() error {
	vid, err := cmgr.vAPI.EthChainId(cmgr.ctx)
	if err != nil {
		return err
	}
	lid, err := cmgr.lAPI.EthChainId(cmgr.ctx)
	if err != nil {
		return err
	}

	if vid != types.EthUint64(lid) {
		return fmt.Errorf("not match %d != %d", vid, lid)
	}

	return nil
}

func (cmgr *compareMgr) compareNetVersion() error {
	vv, err := cmgr.vAPI.NetVersion(cmgr.ctx)
	if err != nil {
		return err
	}
	lv, err := cmgr.vAPI.NetVersion(cmgr.ctx)
	if err != nil {
		return err
	}

	if vv != lv {
		return fmt.Errorf("not match %v != %v", vv, lv)
	}

	return nil
}

func (cmgr *compareMgr) compareNetListening() error {
	vres, err := cmgr.vAPI.NetListening(cmgr.ctx)
	if err != nil {
		return err
	}
	lres, err := cmgr.lAPI.NetListening(cmgr.ctx)
	if err != nil {
		return err
	}

	if vres != lres {
		return fmt.Errorf("not match %v != %v", vres, lres)
	}

	return nil
}

func (cmgr *compareMgr) compareEthProtocolVersion() error {
	vres, err := cmgr.vAPI.EthProtocolVersion(cmgr.ctx)
	if err != nil {
		return err
	}
	lres, err := cmgr.lAPI.EthProtocolVersion(cmgr.ctx)
	if err != nil {
		return err
	}

	if vres != types.EthUint64(lres) {
		return fmt.Errorf("not match %v != %v", vres, lres)
	}

	return nil
}

func (cmgr *compareMgr) compareEthGasPrice() error {
	vres, err := cmgr.vAPI.EthGasPrice(cmgr.ctx)
	if err != nil {
		return err
	}
	lres, err := cmgr.lAPI.EthGasPrice(cmgr.ctx)
	if err != nil {
		return err
	}
	fmt.Printf("compareEthGasPrice gas price %d %d\n", vres, lres)

	// return ethBigIntEqual(vres, lres)
	return nil
}

func (cmgr *compareMgr) compareEthFeeHistory() error {
	blkCount := 10
	newestBlk, err := cmgr.dp.getBlkOptByHeight()
	if err != nil {
		return err
	}
	newestBlk = strings.Replace(newestBlk, "\"", "", -1)
	rewardPercentiles := make([]float64, 0)
	vres, err := cmgr.vAPI.EthFeeHistory(cmgr.ctx, types.EthUint64(blkCount), newestBlk, rewardPercentiles)
	if err != nil {
		return err
	}
	lres, err := cmgr.lAPI.EthFeeHistory(cmgr.ctx, ethtypes.EthUint64(blkCount), newestBlk, rewardPercentiles)
	if err != nil {
		return err
	}

	return checkByJSON(vres, lres)
}

func (cmgr *compareMgr) compareEthMaxPriorityFeePerGas() error {
	vres, err := cmgr.vAPI.EthMaxPriorityFeePerGas(cmgr.ctx)
	if err != nil {
		return err
	}
	lres, err := cmgr.lAPI.EthMaxPriorityFeePerGas(cmgr.ctx)
	if err != nil {
		return err
	}
	fmt.Printf("compareEthMaxPriorityFeePerGas gas premium %d %d\n", vres, lres)

	// return ethBigIntEqual(vres, lres)
	return nil
}

func (cmgr *compareMgr) compareEthEstimateGas() error {
	check := func(vcall types.EthCall, lcall ethtypes.EthCall) error {
		vgas, err := cmgr.vAPI.EthEstimateGas(cmgr.ctx, vcall)
		if err != nil {
			return err
		}
		lgas, err := cmgr.lAPI.EthEstimateGas(cmgr.ctx, lcall)
		if err != nil {
			return err
		}
		if vgas != types.EthUint64(lgas) {
			return fmt.Errorf("not match %d != %d", vgas, lgas)
		}

		return nil
	}

	if err := check(types.EthCall{}, ethtypes.EthCall{}); err != nil {
		return err
	}

	vfrom, err := types.EthAddressFromFilecoinAddress(cmgr.dp.defaultMiner())
	if err != nil {
		return err
	}
	vcall := types.EthCall{
		From:  &vfrom,
		To:    &vfrom,
		Value: types.EthBigInt{Int: big.NewInt(10)},
	}

	lfrom, err := ethtypes.EthAddressFromFilecoinAddress(cmgr.dp.defaultMiner())
	if err != nil {
		return err
	}
	lcall := ethtypes.EthCall{
		From:  &lfrom,
		To:    &lfrom,
		Value: ethtypes.EthBigInt{Int: big.NewInt(10)},
	}

	return check(vcall, lcall)
}

func (cmgr *compareMgr) compareEthCall() error {
	check := func(vcall types.EthCall, lcall ethtypes.EthCall, blkParam string) error {
		vres, err := cmgr.vAPI.EthCall(cmgr.ctx, vcall, blkParam)
		if err != nil {
			return err
		}
		lres, err := cmgr.lAPI.EthCall(cmgr.ctx, lcall, blkParam)
		if err != nil {
			return err
		}
		if !bytes.Equal(vres, lres) {
			return fmt.Errorf("not match %v != %v", vres, lres)
		}

		return nil
	}

	if err := check(types.EthCall{}, ethtypes.EthCall{}, blkParamsLatest); err != nil {
		return err
	}

	vfrom, err := types.EthAddressFromFilecoinAddress(cmgr.dp.defaultMiner())
	if err != nil {
		return err
	}
	vcall := types.EthCall{
		From:  &vfrom,
		To:    &vfrom,
		Value: types.EthBigInt{Int: big.NewInt(10)},
	}

	lfrom, err := ethtypes.EthAddressFromFilecoinAddress(cmgr.dp.defaultMiner())
	if err != nil {
		return err
	}
	lcall := ethtypes.EthCall{
		From:  &lfrom,
		To:    &lfrom,
		Value: ethtypes.EthBigInt{Int: big.NewInt(10)},
	}

	return check(vcall, lcall, blkParamsLatest)
}

func (cmgr *compareMgr) compareEthSendRawTransaction() error {

	return nil
}
