package msg

type RetrieveRes struct {
	Result RetrieveResResult `json:"result"`
	Id     string            `json:"id"`
}

type RetrieveResResult struct {
	Token     string  `json:"token"`
	Contact   Contact `json:"contact"`
	Nonce     int     `json:"nonce"`
	Signature string  `json:"signature"`
}

func (m *RetrieveRes) IsValid() bool {
	return true
}

func (m *RetrieveRes) GetId() string {
	return m.Id
}

func (m *RetrieveRes) SetId(id string) {
	m.Id = id
}

func (m *RetrieveRes) SetNonce(n int) {
	m.Result.Nonce = n
}

func (m *RetrieveRes) SetSignature(sig string) {
	m.Result.Signature = sig
}
