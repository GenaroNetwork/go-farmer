package msg

import (
	"github.com/mitchellh/mapstructure"
	"encoding/json"
)

const (
	MFindNode = "FIND_NODE"
	MPing     = "PING"
	MProbe    = "PROBE"
	MOffer    = "OFFER"
	MPublish  = "PUBLISH"
	MConsign  = "CONSIGN"
	MRetrieve = "RETRIEVE"
	MMirror   = "MIRROR"
	MAudit    = "AUDIT"
)

// Contact of a node
type Contact struct {
	Address  string `json:"address"`
	Port     uint16 `json:"port"`
	NodeID   string `json:"nodeID"`
	Protocol string `json:"protocol"`
}

func (c *Contact) IsValid() bool {
	if c.Address == "" || c.Port == 0 || c.NodeID == "" || c.Protocol == "" {
		return false
	}
	return true
}

// Contract between farmer and renter
type Contract struct {
	//Contact              Contact `json:"contact"`
	RenterID             string  `json:"renter_id" mapstructure:"renter_id"`
	RenterSignature      string  `json:"renter_signature" mapstructure:"renter_signature"`
	RenterHDIndex        int     `json:"renter_hd_index" mapstructure:"renter_hd_index"`
	RenterHDKey          string  `json:"renter_hd_key" mapstructure:"renter_hd_key"`
	FarmerID             *string `json:"farmer_id" mapstructure:"farmer_id"`
	FarmerSignature      *string `json:"farmer_signature" mapstructure:"farmer_signature"`
	DataSize             int     `json:"data_size" mapstructure:"data_size"`
	DataHash             string  `json:"data_hash" mapstructure:"data_hash"`
	StoreBegin           int     `json:"store_begin" mapstructure:"store_begin"`
	StoreEnd             int     `json:"store_end" mapstructure:"store_end"`
	AuditCount           int     `json:"audit_count" mapstructure:"audit_count"`
	PaymentStoragePrice  int     `json:"payment_storage_price" mapstructure:"payment_storage_price"`
	PaymentDownloadPrice int     `json:"payment_download_price" mapstructure:"payment_download_price"`
	PaymentDestination   *string `json:"payment_destination" mapstructure:"payment_destination"`
	Version              int     `json:"version" mapstructure:"version"`
}

func (c *Contract) Stringify() string {
	m := make(map[string]interface{})
	mapstructure.Decode(c, &m)
	delete(m, "renter_signature")
	delete(m, "farmer_signature")
	s, _ := json.Marshal(m)
	return string(s)
}

func (c *Contract) IsValid() (valid bool) {
	valid = false
	if c.RenterID == "" ||
		c.RenterSignature == "" ||
		c.DataSize == 0 ||
		c.DataHash == "" ||
		c.StoreBegin == 0 || c.StoreEnd == 0 || c.StoreBegin >= c.StoreEnd {
		return
	}
	return true
}

// general response for
// PING, PROBE, PUBLISH, MIRROR
type Res struct {
	Result ResResult `json:"result"`
	Id     string    `json:"id"`
}

type ResResult struct {
	Contact   Contact `json:"contact"`
	Nonce     int     `json:"nonce"`
	Signature string  `json:"signature"`
}

func (m *Res) IsValid() bool {
	return true
}

func (m *Res) GetId() string {
	return m.Id
}

func (m *Res) SetId(id string) {
	m.Id = id
}

func (m *Res) SetNonce(n int) {
	m.Result.Nonce = n
}

func (m *Res) SetSignature(sig string) {
	m.Result.Signature = sig
}

// general error response
type ResErr struct {
	Res
	Error ResErrError `json:"error"`
}

type ResErrError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (m *ResErr) IsValid() bool {
	return true
}

func (m *ResErr) GetId() string {
	return m.Id
}

func (m *ResErr) SetId(id string) {
	m.Id = id
}

func (m *ResErr) SetNonce(n int) {
	m.Result.Nonce = n
}

func (m *ResErr) SetSignature(sig string) {
	m.Result.Signature = sig
}

func NewResErr(contact Contact, msg string) *ResErr {
	return &ResErr{
		Res: Res{
			Result: ResResult{
				Contact: contact,
			},
		},
		Error: ResErrError{
			Code:    -1,
			Message: msg,
		},
	}
}
