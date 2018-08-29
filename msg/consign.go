package msg

type Consign struct {
	JsonRpc string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  ConsignParams `json:"params"`
	Id      string        `json:"id"`
}

type ConsignParams struct {
	DataHash  string   `json:"data_hash" mapstructure:"data_hash"`
	AuditTree []string `json:"audit_tree" mapstructure:"audit_tree"`
	Contact   Contact  `json:"contact"`
	Nonce     int      `json:"nonce"`
	Signature string   `json:"signature"`
}

func (m *Consign) IsValid() bool {
	return false
}

func (m *Consign) GetId() string {
	return m.Id
}

func (m *Consign) SetId(id string) {
	m.Id = id
}

func (m *Consign) SetNonce(n int) {
	m.Params.Nonce = n
}

func (m *Consign) SetSignature(sig string) {
	m.Params.Signature = sig
}
