package msg

type Ping struct {
	JsonRpc string     `json:"jsonrpc"`
	Method  string     `json:"method"`
	Params  PingParams `json:"params"`
	Id      string     `json:"id"`
}

type PingParams struct {
	Contact   Contact `json:"contact"`
	Nonce     int     `json:"nonce"`
	Signature string  `json:"signature"`
}

func (m *Ping) IsValid() bool {
	return true
}

func (m *Ping) GetId() string {
	return m.Id
}

func (m *Ping) SetId(id string) {
	m.Id = id
}

func (m *Ping) SetNonce(n int) {
	m.Params.Nonce = n
}

func (m *Ping) SetSignature(sig string) {
	m.Params.Signature = sig
}
