package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/go-state-types/big"
	lapi "github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/api/v1api"
	ltypes "github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/lotus/chain/types/ethtypes"
	"github.com/filecoin-project/venus/venus-shared/api"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs/go-cid"
)

func newLotusFullNodeRPCV1(ctx context.Context, url, token string) (lapi.FullNode, jsonrpc.ClientCloser, error) {
	ainfo := api.NewAPIInfo(url, token)
	endpoint, err := ainfo.DialArgs("v1")
	if err != nil {
		return nil, nil, err
	}

	var res v1api.FullNodeStruct
	closer, err := jsonrpc.NewMergeClient(ctx, endpoint, "Filecoin",
		api.GetInternalStructs(&res), ainfo.AuthHeader())

	return &res, closer, err
}

func toLoutsTipsetKey(key types.TipSetKey) ltypes.TipSetKey {
	if key.IsEmpty() {
		return ltypes.EmptyTSK
	}
	return ltypes.NewTipSetKey(key.Cids()...)
}

func toLotusMsg(msg *types.Message) *ltypes.Message {
	return &ltypes.Message{
		Version:    msg.Version,
		To:         msg.To,
		From:       msg.From,
		Nonce:      msg.Nonce,
		Value:      msg.Value,
		GasLimit:   msg.GasLimit,
		GasFeeCap:  msg.GasFeeCap,
		GasPremium: msg.GasPremium,
		Method:     msg.Method,
		Params:     msg.Params,
	}
}

func toLotusEthMessageMatch(src *types.MessageMatch) lapi.MessageMatch {
	return lapi.MessageMatch{
		From: src.From,
		To:   src.To,
	}
}

func toLotusEthCall(src types.EthCall) ethtypes.EthCall {
	return ethtypes.EthCall{
		From:     (*ethtypes.EthAddress)(src.From),
		To:       (*ethtypes.EthAddress)(src.To),
		Gas:      ethtypes.EthUint64(src.Gas),
		GasPrice: ethtypes.EthBigInt(src.GasPrice),
		Value:    ethtypes.EthBigInt(src.Value),
		Data:     ethtypes.EthBytes(src.Data),
	}
}

func checkByJSON(a, b interface{}) error {
	d, err := json.Marshal(a)
	if err != nil {
		return fmt.Errorf("failed to marshal 'a': %v", err)
	}
	d2, err := json.Marshal(b)
	if err != nil {
		return fmt.Errorf("failed to marshal 'b': %v", err)
	}

	if string(d) == string(d2) {
		return nil
	}

	return fmt.Errorf("json marshal result not match %s != %s", string(d), string(d2))
}

func checkInvocResult(vres *types.InvocResult, lres *lapi.InvocResult) error {
	if err := checkByJSON(vres.MsgRct, lres.MsgRct); err != nil {
		return fmt.Errorf("%+v != %+v", vres.MsgRct, lres.MsgRct)
	}
	if err := checkByJSON(vres.GasCost, lres.GasCost); err != nil {
		return fmt.Errorf("%+v != %+v", vres.GasCost, lres.GasCost)
	}
	if err := checkByJSON(vres.ExecutionTrace, lres.ExecutionTrace); err != nil {
		return fmt.Errorf("%+v != %+v", vres.ExecutionTrace, lres.ExecutionTrace)
	}

	return nil
}

func tsEquals(ts *types.TipSet, ots *ltypes.TipSet) error {
	if ts == nil && ots == nil {
		return nil
	}
	if ts == nil || ots == nil {
		return fmt.Errorf("one is nil %v %v", ts == nil, ots == nil)
	}

	if ts.Height() != ots.Height() {
		return fmt.Errorf("heith %d != %d", ts.Height(), ots.Height())
	}

	if len(ts.Cids()) != len(ots.Cids()) {
		return fmt.Errorf("block length %d != %d", len(ts.Cids()), len(ots.Cids()))
	}

	for i, cid := range ts.Cids() {
		if cid != ots.Cids()[i] {
			return fmt.Errorf("block %s != %s", cid, ots.Cids()[i])
		}
	}

	return nil
}

func bigIntEqual(a, b *big.Int) error {
	if a == nil && b == nil {
		return nil
	}
	if a == nil || b == nil {
		return fmt.Errorf("not match %v != %v", a, b)
	}
	if a.Int == nil && b.Int == nil {
		return nil
	}

	if (a.Int == nil || b.Int == nil) || !a.Equals(*b) {
		return fmt.Errorf("not match %v != %v", a, b)
	}

	return nil
}

func equal(a, b interface{}) bool {
	abytes, ok := a.([]byte)
	bbytes, ok2 := b.([]byte)
	if ok || ok2 {
		if ok && ok2 {
			return bytes.Equal(abytes, bbytes)
		} else {
			return false
		}
	}

	av := reflect.ValueOf(a)
	bv := reflect.ValueOf(b)

	if av.Kind() != bv.Kind() {
		return false
	}

	switch av.Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Interface, reflect.Pointer, reflect.Slice:
		if av.IsNil() || bv.IsNil() {
			if av.IsNil() && bv.IsNil() {
				return true
			}
			return false
		}
	}

	if !av.IsValid() || !bv.IsValid() {
		if !av.IsValid() && bv.IsValid() {
			return true
		}
		return false
	}

	if av.Kind() == reflect.Pointer {
		av = av.Elem()
		bv = bv.Elem()
	}

	switch av.Kind() {
	case reflect.Struct:
		for i := 0; i < av.NumField(); i++ {
			val := av.Field(i)
			name := av.Type().Field(i).Name
			val2 := bv.FieldByName(name)

			if !av.Type().Field(i).IsExported() {
				return equalJSONMarshal(a, b)
			}

			if !equal(val.Interface(), val2.Interface()) {
				return false
			}
		}
	case reflect.Slice:
		if av.Len() != bv.Len() {
			return false
		}
		for i := 0; i < av.Len(); i++ {
			val := av.Index(i)
			val2 := bv.Index(i)

			if !equal(val, val2) {
				return false
			}
		}
	case reflect.Map:
		if av.Len() != bv.Len() {
			return false
		}
		iter := av.MapRange()
		for iter.Next() {
			key := iter.Key()
			val := iter.Value()
			val2 := bv.MapIndex(key)

			if !equal(val, val2) {
				return false
			}
		}
	default:
		if av.Interface() != bv.Interface() {
			return false
		}
	}

	return true
}

func equalJSONMarshal(a, b interface{}) bool {
	data, err := json.Marshal(a)
	if err != nil {
		fmt.Println("marshal failed: ", a)
		return false
	}
	data2, err := json.Marshal(b)
	if err != nil {
		fmt.Println("marshal failed: ", b)
		return false
	}
	return string(data) == string(data2)
}

func resultCheckWithEqual(o1, o2 interface{}) error {
	if !equal(o1, o2) {
		return fmt.Errorf("not match obj1 %+v, obj2 %+v", o1, o2)
	}
	return nil
}

func resultCheckWithInvocResult(msg cid.Cid, o1, o2 interface{}) error {
	r1, _ := o1.(*types.InvocResult)
	r2, _ := o2.(*lapi.InvocResult)

	if err := checkInvocResult(r1, r2); err != nil {
		return fmt.Errorf("msg %s, \nerror: %v", msg, err)
	}

	return nil
}

func toInterface(objs ...interface{}) []interface{} {
	i := make([]interface{}, 0, len(objs))
	i = append(i, objs...)

	return i
}