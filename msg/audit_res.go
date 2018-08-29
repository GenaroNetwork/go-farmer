package msg

type AuditRes struct {
	Result AuditResResult `json:"result"`
	Id     string         `json:"id"`
}

type AuditResResult struct {
	Proofs    []interface{} `json:"proofs"`
	Contact   Contact       `json:"contact"`
	Nonce     int           `json:"nonce"`
	Signature string        `json:"signature"`
}

func (m *AuditRes) IsValid() bool {
	return true
}

func (m *AuditRes) GetId() string {
	return m.Id
}

func (m *AuditRes) SetId(id string) {
	m.Id = id
}

func (m *AuditRes) SetNonce(n int) {
	m.Result.Nonce = n
}

func (m *AuditRes) SetSignature(sig string) {
	m.Result.Signature = sig
}
