package msg

type Audit struct {
	JsonRpc string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  AuditParams `json:"params"`
	Id      string      `json:"id"`
}

type AuditParams struct {
	Audits    []AuditParamAudit `json:"audits"`
	Contact   Contact           `json:"contact"`
	Nonce     int               `json:"nonce"`
	Signature string            `json:"signature"`
}

type AuditParamAudit struct {
	DataHash  string `json:"data_hash" mapstructure:"data_hash"`
	Challenge string `json:"challenge"`
}

func (m *Audit) IsValid() bool {
	return true
}

func (m *Audit) GetId() string {
	return m.Id
}

func (m *Audit) SetId(id string) {
	m.Id = id
}

func (m *Audit) SetNonce(n int) {
	m.Params.Nonce = n
}

func (m *Audit) SetSignature(sig string) {
	m.Params.Signature = sig
}
