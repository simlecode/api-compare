package cmd

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/lotus/api"
	ltypes "github.com/filecoin-project/lotus/chain/types"
	v1 "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/sirupsen/logrus"
)

func newCompareMgr(ctx context.Context,
	vAPI v1.FullNode,
	lAPI api.FullNode,
	dp *dataProvider,
	r *register,
	currentTS *types.TipSet,
) *compareMgr {
	mgr := &compareMgr{
		ctx:       ctx,
		vAPI:      vAPI,
		lAPI:      lAPI,
		dp:        dp,
		currentTS: currentTS,
		register:  r,
		next:      make(chan struct{}, 10),
	}

	return mgr
}

type compareMgr struct {
	ctx context.Context

	vAPI v1.FullNode
	lAPI api.FullNode

	dp       *dataProvider
	register *register

	currentTS *types.TipSet

	next chan struct{}
}

func (mgr *compareMgr) start() {
	if err := mgr.chainNotify(); err != nil {
		logrus.Fatalf("chain notify error: %v\n", err)
	}

	compare := func(h abi.ChainEpoch) {
		ts, err := mgr.findTSByHeight(h)
		if err != nil {
			logrus.Errorf("found ts failed %v error %v", h, err)
			return
		}
		mgr.currentTS = ts

		if err := mgr.compareAPI(); err != nil {
			logrus.Errorf("compare api error: %v", err)
		}
	}

	h := mgr.currentTS.Height() - defaultConfidence
	if h < 0 {
		h = 0
	}
	compare(h)

	for {
		select {
		case <-mgr.ctx.Done():
			logrus.Warn("context done")
			return
		case <-mgr.next:
			compare(mgr.currentTS.Height() + 1)
		}
	}
}

func (mgr *compareMgr) chainNotify() error {
	notifies, err := mgr.vAPI.ChainNotify(mgr.ctx)
	if err != nil {
		return err
	}

	select {
	case notify := <-notifies:
		if len(notify) != 1 {
			return fmt.Errorf("expect hccurrent length 1 but for %d", len(notify))
		}

		if notify[0].Type != types.HCCurrent {
			return fmt.Errorf("expect hccurrent event but got %s ", notify[0].Type)
		}
	case <-mgr.ctx.Done():
		return mgr.ctx.Err()
	}

	go func() {
		for notify := range notifies {
			var apply []*types.TipSet

			for _, change := range notify {
				switch change.Type {
				case types.HCApply:
					apply = append(apply, change.Val)
				}
			}
			if apply[0].Height() > (mgr.currentTS.Height() + defaultConfidence) {
				mgr.next <- struct{}{}
			}
		}
	}()

	return nil
}

func (mgr *compareMgr) findTSByHeight(h abi.ChainEpoch) (*types.TipSet, error) {
	vts, err := mgr.vAPI.ChainGetTipSetAfterHeight(mgr.ctx, h, types.EmptyTSK)
	if err != nil {
		return nil, err
	}
	lts, err := mgr.lAPI.ChainGetTipSetAfterHeight(mgr.ctx, h, ltypes.EmptyTSK)
	if err != nil {
		return nil, err
	}

	if vts.Height() != lts.Height() {
		return nil, fmt.Errorf("height not match %d != %d", vts.Height(), lts.Height())
	}
	if !vts.Key().Equals(types.NewTipSetKey(lts.Cids()...)) {
		return nil, fmt.Errorf("key not match %v != %v", vts.Key(), lts.Key())
	}

	return vts, nil
}

func (mgr *compareMgr) compareAPI() error {
	if err := mgr.dp.reset(mgr.currentTS); err != nil {
		return err
	}
	logrus.Infof("start compare %d methods, height %d", len(mgr.register.funcs), mgr.currentTS.Height())

	sorted := make([]struct {
		name string
		f    rf
	}, 0, len(mgr.register.funcs))

	for name, f := range mgr.register.funcs {
		sorted = append(sorted, struct {
			name string
			f    rf
		}{name: name, f: f})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].name > sorted[j].name
	})
	for _, v := range sorted {
		logrus.Debugf(v.name)
	}

	start := time.Now()
	wg := sync.WaitGroup{}
	for _, v := range sorted {
		wg.Add(1)

		name := v.name
		f := v.f
		go func() {
			defer wg.Done()
			mgr.printResult(name, f())
		}()

	}
	wg.Wait()

	logrus.Infof("end compare methods took %v\n\n", time.Since(start))

	return nil
}

func (mgr *compareMgr) printResult(method string, err error) {
	if err != nil {
		logrus.Errorf("compare %s failed: %v \n", method, err)
	} else {
		logrus.Infof("compare %s success \n", method)
	}
}
