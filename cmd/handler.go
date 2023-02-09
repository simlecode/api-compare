package cmd

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	lapi "github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/chain/types/ethtypes"
	vapi "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/sirupsen/logrus"
)

func newHandler(ctx context.Context, vAPI vapi.FullNode, lAPI lapi.FullNode, concurrency int) *handler {
	h := &handler{
		ctx:         ctx,
		concurrency: concurrency,

		vAPI: apiInfo{
			rv: reflect.ValueOf(vAPI),
			rt: reflect.TypeOf(vAPI),
		},
		lAPI: apiInfo{
			rv: reflect.ValueOf(lAPI),
			rt: reflect.TypeOf(lAPI),
		},

		receiver: make(chan *req, 20),
	}

	go h.start()

	return h
}

type handler struct {
	ctx         context.Context
	concurrency int

	vAPI apiInfo
	lAPI apiInfo

	receiver chan *req
}

type apiInfo struct {
	rv reflect.Value
	rt reflect.Type
}

type req struct {
	methodName string
	in         []interface{}
	err        chan error

	// option
	resultChecker      resultCheckFunc
	expectCallAPIError bool
}

type reqOpt func(*req)

func newReq(methodName string, in []interface{}, opts ...reqOpt) *req {
	r := &req{
		methodName: methodName,
		in:         in,
		err:        make(chan error, 1),
	}
	for _, opt := range opts {
		opt(r)
	}

	return r
}

func withResultCheck(f resultCheckFunc) reqOpt {
	return func(r *req) {
		r.resultChecker = f
	}
}

func withExpectCallAPIError() reqOpt {
	return func(r *req) {
		r.expectCallAPIError = true
	}
}

type resultCheckFunc func(r1, r2 interface{}) error

func (h *handler) start() {
	controlCh := make(chan struct{}, h.concurrency)
	done := func() {
		<-controlCh
	}
	for {
		select {
		case <-h.ctx.Done():
			logrus.Warn("context done, stop handler req")
			return
		case r := <-h.receiver:
			controlCh <- struct{}{}
			go func() {
				defer done()

				r.err <- h.compare(r)
				close(r.err)
			}()
		}
	}
}

func (h *handler) compare(r *req) error {
	logrus.Debugf("start handler compare %v", r.methodName)
	defer func() {
		logrus.Debugf("end handler compare %v", r.methodName)
	}()
	vm, ok := h.vAPI.rv.Type().MethodByName(r.methodName)
	if !ok {
		return fmt.Errorf("not found method %s", r.methodName)
	}
	lm, ok := h.lAPI.rv.Type().MethodByName(r.methodName)
	if !ok {
		return fmt.Errorf("not found method %s", r.methodName)
	}

	inParams := make([]reflect.Value, 0, len(r.in))
	inParams2 := make([]reflect.Value, 0, len(r.in))
	for i, param := range r.in {
		v := reflect.ValueOf(param)
		inParams = append(inParams, v)
		// The first parameter is usually context.Context
		if i == 0 {
			inParams2 = append(inParams2, v)
			continue
		}
		inParams2 = append(inParams2, reflect.ValueOf(tryConvertParam(param)))
	}

	var vres, lres []reflect.Value
	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()
		vres = vm.Func.Call(append([]reflect.Value{h.vAPI.rv}, inParams...))
	}()
	go func() {
		defer wg.Done()
		lres = lm.Func.Call(append([]reflect.Value{h.lAPI.rv}, inParams2...))
	}()
	wg.Wait()

	if len(vres) == 2 && vres[1].Interface() != nil && !r.expectCallAPIError {
		return fmt.Errorf("venus call %s error %v", r.methodName, vres[1].Interface())
	}
	if len(lres) == 2 && lres[1].Interface() != nil && !r.expectCallAPIError {
		return fmt.Errorf("lotus call %s error %v", r.methodName, lres[1].Interface())
	}
	logrus.Tracef("call %s result: \n%+v \n%+v", r.methodName, vres[0].Interface(), lres[0].Interface())

	if r.resultChecker != nil {
		return r.resultChecker(vres[0].Interface(), lres[0].Interface())
	}

	return checkByJSON(vres[0].Interface(), lres[0].Interface())
}

// todo: not check each param
func tryConvertParam(param interface{}) interface{} {
	vkey, ok := param.(types.TipSetKey)
	if ok {
		return toLoutsTipsetKey(types.TipSetKey(vkey))
	}
	vmsg, ok := param.(*types.Message)
	if ok {
		return toLotusMsg(vmsg)
	}
	vnum, ok := param.(types.EthUint64)
	if ok {
		return ethtypes.EthUint64(vnum)
	}
	vhash, ok := param.(types.EthHash)
	if ok {
		return ethtypes.EthHash(vhash)
	}
	vptrHash, ok := param.(*types.EthHash)
	if ok {
		return (*ethtypes.EthHash)(vptrHash)
	}
	vaddr, ok := param.(types.EthAddress)
	if ok {
		return ethtypes.EthAddress(vaddr)
	}
	vcall, ok := param.(types.EthCall)
	if ok {
		return toLotusEthCall(vcall)
	}
	vbytes, ok := param.(types.EthBytes)
	if ok {
		return ethtypes.EthBytes(vbytes)
	}
	vmsgMatch, ok := param.(*types.MessageMatch)
	if ok {
		return toLotusEthMessageMatch(vmsgMatch)
	}

	return param
}

func (h *handler) send(r *req) {
	select {
	case <-h.ctx.Done():
		r.err <- h.ctx.Err()
		return
	default:
	}

	h.receiver <- r
}
