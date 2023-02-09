package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRegisterAPICompare(t *testing.T) {
	ac := &apiCompare{}
	r := newRegister()

	assert.NoError(t, r.registerAPICompare(ac))
	assert.GreaterOrEqual(t, len(r.funcs), 1)
}
