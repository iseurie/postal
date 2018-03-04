package postal

import (
	"fmt"
	"github.com/lukechampine/stm"
)

type synapseRouteMap = map[ExchangeId] exchange

type exchange struct {
	stanzas   chan TxnStanza
	future    ExchangeResolution
	first     bool
}

func mkExchange(txn Transaction, first bool) exchange {
	xchg := exchange {
		first  : first,
		future : mkExchangeRes(),
	}
	if first {
		xchg.stanzas = make(chan TxnStanza, len(txn.calls))
	} else {
		xchg.stanzas = make(chan TxnStanza, len(txn.resps))
	}
	for _, f := range txn.calls {
		xchg.stanzas <- f
	}
	return xchg
}

type ExchangeResolution struct {
	err chan error
	res chan *Message
}

func (x ExchangeResolution) Force() (msg *Message, err error) {
	select {
	case msg = <-x.res: return
	case err = <-x.err: return
	}
}

func (x ExchangeResolution) Try() (msg *Message, err error) {
	select {
	case msg = <-x.res: return
	case err = <-x.err: return
	default: return nil, nil
	}
}

func (x ExchangeResolution) resolve(msg *Message, err error) {
	if err != nil {
		x.err<-err
	} else {
		x.res<-msg
	}
	close(x.err)
	close(x.res)
}

func mkExchangeRes() ExchangeResolution {
	return ExchangeResolution {
		err : make(chan error, 1),
		res : make(chan *Message, 1),
	}
}

type synapseRouter struct {
	routes  *stm.Var
	synapse *Synapse
}

func mkSynapseRouter (syn *Synapse) synapseRouter {
	return synapseRouter {
		routes  : stm.NewVar(make(synapseRouteMap)),
		synapse : syn,
	}
}

func (r *synapseRouter) Turn(incoming Message) (ok bool) {
	fmt.Printf("Turning Exchange ID#%d...", incoming.xchgid)
	routes := stm.AtomicGet(r.routes).(synapseRouteMap)
	route, ok := routes[incoming.xchgid]
	if !ok { return }
	select {
	case f := <-route.stanzas:
		outgoing, err := f(incoming)
		if err != nil {
			routes[incoming.xchgid].future.resolve(nil, err)
			r.synapse.outbox <- incoming.MkError(err)
		}
		r.synapse.outbox<-outgoing
	default:
		routes[incoming.xchgid].future.resolve(&incoming, nil)
		stm.Atomically(func(tx *stm.Tx) {
			routes := tx.Get(r.routes).(synapseRouteMap)
			delete(routes, incoming.xchgid)
			tx.Set(r.routes, routes)
		})
	}
	return
}

func (r *synapseRouter) LoadFirst(txn Transaction, init interface{}) ExchangeResolution {
	// Map onto 'incoming' method chain
	fmt.Println("map onto 'incoming' method chain")
	xchg := mkExchange(txn, true)
	id := GenXchgId()
	stm.Atomically(func(tx *stm.Tx) {
		routes := tx.Get(r.routes).(synapseRouteMap)
		routes[id] = xchg
		tx.Set(r.routes, routes)
	})
	// send init payload
	fmt.Println("sending init payload")
	r.synapse.outbox <- Message { id, init }
	fmt.Println("...sent")
	return xchg.future
}

func (r *synapseRouter) LoadLast(incoming Message, txn Transaction) {
	// Map onto 'outgoing' method chain
	stm.Atomically(func(tx *stm.Tx) {
		routes := tx.Get(r.routes).(synapseRouteMap)
		routes[incoming.xchgid] = mkExchange(txn, false)
		tx.Set(r.routes, routes)
	})
}

func(r *synapseRouter) PopXchgStanza (id ExchangeId) (TxnStanza, bool) {
	select {
	case f := <-(stm.AtomicGet(r.routes).(synapseRouteMap)[id]).stanzas:
		return f, true
	default:
		return nil, false
	}
}
