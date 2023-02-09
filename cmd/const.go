package cmd

import (
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/lotus/chain/types/ethtypes"
	"github.com/filecoin-project/venus/venus-shared/types"
)

const latestNetworkVersion = network.Version17

const (
	stateAccountKey              = "StateAccountKey"
	chainGetTipSet               = "ChainGetTipSet"
	chainGetTipSetByHeight       = "ChainGetTipSetByHeight"
	stateGetRandomnessFromBeacon = "StateGetRandomnessFromBeacon"
	stateGetBeaconEntry          = "StateGetBeaconEntry"
	chainGetBlock                = "ChainGetBlock"
	chainGetBlockMessages        = "ChainGetBlockMessages"
	chainGetMessage              = "ChainGetMessage"
	chainGetMessagesInTipset     = "ChainGetMessagesInTipset"
	chainGetParentMessages       = "ChainGetParentMessages"
	chainGetParentReceipts       = "ChainGetParentReceipts"
	stateVerifiedRegistryRootKey = "StateVerifiedRegistryRootKey"
	stateVerifierStatus          = "StateVerifierStatus"
	stateNetworkName             = "StateNetworkName"
	stateSearchMsg               = "StateSearchMsg"
	stateWaitMsg                 = "StateWaitMsg"
	stateNetworkVersion          = "StateNetworkVersion"
	chainGetPath                 = "ChainGetPath"
	stateGetNetworkParams        = "StateGetNetworkParams"
	stateActorCodeCIDs           = "StateActorCodeCIDs"
	chainGetGenesis              = "ChainGetGenesis"
	stateActorManifestCID        = "StateActorManifestCID"
	stateCall                    = "StateCall"
	stateReplay                  = "StateReplay"
	minerGetBaseInfo             = "MinerGetBaseInfo"

	// state
	stateReadState    = "StateReadState"
	stateListMessages = "StateListMessages"
	stateDecodeParams = "StateDecodeParams"

	// eth
	ethAccounts                            = "EthAccounts"
	ethBlockNumber                         = "EthBlockNumber"
	ethGetBlockTransactionCountByNumber    = "EthGetBlockTransactionCountByNumber"
	ethGetBlockTransactionCountByHash      = "EthGetBlockTransactionCountByHash"
	ethGetBlockByHash                      = "EthGetBlockByHash"
	ethGetBlockByNumber                    = "EthGetBlockByNumber"
	ethGetTransactionByHash                = "EthGetTransactionByHash"
	ethGetTransactionCount                 = "EthGetTransactionCount"
	ethGetTransactionReceipt               = "EthGetTransactionReceipt"
	ethGetTransactionByBlockHashAndIndex   = "EthGetTransactionByBlockHashAndIndex"
	ethGetTransactionByBlockNumberAndIndex = "EthGetTransactionByBlockNumberAndIndex"
	ethGetCode                             = "EthGetCode"
	ethGetStorageAt                        = "EthGetStorageAt"
	ethGetBalance                          = "EthGetBalance"
	ethChainId                             = "EthChainId"
	netVersion                             = "NetVersion"
	netListening                           = "NetListening"
	ethProtocolVersion                     = "EthProtocolVersion"
	ethGasPrice                            = "EthGasPrice"
	ethFeeHistory                          = "EthFeeHistory"
	ethMaxPriorityFeePerGas                = "EthMaxPriorityFeePerGas"
	ethEstimateGas                         = "EthEstimateGas"
	ethCall                                = "EthCall"
	web3ClientVersion                      = "Web3ClientVersion"
	ethGetTransactionHashByCid             = "EthGetTransactionHashByCid"
	ethGetMessageCidByTransactionHash      = "EthGetMessageCidByTransactionHash"
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
