package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/filecoin-project/go-state-types/abi"
	v1 "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/urfave/cli/v2"
)

const (
	defaultConfidence = 5
)

var Commands = cli.Command{}

func Run(cctx *cli.Context) error {
	vURL := cctx.String("venus-url")
	vToken := cctx.String("venus-token")
	lURL := cctx.String("lotus-url")
	lToken := cctx.String("lotus-token")

	fmt.Println("lotus url", lURL, "lotus token", lToken)
	fmt.Println("venus url", vURL, "venus token", vToken)

	ctx, cancel := context.WithCancel(cctx.Context)
	defer cancel()

	vAPI, vClose, err := v1.DialFullNodeRPC(ctx, vURL, vToken, nil)
	if err != nil {
		return fmt.Errorf("create venus rpc error: %v", err)
	}
	defer vClose()

	lAPI, lClose, err := newLotusFullNodeRPCV1(ctx, lURL, lToken)
	if err != nil {
		return fmt.Errorf("create lotus rpc error: %v", err)
	}
	defer lClose()

	head, err := vAPI.ChainHead(ctx)
	if err != nil {
		return err
	}

	var currentTS *types.TipSet
	var startHeight abi.ChainEpoch
	if cctx.IsSet("start-height") {
		startHeight = abi.ChainEpoch(cctx.Int("start-height"))
		if startHeight > head.Height() {
			startHeight = head.Height()
		}
	} else {
		startHeight = head.Height() - abi.ChainEpoch(defaultConfidence)
	}
	if startHeight < 0 {
		startHeight = 0
	}
	currentTS, err = vAPI.ChainGetTipSetAfterHeight(ctx, startHeight, types.EmptyTSK)
	if err != nil {
		return err
	}

	c := make(chan os.Signal, 1)
	go func() {
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	}()

	dp, err := newDataProvider(ctx, vAPI)
	if err != nil {
		return fmt.Errorf("new data provider error: %v", err)
	}

	r := newRegister()
	ac := newAPICompare(ctx, vAPI, lAPI, dp, cctx.Int("concurrency"))
	if err := r.registerAPICompare(ac); err != nil {
		return err
	}

	mgr := newCompareMgr(ctx, vAPI, lAPI, dp, r, currentTS)
	go mgr.start()

	<-c

	return nil
}
