package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/GenaroNetwork/go-farmer/crypto"
	"github.com/GenaroNetwork/go-farmer/msg"
	"github.com/patrickmn/go-cache"
	"github.com/satori/go.uuid"
	"golang.org/x/crypto/ripemd160"
)

var contractCache *cache.Cache
var mirrorCache *cache.Cache

type storageItem struct {
	Contract msg.Contract `json:"contract"`
	//Shard bool `json:"shard"`
	Trees []string `json:"trees"`
}

func init() {
	contractCache = cache.New(2*time.Minute, 5*time.Minute)
	mirrorCache = cache.New(2*time.Minute, 5*time.Minute)
}

type Farmer struct {
	contact    msg.Contact
	privateKey crypto.PrivateKey
}

func (f *Farmer) Init(config Config) error {
	f.SetContact(msg.Contact{
		Address:  Cfg.GetLocalAddr(),
		Port:     Cfg.GetLocalPortNum(),
		NodeID:   Cfg.NodeId,
		Protocol: "1.2.0-local",
	})
	pk := crypto.PrivateKey{}
	pk.FromHexStr(Cfg.NodePrivateKey)
	f.SetPrivateKey(pk)
	return nil
}

func (f *Farmer) Contact() msg.Contact {
	return f.contact
}
func (f *Farmer) SetContact(contact msg.Contact) {
	f.contact = contact
}

func (f *Farmer) PrivateKey() crypto.PrivateKey {
	return f.privateKey
}
func (f *Farmer) SetPrivateKey(pk crypto.PrivateKey) {
	f.privateKey = pk
}

func (f *Farmer) Sign(m IMessage) {
	Id := m.GetId()
	if Id == "" {
		Id = hex.EncodeToString(uuid.NewV4().Bytes())
		m.SetId(Id)
	}

	// to be compatible with js version
	nonce := int(time.Now().Unix()) * 1000000000
	m.SetNonce(nonce)

	ms := Id + strconv.Itoa(nonce)
	msHash := MagicHash([]byte(ms))
	pk := f.PrivateKey()
	sig := pk.Sign(msHash[:])
	sigStr := base64.StdEncoding.EncodeToString(sig)
	m.SetSignature(sigStr)
}

func (f *Farmer) SignContract(c *msg.Contract) {
	nodeID := f.Contact().NodeID
	c.FarmerID = &nodeID
	s := c.Stringify()
	hash := MagicHash([]byte(s))
	pk := f.PrivateKey()
	sig := pk.Sign(hash[:])
	sigStr := base64.StdEncoding.EncodeToString(sig)
	c.FarmerSignature = &sigStr
}

func (f *Farmer) ProcessMsgInOut(m *MsgInOut) {
	m.ParseMsgInRaw()
	msgStruct := m.MsgInStruct()

	// switch message
	var idStr string
	var res IMessage
	switch msgStruct.(type) {
	case *msg.Ping:
		res = f.onPing(m)
	case *msg.Probe:
		res = f.onProbe(m)
	case *msg.FindNode:
		res = f.onFindNode(m)
	case *msg.Publish:
		res = f.onPublish(m)
		//case *msg.Offer:
	case *msg.Consign:
		res = f.onConsign(m)
	case *msg.Retrieve:
		res = f.onRetrieve(m)
	case *msg.Mirror:
		res = f.onMirror(m)
	case *msg.Audit:
		res = f.onAudit(m)
	default:
		// print unknown message
		if msgStruct != nil {
			log.Printf("[UNKNOWN] MESSAGE=%v\n", msgStruct)
		} else if len(m.MsgInMap()) != 0 {
			log.Printf("[UNKNOWN] MESSAGE=%v\n", m.MsgInMap())
		} else {
			log.Printf("[UNKNOWN] MESSAGE=%v\n", string(m.MsgInRaw()))
		}
		// get id
		id, okId := m.MsgInMap()["id"]
		_idStr, okString := id.(string)
		if okId && okString {
			idStr = _idStr
		} else {
			idStr = hex.EncodeToString(uuid.NewV4().Bytes())
		}
		goto unknown
	}
	idStr = m.MsgInStruct().GetId()
	goto ret
unknown:
	res = &msg.ResErr{
		Res: msg.Res{
			Result: msg.ResResult{
				Contact: f.Contact(),
			},
		},
		Error: msg.ResErrError{
			Code:    -1,
			Message: "unknown message",
		},
	}
ret:
	res.SetId(idStr) // response id should be same as request id
	f.Sign(res)
	m.SetMsgOutStruct(res)
}

func (f *Farmer) HeartBeat() {
	joinSucc := false
	for _, seed := range Cfg.GetSeedList() {
		err := f.probe(seed)
		if err != nil {
			log.Printf("[HEARTBEAT] try join network PEER=%v ERROR=%v\n", seed.NodeID, err)
			continue
		}
		joinSucc = true
		break
	}
	if joinSucc == false {
		log.Println("[HEARTBEAT] try join network failed")
		os.Exit(-1)
	}
}

///////////////
// do request
///////////////
func (f *Farmer) ping(contact msg.Contact) error {
	log.Printf("[PING] PEER=%v\n", contact.NodeID)
	msgPing := msg.Ping{
		JsonRpc: "2.0",
		Method:  msg.MPing,
		Params: msg.PingParams{
			Contact: f.Contact(),
		},
	}
	f.Sign(&msgPing)
	msgInOut := MsgInOut{}
	msgInOut.SetMsgOutStruct(&msgPing)
	err := SendMsg(contact, &msgInOut, time.Second*4, func() error {
		msgInOut.ParseMsgInRaw()
		return nil
	})
	if err != nil {
		log.Printf("[PING] fail PEER=%v ERROR=%v\n", contact.NodeID, err)
	} else {
		log.Printf("[PING] success PEER=%v\n", contact.NodeID)
	}
	return err
}

func (f *Farmer) probe(contact msg.Contact) error {
	log.Printf("[PROBE] PEER=%v\n", contact.NodeID)
	msgPing := msg.Probe{
		JsonRpc: "2.0",
		Method:  msg.MProbe,
		Params: msg.ProbeParams{
			Contact: f.Contact(),
		},
	}
	f.Sign(&msgPing)
	msgInOut := MsgInOut{}
	msgInOut.SetMsgOutStruct(&msgPing)
	err := SendMsg(contact, &msgInOut, time.Second*8, func() error {
		msgInOut.ParseMsgInRaw()
		return nil
	})
	if err != nil {
		log.Printf("[PROBE] PEER=%v ERROR=%v\n", contact.NodeID, err)
	} else {
		log.Printf("[PROBE] success PEER=%v\n", contact.NodeID)
	}
	return err
}

func (f *Farmer) findNode(contact msg.Contact) {
	msgFindNode := msg.FindNode{
		JsonRpc: "2.0",
		Method:  msg.MFindNode,
		Params: msg.FindNodeParams{
			Key:     f.Contact().NodeID,
			Contact: f.Contact(),
		},
	}
	f.Sign(&msgFindNode)
	msgInOut := MsgInOut{}
	msgInOut.SetMsgOutStruct(&msgFindNode)
	err := SendMsg(contact, &msgInOut, time.Second*4, func() error {
		msgInOut.ParseMsgInRaw()
		return nil
	})
	if err != nil {
		log.Printf("[FIND_NODE] PEER=%v ERROR=%v\n", contact.NodeID, err)
	} else {
		log.Printf("[FIND_NODE] success PEER=%v\n", contact.NodeID)
	}
}

func (f *Farmer) offer(contact msg.Contact, c msg.Contract) {
	msgOffer := msg.Offer{
		JsonRpc: "2.0",
		Method:  msg.MOffer,
		Params: msg.OfferParams{
			Contract: c,
			Contact:  f.Contact(),
		},
	}
	f.Sign(&msgOffer)
	msgInOut := MsgInOut{}
	msgInOut.SetMsgOutStruct(&msgOffer)
	err := SendMsg(contact, &msgInOut, time.Second*4, func() error {
		msgInOut.ParseResInRaw(msg.MOffer)
		msgInStruct := msgInOut.MsgInStruct()
		switch msgInStruct.(type) {
		case *msg.ResErr:
			log.Printf("[OFFER] error RESP=%v\n", msgInStruct)
			msgResErr := msgInStruct.(*msg.ResErr)
			return errors.New(msgResErr.Error.Message)
		case *msg.OfferRes:
			msgOfferRes := msgInStruct.(*msg.OfferRes)
			contract := msgOfferRes.Result.Contract

			// check if audit_count is power of 2
			if contract.AuditCount == 0 || (contract.AuditCount&(contract.AuditCount-1)) != 0 {
				return errors.New("audit_count is not power of 2")
			}
			// save contract in db, wrapped in storageItem
			sItem := storageItem{
				Contract: contract,
			}
			js, _ := json.Marshal(sItem)
			err := BoltDbSet([]byte(contract.DataHash), js, BucketContract, false)

			if err != nil {
				// delete cache so we can process it again
				contractCache.Delete(contract.DataHash)
				return err
			}
			return nil
		default:
			return fmt.Errorf("unknown response %v", string(msgInOut.MsgInRaw()))
		}
	})
	if err != nil {
		log.Printf("[OFFER] send offer error DATA_HASH=%v ERROR=%v\n", c.DataHash, err)
	} else {
		log.Printf("[OFFER] success DATA_HASH=%v\n", c.DataHash)
	}
}

/////////////////////
// on request message
/////////////////////
func (f *Farmer) _generalRes(m *MsgInOut) IMessage {
	c := f.Contact()
	res := msg.Res{
		Result: msg.ResResult{
			Contact: c,
		},
	}
	return &res
}

func (f *Farmer) onPing(m *MsgInOut) IMessage {
	msgPing := m.MsgInStruct().(*msg.Ping)
	log.Printf("[ON PING] PEER=%v\n", msgPing.Params.Contact.NodeID)
	return f._generalRes(m)
}

func (f *Farmer) onProbe(m *MsgInOut) IMessage {
	msgProbe := m.MsgInStruct().(*msg.Probe)
	contact := msgProbe.Params.Contact
	chanProbeSucc := make(chan error)
	go func() {
		err := f.ping(contact)
		chanProbeSucc <- err
	}()
	if err := <-chanProbeSucc; err != nil {
		log.Printf("[ON PROBE] fail PEER=%v ERROR=%v\n", contact.NodeID, err)
		return &msg.ResErr{
			Res: msg.Res{
				Result: msg.ResResult{
					Contact: f.Contact(),
				},
			},
			Error: msg.ResErrError{
				Code:    -1,
				Message: "unknown message",
			},
		}
	}
	log.Printf("[ON PROBE] success PEER=%v\n", contact.NodeID)
	return f._generalRes(m)
}

func (f *Farmer) onPublish(m *MsgInOut) IMessage {
	doOffer := true
	msgPublish := m.MsgInStruct().(*msg.Publish)
	_uuid := msgPublish.Params.Uuid
	_dataHash := msgPublish.Params.Contents.DataHash

	// if contract valid
	contract := msgPublish.Params.Contents
	if contract.IsValid() == false {
		return f._generalRes(m)
	}
	log.Printf("[PUBLISH] DATA_HASH=%v UUID=%v\n", _dataHash, _uuid)

	// if already processed
	// check data_hash
	_, okDataHash := contractCache.Get(_dataHash)
	if okDataHash {
		log.Printf("[PUBLISH] already processed UUID=%v DATA_HASH=%v\n", _uuid, _dataHash)
		doOffer = false
	}

	// TODO: send offer (or not) according to contract
	if doOffer {
		contractCache.Set(_dataHash, nil, cache.DefaultExpiration)
		addr := "0x5d14313c94f1b26d23f4ce3a49a2e136a88a584b"
		contract.PaymentDestination = &addr
		f.SignContract(&contract)
		go func() {
			f.offer(msgPublish.Params.Contact, contract)
		}()
	}
	return f._generalRes(m)
}

func (f *Farmer) onFindNode(m *MsgInOut) IMessage {
	c := f.Contact()
	res := msg.FindNodeRes{
		Result: msg.FindNodeResResult{
			Nodes:   []msg.Contact{},
			Contact: c,
		},
	}

	msgFindNode := m.MsgInStruct().(*msg.FindNode)
	peer := msgFindNode.Params.Contact
	log.Printf("[ON FIND_NODE] PEER=%v\n", peer.NodeID)
	return &res
}

//func offer(node INode, m *MsgInOut) IMessage {
//	offer := m.MsgInStruct().(*msg.Offer)
//	c := node.Contact()
//	res := msg.OfferRes{
//		Result: msg.OfferResResult{
//			Contract: offer.Params.Contract,
//			Contact:  c,
//		},
//	}
//	return &res
//}

func (f *Farmer) onConsign(m *MsgInOut) IMessage {
	// verify trees
	msgConsign := m.MsgInStruct().(*msg.Consign)
	trees := msgConsign.Params.AuditTree
	if len(trees) == 0 {
		return &msg.ResErr{
			Res: msg.Res{
				Result: msg.ResResult{
					Contact: f.Contact(),
				},
			},
			Error: msg.ResErrError{
				Code:    -1,
				Message: "audit_tree is empty",
			},
		}
	}

	// get storageItem
	dataHash := msgConsign.Params.DataHash
	sItemRaw, err := BoltDbGet([]byte(dataHash), BucketContract)
	if err != nil {
		return &msg.ResErr{
			Res: msg.Res{
				Result: msg.ResResult{
					Contact: f.Contact(),
				},
			},
			Error: msg.ResErrError{
				Code:    -1,
				Message: "no contract for data_hash",
			},
		}
	}

	// unmarshal storageItem
	var sItem storageItem
	err = json.Unmarshal(sItemRaw, &sItem)
	if err != nil {
		return &msg.ResErr{
			Res: msg.Res{
				Result: msg.ResResult{
					Contact: f.Contact(),
				},
			},
			Error: msg.ResErrError{
				Code:    -1,
				Message: "internal error",
			},
		}
	}

	// verify trees length
	if sItem.Contract.AuditCount != len(trees) {
		return &msg.ResErr{
			Res: msg.Res{
				Result: msg.ResResult{
					Contact: f.Contact(),
				},
			},
			Error: msg.ResErrError{
				Code:    -1,
				Message: "internal error",
			},
		}
	}

	// generate and save token
	token := hex.EncodeToString(uuid.NewV4().Bytes())
	err = BoltDbSet([]byte(token), []byte{}, BucketToken, false)
	if err != nil {
		log.Printf("[ON CONSIGN] save token error DATA_HASH=%v ERROR=%v\n", dataHash, err)
		return &msg.ResErr{
			Res: msg.Res{
				Result: msg.ResResult{
					Contact: f.Contact(),
				},
			},
			Error: msg.ResErrError{
				Code:    -1,
				Message: "internal error",
			},
		}
	} else {
		log.Printf("[ON CONSIGN] token saved DATA_HASH=%v TOKEN=%v\n", dataHash, token)
	}

	// save trees
	// after token is saved, so that we won't have trees while not supply token
	sItem.Trees = trees
	js, _ := json.Marshal(sItem)
	err = BoltDbSet([]byte(dataHash), js, BucketContract, true)
	if err != nil {
		log.Printf("[ON CONSIGN] save audit trees error DATA_HASH=%v ERROR=%v\n", dataHash, err)
		return &msg.ResErr{Res: msg.Res{
			Result: msg.ResResult{
				Contact: f.Contact(),
			},
		},
			Error: msg.ResErrError{
				Code:    -1,
				Message: "internal error",
			},
		}
	}

	// prepare response
	c := f.Contact()
	res := msg.ConsignRes{
		Result: msg.ConsignResResult{
			Token:   token,
			Contact: c,
		},
	}
	log.Printf("[ON CONSIGN] success DATA_HASH=%v TOKEN=%v\n", dataHash, token)
	return &res
}

func (f *Farmer) onRetrieve(m *MsgInOut) IMessage {
	// TODO: shard exist?
	msgRetrieve := m.MsgInStruct().(*msg.Retrieve)
	dataHash := msgRetrieve.Params.DataHash
	c := f.Contact()
	token := hex.EncodeToString(uuid.NewV4().Bytes())
	res := msg.RetrieveRes{
		Result: msg.RetrieveResResult{
			Token:   token,
			Contact: c,
		},
	}
	err := BoltDbSet([]byte(token), []byte{}, BucketToken, false)
	if err != nil {
		log.Printf("[ON RETRIEVE] save token error DATA_HASH=%v ERROR=%v\n", dataHash, err)
	} else {
		log.Printf("[ON RETRIEVE] saved token DATA_HASH=%v TOKEN=%v\n", dataHash, token)
	}
	return &res
}

func (f *Farmer) onMirror(m *MsgInOut) IMessage {
	msgMirror := m.MsgInStruct().(*msg.Mirror)
	// if already processed
	dataHash := msgMirror.Params.DataHash
	_, okDataHash := mirrorCache.Get(dataHash)
	if okDataHash {
		log.Printf("[ON MIRROR] message already processed DATA_HASH=%v\n", dataHash)
		return f._generalRes(m)
	}
	mirrorCache.Set(dataHash, nil, time.Second*30)

	// if contract exist
	sItemRaw, err := BoltDbGet([]byte(dataHash), BucketContract)
	if err != nil {
		log.Printf("[ON MIRROR] no signed contract DATA_HASH=%v ERROR=%v\n", dataHash, err)
		return msg.NewResErr(f.Contact(), "no signed contract")
	}

	// if trees exists
	var sItem storageItem
	err = json.Unmarshal(sItemRaw, &sItem)
	if err != nil {
		log.Printf("[ON MIRROR] sItem bad format DATA_HASH=%v ERROR=%v\n", dataHash, err)
		return msg.NewResErr(f.Contact(), "sItem bad format")
	}
	trees := msgMirror.Params.AuditTree
	if sItem.Contract.AuditCount != len(trees) {
		log.Printf("[ON MIRROR] audit trees length != audit count DATA_HASH=%v\n", dataHash)
		return msg.NewResErr(f.Contact(), "mirror message bad format")
	}

	// if shard exist
	fPath := path.Join(Cfg.GetShardsPath(), dataHash)
	_, err = os.Stat(fPath)
	if os.IsExist(err) {
		log.Printf("[ON MIRROR] shard already exist DATA_HASH=%v\n", dataHash)
		return f._generalRes(m)
	}

	// TODO: lock contract ?
	log.Printf("[ON MIRROR] DATA_HASH=%v\n", dataHash)
	err = DownloadShard(msgMirror.Params.Farmer, dataHash, msgMirror.Params.Token)
	if err != nil {
		log.Printf("[ON MIRROR] download shard error DATA_HASH=%v ERROR=%v\n", dataHash, err)
		return msg.NewResErr(f.Contact(), "mirror shard failed")
	}

	// save trees
	sItem.Trees = trees
	sItemRaw, _ = json.Marshal(sItem)
	err = BoltDbSet([]byte(dataHash), sItemRaw, BucketContract, true)
	if err != nil {
		// TODO: save trees failed, but have shard downloaded
		log.Printf("[ON MIRROR] save audit trees error DATA_HASH=%v ERROR=%v\n", dataHash, err)
		return msg.NewResErr(f.Contact(), "internal error")
	}
	return f._generalRes(m)
}

func (f *Farmer) onAudit(m *MsgInOut) IMessage {
	// check challenge existence
	msgAudit := m.MsgInStruct().(*msg.Audit)
	if len(msgAudit.Params.Audits) == 0 {
		log.Printf("[ON AUDIT] message bad format ID=%v\n", msgAudit.GetId())
		return msg.NewResErr(f.Contact(), "message bad format")
	}
	audit := msgAudit.Params.Audits[0]
	log.Printf("[ON AUDIT] DATA_HASH=%v\n", audit.DataHash)

	// check shard existence
	fPath := path.Join(Cfg.GetShardsPath(), audit.DataHash)
	_, err := os.Stat(fPath)
	if os.IsNotExist(err) {
		log.Printf("[ON AUDIT] not shard DATA_HASH=%v\n", audit.DataHash)
		return &msg.ResErr{
			Res: msg.Res{
				Result: msg.ResResult{
					Contact: f.Contact(),
				},
			},
			Error: msg.ResErrError{
				Code:    -1,
				Message: "no shard",
			},
		}
	}

	// get trees
	sItemRaw, err := BoltDbGet([]byte(audit.DataHash), BucketContract)
	if err != nil {
		log.Printf("[ON AUDIT] get sItem error DATA_HASH=%v ERROR=%v\n", audit.DataHash, err)
		return &msg.ResErr{
			Res: msg.Res{
				Result: msg.ResResult{
					Contact: f.Contact(),
				},
			},
			Error: msg.ResErrError{
				Code:    -1,
				Message: "internal error",
			},
		}
	}
	var sItem storageItem
	err = json.Unmarshal(sItemRaw, &sItem)
	if err != nil || len(sItem.Trees) == 0 {
		if err != nil {
			log.Printf("[ON AUDIT] parse sItem failed DATA_HASH=%v ERROR=%v\n", audit.DataHash, err)
		} else {
			log.Printf("[ON AUDIT] audit trees length == 0 DATA_HASH=%v\n", audit.DataHash)
		}
		return &msg.ResErr{
			Res: msg.Res{
				Result: msg.ResResult{
					Contact: f.Contact(),
				},
			},
			Error: msg.ResErrError{
				Code:    -1,
				Message: "internal error",
			},
		}
	}

	// open shard for read
	fHandle, err := os.Open(fPath)
	if err != nil {
		log.Printf("[ON AUDIT] open shard error DATA_HASH=%v ERROR=%v\n", audit.DataHash, err)
		return &msg.ResErr{
			Res: msg.Res{
				Result: msg.ResResult{
					Contact: f.Contact(),
				},
			},
			Error: msg.ResErrError{
				Code:    -1,
				Message: "internal error",
			},
		}
	}
	defer fHandle.Close()

	// sha256 of shard
	h := sha256.New()
	chal, err := hex.DecodeString(audit.Challenge)
	if err != nil {
		log.Printf("[ON AUDIT] challenge is not hex string DATA_HASH=%v\n", audit.DataHash)
		return &msg.ResErr{
			Res: msg.Res{
				Result: msg.ResResult{
					Contact: f.Contact(),
				},
			},
			Error: msg.ResErrError{
				Code:    -1,
				Message: "challenge is not hex string",
			},
		}
	}
	h.Write(chal)
	if _, err := io.Copy(h, fHandle); err != nil {
		log.Printf("[ON AUDIT] read shard error DATA_HASH=%v ERROR=%v\n", audit.DataHash, err)
		return &msg.ResErr{
			Res: msg.Res{
				Result: msg.ResResult{
					Contact: f.Contact(),
				},
			},
			Error: msg.ResErrError{
				Code:    -1,
				Message: "internal error",
			},
		}
	}
	h256 := h.Sum(nil)
	hrip := ripemd160.New()
	hrip.Write(h256)
	auditRes := hrip.Sum(nil)
	auditResCmp := crypto.Ripemd160Sha256(auditRes)
	auditResCmpStr := hex.EncodeToString(auditResCmp)

	//
	auditResCmpPos := -1
	for i, v := range sItem.Trees {
		if v == auditResCmpStr {
			auditResCmpPos = i
			break
		}
	}
	if auditResCmpPos == -1 {
		log.Printf("[ON AUDIT] generated tree not found in trees DATA_HASH=%v\n", audit.DataHash)
		return &msg.ResErr{
			Res: msg.Res{
				Result: msg.ResResult{
					Contact: f.Contact(),
				},
			},
			Error: msg.ResErrError{
				Code:    -1,
				Message: "audit failed",
			},
		}
	}
	curLen := len(sItem.Trees)
	proof := make([]interface{}, curLen)
	for i, t := range sItem.Trees {
		if i == auditResCmpPos {
			auditResHex := hex.EncodeToString(auditRes)
			proof[i] = auditResHex
			continue
		}
		proof[i] = t
	}
	pos := auditResCmpPos
	for len(proof) != 2 {
		var _proof []interface{}
		for i := 0; i < len(proof); i += 2 {
			if i == pos {
				_proof = append(_proof, []interface{}{[]interface{}{proof[i]}, proof[i+1]})
			} else if i+1 == pos {
				_proof = append(_proof, []interface{}{proof[i], []interface{}{proof[i+1]}})
			} else {
				m0 := proof[i].(string)
				m1 := proof[i+1].(string)
				m0b, _ := hex.DecodeString(m0)
				m1b, _ := hex.DecodeString(m1)
				mb := append(m0b, m1b...)
				hash := crypto.Ripemd160Sha256(mb)
				hashStr := hex.EncodeToString(hash[:])
				_proof = append(_proof, hashStr)
			}
		}
		proof = _proof
		pos /= 2
	}
	return &msg.AuditRes{
		Result: msg.AuditResResult{
			Proofs:  []interface{}{proof},
			Contact: f.Contact(),
		},
	}
}
