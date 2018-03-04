package postal

import (
	"testing"
	"bytes"
	"github.com/stretchr/testify/assert"
)

func TestSynapticParity(t *testing.T) {
	rtn := "this is a triumph"
	basetxn, err := NewTransaction(
		TxnStanzas {},
		TxnStanzas {
			func(incoming Message) (Message, error) {
				return incoming.MkReply(incoming.payload.(string)), nil
			},
		})
	if err != nil {
		t.Errorf("Create transaction: %v", err)
	}
	inf := make(SynapseInterface)
	inf.Load(basetxn)
	var transport bytes.Buffer
	syn := NewSynapse(inf, &transport)
	promise := syn.Dispatch(basetxn, rtn)
	syn.Spool()
	msg, err := promise.Force()
	if err != nil {
		t.Errorf("Force promise: %v", err)
	}
	assert.Equal(t, msg.payload.(string), rtn)
}
