package msg

type Publish struct {
	JsonRpc string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  PublishParams `json:"params"`
	Id      string        `json:"id"`
}

type PublishParams struct {
	Uuid       string   `json:"uuid"`
	Topic      string   `json:"topic"`
	Contents   Contract `json:"contents"`
	Publishers []string `json:"publishers"`
	Ttl        int      `json:"ttl"`
	Contact    Contact  `json:"contact"`
	Nonce      int      `json:"nonce"`
	Signature  string   `json:"signature"`
}

func (m *Publish) IsValid() bool {
	return false
}

func (m *Publish) GetId() string {
	return m.Id
}

func (m *Publish) SetId(id string) {
	m.Id = id
}

func (m *Publish) SetNonce(n int) {
	m.Params.Nonce = n
}

func (m *Publish) SetSignature(sig string) {
	m.Params.Signature = sig
}
