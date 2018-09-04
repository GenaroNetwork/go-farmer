package msg

type Mirror struct {
	JsonRpc string       `json:"jsonrpc"`
	Method  string       `json:"method"`
	Params  MirrorParams `json:"params"`
	Id      string       `json:"id"`
}

type MirrorParams struct {
	DataHash  string   `json:"data_hash" mapstructure:"data_hash"`
	Token     string   `json:"token"`
	Farmer    Contact  `json:"farmer"`
	Contact   Contact  `json:"contact"`
	AuditTree []string `json:"audit_tree" mapstructure:"audit_tree"`
	Nonce     int      `json:"nonce"`
	Signature string   `json:"signature"`
}

func (m *Mirror) IsValid() bool {
	return true
}

func (m *Mirror) GetId() string {
	return m.Id
}

func (m *Mirror) SetId(id string) {
	m.Id = id
}

func (m *Mirror) SetNonce(n int) {
	m.Params.Nonce = n
}

func (m *Mirror) SetSignature(sig string) {
	m.Params.Signature = sig
}
