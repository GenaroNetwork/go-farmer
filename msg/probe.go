package msg

type Probe struct {
	JsonRpc string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  ProbeParams `json:"params"`
	Id      string      `json:"id"`
}

type ProbeParams struct {
	Contact   Contact `json:"contact"`
	Nonce     int     `json:"nonce"`
	Signature string  `json:"signature"`
}

func (m *Probe) IsValid() bool {
	return false
}

func (m *Probe) GetId() string {
	return m.Id
}

func (m *Probe) SetId(id string) {
	m.Id = id
}

func (m *Probe) SetNonce(n int) {
	m.Params.Nonce = n
}

func (m *Probe) SetSignature(sig string) {
	m.Params.Signature = sig
}
