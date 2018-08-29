package msg

type FindNode struct {
	JsonRpc string         `json:"jsonrpc"`
	Method  string         `json:"method"`
	Params  FindNodeParams `json:"params"`
	Id      string         `json:"id"`
}

type FindNodeParams struct {
	Key       string  `json:"key"`
	Contact   Contact `json:"contact"`
	Nonce     int     `json:"nonce"`
	Signature string  `json:"signature"`
}

func (m *FindNode) IsValid() bool {
	return true
}

func (m *FindNode) GetId() string {
	return m.Id
}

func (m *FindNode) SetId(id string) {
	m.Id = id
}

func (m *FindNode) SetNonce(n int) {
	m.Params.Nonce = n
}

func (m *FindNode) SetSignature(sig string) {
	m.Params.Signature = sig
}
