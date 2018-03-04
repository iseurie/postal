- Synapse: Generic class, specialize for different types of node idents
  - handshake(instance, ident): generic, specialize for different types of node idents (e.g. IRC)
  - send(x): Pushes 'x' onto the cluster and returns a promise for a response
  - receive: Buffered CSP channel slot; fills as the cl-async event loop runs and poisoned upon loss of connection

Exchange:
- Send
- Wait for reply of appropriate type
- Decode, send response based on process function

Exchange (peer call/response chain):
- Send initial payload
- For each function
  - Take response, process, send return
  - If any error:
    - Stop chaining
	- Notify peer
	- Cease exchange
