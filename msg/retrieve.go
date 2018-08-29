package msg

type Retrieve struct {
	JsonRpc string         `json:"jsonrpc"`
	Method  string         `json:"method"`
	Params  RetrieveParams `json:"params"`
	Id      string         `json:"id"`
}

type RetrieveParams struct {
	DataHash  string  `json:"data_hash" mapstructure:"data_hash"`
	Contact   Contact `json:"contact"`
	Nonce     int     `json:"nonce"`
	Signature string  `json:"signature"`
}

func (m *Retrieve) IsValid() bool {
	return false
}

func (m *Retrieve) GetId() string {
	return m.Id
}

func (m *Retrieve) SetId(id string) {
	m.Id = id
}

func (m *Retrieve) SetNonce(n int) {
	m.Params.Nonce = n
}

func (m *Retrieve) SetSignature(sig string) {
	m.Params.Signature = sig
}
