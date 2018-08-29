package msg

type ConsignRes struct {
	Result ConsignResResult `json:"result"`
	Id     string           `json:"id"`
}

type ConsignResResult struct {
	Token     string  `json:"token"`
	Contact   Contact `json:"contact"`
	Nonce     int     `json:"nonce"`
	Signature string  `json:"signature"`
}

func (m *ConsignRes) IsValid() bool {
	return true
}

func (m *ConsignRes) GetId() string {
	return m.Id
}

func (m *ConsignRes) SetId(id string) {
	m.Id = id
}

func (m *ConsignRes) SetNonce(n int) {
	m.Result.Nonce = n
}

func (m *ConsignRes) SetSignature(sig string) {
	m.Result.Signature = sig
}
