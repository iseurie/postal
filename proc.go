package postal

import (
	"math/rand"
	"fmt"
	"encoding/binary"
	"runtime"
	"reflect"
	"hash/adler32"
)

type ExchangeId = uint32
type TxnCkSum = uint32
type MessagePayload = interface{}

func GenXchgId() ExchangeId {
	return rand.Uint32()
}

type Message struct {
	xchgid    ExchangeId
	payload   MessagePayload
}

func MkMessage(id ExchangeId, payload interface{}) Message {
	return Message { xchgid: id, payload: payload}
}

func (incoming *Message) MkReply(resp interface{}) Message {
	return Message { xchgid: incoming.xchgid, payload: resp }
}

func (incoming *Message) MkError(reason error) Message {
	return incoming.MkReply(fmt.Errorf("Exchange ID#%d: %v", incoming.xchgid, reason))
}

type TxnStanza = func(Message) (Message, error)
type TxnStanzas = []TxnStanza

type Transaction struct {
	calls TxnStanzas
	resps TxnStanzas
}

func NewTransaction(calls TxnStanzas, resps TxnStanzas) (txn Transaction, err error) {
	if len(resps) != len(calls) + 1 {
		err = fmt.Errorf("Responder stanzas must have length of calling stanzas + 1")
		return
	}
	txn = Transaction { calls, resps }
	return
}

func (txn *Transaction) CkSum() TxnCkSum {
	hasher := adler32.New()
	for _, f := range txn.calls {
		rf := runtime.FuncForPC(reflect.ValueOf(f).Pointer())
		filename, srcline := rf.FileLine(rf.Entry())
		buf := []byte(filename)
		binary.PutVarint(buf, (int64)(srcline))
		hasher.Write(buf)
	}
	return hasher.Sum32()
}
