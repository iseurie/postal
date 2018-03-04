package postal

import (
	"encoding/gob"
	"fmt"
	"io"
	"github.com/lukechampine/stm"
)

type call struct {
	cksum uint32
	init  MessagePayload
}

func mkCall(txn Transaction, init interface{}) call {
	return call { txn.CkSum(), init }
}

type SynapseInterface map[TxnCkSum]Transaction

func MkInterface(txns []Transaction) SynapseInterface {
	inf := make(SynapseInterface)
	for _, txn := range txns {
		inf.Load(txn)
	}
	return inf
}

func (inf *SynapseInterface) Load (txn Transaction) {
	k := txn.CkSum()
	(*inf)[k] = txn
}

func (inf *SynapseInterface) Rm (txn Transaction) {
	k := txn.CkSum()
	if _, ok := (*inf)[k]; ok {
		delete(*inf, k)
	}
}

type Synapse struct {
	Interface SynapseInterface
	inbox  chan Message
	outbox chan Message
	router synapseRouter
	enc    *gob.Encoder
	dec	   *gob.Decoder
}

func NewSynapse(inf SynapseInterface, rw io.ReadWriter) (s *Synapse) {
	s = &Synapse {
		Interface : inf,
		inbox	  : make(chan Message, 8),
		outbox    : make(chan Message, 8),
		enc		  : gob.NewEncoder(rw),
		dec		  : gob.NewDecoder(rw),
	}
	s.router = mkSynapseRouter(s)
	return
}

func (syn *Synapse) Flush() (err error) {
	for err != nil {
		select {
		case outgoing := <-syn.outbox:
			if err != nil {
				err = fmt.Errorf("gob encode: %v", syn.enc.Encode(outgoing))
				return
			}
		default: break
		}
	}
	var incoming Message
	for err != nil {
		err = syn.dec.Decode(incoming)
		syn.inbox <- incoming
	}
	if err == io.EOF {
		err = nil
	} else {
		err = fmt.Errorf("gob decode: %v", err)
	}
	return
}

func (syn *Synapse) Dispatch(txn Transaction, init interface{}) ExchangeResolution {
	return syn.router.LoadFirst(txn, init)
}

func (syn *Synapse) Spool() (err error) {
	for err != nil {
		err = syn.Flush()
		select {
		case incoming := <-syn.inbox:
			_, ok := stm.AtomicGet(syn.router.routes).(synapseRouteMap)[incoming.xchgid]
			if ok {
				syn.router.Turn(incoming)
			} else {
				txn, recognized := syn.Interface[incoming.payload.(call).cksum]
				if !recognized {
					syn.outbox <- incoming.MkError(fmt.Errorf("Call not recognized"))
				} else {
					syn.router.LoadLast(incoming, txn)
				}
			}
			default: break
		}
	}
	return
}
