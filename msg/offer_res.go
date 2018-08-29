package msg

type OfferRes struct {
	Result OfferResResult `json:"result"`
	Id     string         `json:"id"`
}

type OfferResResult struct {
	Contract  Contract `json:"contract"`
	Contact   Contact  `json:"contact"`
	Nonce     int      `json:"nonce"`
	Signature string   `json:"signature"`
}

func (m *OfferRes) IsValid() bool {
	return true
}

func (m *OfferRes) GetId() string {
	return m.Id
}

func (m *OfferRes) SetId(id string) {
	m.Id = id
}

func (m *OfferRes) SetNonce(n int) {
	m.Result.Nonce = n
}

func (m *OfferRes) SetSignature(sig string) {
	m.Result.Signature = sig
}
