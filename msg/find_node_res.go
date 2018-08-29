package msg

type FindNodeRes struct {
	Result FindNodeResResult `json:"result"`
	Id     string            `json:"id"`
}

type FindNodeResResult struct {
	Nodes     []Contact `json:"nodes"`
	Contact   Contact   `json:"contact"`
	Nonce     int       `json:"nonce"`
	Signature string    `json:"signature"`
}

func (m *FindNodeRes) IsValid() bool {
	return true
}

func (m *FindNodeRes) GetId() string {
	return m.Id
}

func (m *FindNodeRes) SetId(id string) {
	m.Id = id
}

func (m *FindNodeRes) SetNonce(n int) {
	m.Result.Nonce = n
}

func (m *FindNodeRes) SetSignature(sig string) {
	m.Result.Signature = sig
}
