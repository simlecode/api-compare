package cmd

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

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
	currTS *types.TipSet,
) *compareMgr {
	cmgr := &compareMgr{
		ctx:      ctx,
		vAPI:     vAPI,
		lAPI:     lAPI,
		dp:       dp,
		currTS:   currTS,
		register: r,
		next:     make(chan struct{}, 10),
	}

	return cmgr
}

type compareMgr struct {
	ctx context.Context

	vAPI v1.FullNode
	lAPI api.FullNode

	dp       *dataProvider
	register *register

	currTS   *types.TipSet
	latestTS *types.TipSet

	next chan struct{}
}

func (cmgr *compareMgr) start() {
	if err := cmgr.chainNotify(); err != nil {
		logrus.Fatalf("chain notify error: %v\n", err)
	}

	first := true
	cmgr.next <- struct{}{}

	for {
		select {
		case <-cmgr.ctx.Done():
			logrus.Warn("context done")
			return
		case <-cmgr.next:
			if first {
				first = false
			} else {
				ts, err := cmgr.findNextTS(cmgr.currTS)
				if err != nil {
					logrus.Fatal(err)
				}
				cmgr.currTS = ts

			}
			if err := cmgr.compareAPI(); err != nil {
				logrus.Errorf("compare api error: %v", err)
			}
		}
	}
}

func (cmgr *compareMgr) chainNotify() error {
	notifs, err := cmgr.vAPI.ChainNotify(cmgr.ctx)
	if err != nil {
		return err
	}

	select {
	case noti := <-notifs:
		if len(noti) != 1 {
			return fmt.Errorf("expect hccurrent length 1 but for %d", len(noti))
		}

		if noti[0].Type != types.HCCurrent {
			return fmt.Errorf("expect hccurrent event but got %s ", noti[0].Type)
		}
		cmgr.latestTS = noti[0].Val
	case <-cmgr.ctx.Done():
		return cmgr.ctx.Err()
	}

	go func() {
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
	logrus.Infof("start compare %d methods, height %d", len(cmgr.register.funcs), cmgr.currTS.Height())

	sorted := make([]struct {
		name string
		f    rfunc
	}, 0, len(cmgr.register.funcs))

	for name, f := range cmgr.register.funcs {
		sorted = append(sorted, struct {
			name string
			f    rfunc
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
			cmgr.printResult(name, f())
		}()

	}
	wg.Wait()

	logrus.Infof("end compare methods took %v\n", time.Since(start))

	return nil
}

func (cmgr *compareMgr) printResult(method string, err error) {
	if err != nil {
		logrus.Errorf("compare %s failed, reason: %v \n", method, err)
	} else {
		logrus.Infof("compare %s success \n", method)
	}
}
