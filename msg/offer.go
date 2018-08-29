package msg

type Offer struct {
	JsonRpc string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  OfferParams `json:"params"`
	Id      string      `json:"id"`
}

type OfferParams struct {
	Contract  Contract `json:"contract"`
	Contact   Contact  `json:"contact"`
	Nonce     int      `json:"nonce"`
	Signature string   `json:"signature"`
}

func (m *Offer) IsValid() bool {
	return true
}

func (m *Offer) GetId() string {
	return m.Id
}

func (m *Offer) SetId(id string) {
	m.Id = id
}

func (m *Offer) SetNonce(n int) {
	m.Params.Nonce = n
}

func (m *Offer) SetSignature(sig string) {
	m.Params.Signature = sig
}
