package main

import (
	"gofarmer/msg"
	"gofarmer/crypto"
	"github.com/satori/go.uuid"
	"encoding/hex"
	"time"
	"strconv"
	"encoding/base64"
	"fmt"
	"os"
	"github.com/patrickmn/go-cache"
	"errors"
	"encoding/json"
	"path"
	"crypto/sha256"
	"io"
	"golang.org/x/crypto/ripemd160"
)

var contractCache *cache.Cache

type storageItem struct {
	Contract msg.Contract `json:"contract"`
	//Shard bool `json:"shard"`
	Trees []string `json:"trees"`
}

func init() {
	contractCache = cache.New(2*time.Minute, 5*time.Minute)
	//publishCache = cache.New(2*time.Minute, 5*time.Minute)
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

	// compatible with js version
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
		// print in message
		if msgStruct != nil {
			fmt.Println(msgStruct)
		} else if len(m.MsgInMap()) != 0 {
			fmt.Println(m.MsgInMap())
		} else {
			fmt.Println(string(m.MsgInRaw()))
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
	//js, _ := json.MarshalIndent(res, "", "  ")
	//fmt.Println(string(js))
	m.SetMsgOutStruct(res)
}

func (f *Farmer) HeartBeat() {
	joinSucc := false
	for _, seed := range Cfg.GetSeedList() {
		err := f.probe(seed)
		if err != nil {
			fmt.Printf("try join network via %v failed: %v\n", seed.NodeID, err)
			continue
		}
		joinSucc = true
		break
	}
	if joinSucc == false {
		fmt.Println("try join network failed")
		os.Exit(-1)
	}
}

///////////////
// do request
///////////////
func (f *Farmer) ping(contact msg.Contact) error {
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
		fmt.Printf("send %v error: %v\n", msg.MPing, err)
	} else {
		fmt.Printf("send %v success\n", msg.MPing)
	}
	return err
}

func (f *Farmer) probe(contact msg.Contact) error {
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
		fmt.Printf("send %v error: %v\n", msg.MProbe, err)
	} else {
		fmt.Printf("send %v success\n", msg.MProbe)
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
		fmt.Printf("send %v error: %v\n", msg.MFindNode, err)
	} else {
		fmt.Printf("send %v success\n", msg.MFindNode)
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
			fmt.Printf("%v\n", string(msgInOut.MsgInRaw()))
			return errors.New(fmt.Sprintf("unknown response %T", msgInStruct))
		}
	})
	if err != nil {
		fmt.Printf("send %v error: %v\n", msg.MOffer, err)
	} else {
		fmt.Printf("send %v success\n", msg.MOffer)
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
	return f._generalRes(m)
}

func (f *Farmer) onProbe(m *MsgInOut) IMessage {
	msgProbe := m.MsgInStruct().(*msg.Probe)
	chanProbeSucc := make(chan bool)
	go func() {
		err := f.ping(msgProbe.Params.Contact)
		if err != nil {
			chanProbeSucc <- true
		} else {
			chanProbeSucc <- false
		}
	}()
	if <-chanProbeSucc {
		return f._generalRes(m)
	}
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

func (f *Farmer) onPublish(m *MsgInOut) IMessage {
	doOffer := true
	msgPublish := m.MsgInStruct().(*msg.Publish)
	_uuid := msgPublish.Params.Uuid
	_dataHash := msgPublish.Params.Contents.DataHash
	fmt.Printf("received msg: %v %v\n", msg.MPublish, _uuid)

	// if already processed
	// check data_hash
	_, okDataHash := contractCache.Get(_dataHash)
	if okDataHash {
		fmt.Printf("%v %v already processed\n", msg.MPublish, _uuid)
		doOffer = false
	}

	// TODO: send offer (or not) according to contract
	if doOffer {
		contractCache.Set(_dataHash, "", cache.DefaultExpiration)
		//publishCache.Set(_uuid, "", cache.DefaultExpiration)
		contract := msgPublish.Params.Contents
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
		fmt.Printf("save consign token err: %v\n", err)
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
		fmt.Printf("save consign token %v\n", token)
	}

	// save trees
	// after token is saved, so that we won't have trees while not supply token
	sItem.Trees = trees
	js, _ := json.Marshal(sItem)
	err = BoltDbSet([]byte(dataHash), js, BucketContract, true)
	if err != nil {
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
	return &res
}

func (f *Farmer) onRetrieve(m *MsgInOut) IMessage {
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
		fmt.Printf("save retrieve token err: %v\n", err)
	} else {
		fmt.Printf("save retrieve token %v\n", token)
	}
	return &res
}

func (f *Farmer) onMirror(m *MsgInOut) IMessage {
	// if contract exist
	// if shard not exist
	// lock contract ?
	msgMirror := m.MsgInStruct().(*msg.Mirror)
	err := DownloadShard(msgMirror.Params.Contact, msgMirror.Params.DataHash, msgMirror.Params.Token)
	if err != nil {
		fmt.Printf("mirror error: %v\n", err)
		return &msg.ResErr{
			Res: msg.Res{
				Result: msg.ResResult{
					Contact: f.Contact(),
				},
			},
			Error: msg.ResErrError{
				Code:    -1,
				Message: "mirror shard failed",
			},
		}
	}
	return f._generalRes(m)
}

func (f *Farmer) onAudit(m *MsgInOut) IMessage {
	// check challenge existence
	msgAudit := m.MsgInStruct().(*msg.Audit)
	fmt.Printf("received msg AUDIT, hash: %v, id: %v\n", msgAudit.Params.Audits[0].DataHash, msgAudit.GetId())
	if len(msgAudit.Params.Audits) == 0 {
		fmt.Printf("audit no challenge, msg id: %v", msgAudit.GetId())
		return &msg.ResErr{
			Res: msg.Res{
				Result: msg.ResResult{
					Contact: f.Contact(),
				},
			},
			Error: msg.ResErrError{
				Code:    -1,
				Message: "audit no challenge",
			},
		}
	}
	audit := msgAudit.Params.Audits[0]

	// check shard existence
	fPath := path.Join(Cfg.GetShardsPath(), audit.DataHash)
	_, err := os.Stat(fPath)
	if os.IsNotExist(err) {
		fmt.Printf("audit shard not exist: %v\n", audit.DataHash)
		return &msg.ResErr{
			Res: msg.Res{
				Result: msg.ResResult{
					Contact: f.Contact(),
				},
			},
			Error: msg.ResErrError{
				Code:    -1,
				Message: "audit shard not exist",
			},
		}
	}

	// get trees
	sItemRaw, err := BoltDbGet([]byte(audit.DataHash), BucketContract)
	if err != nil {
		fmt.Printf("get sItem error for shard: %v\n", audit.DataHash)
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

	// sha256 of shard
	h := sha256.New()
	chal, err := hex.DecodeString(audit.Challenge)
	if err != nil {
		return &msg.ResErr{
			Res: msg.Res{
				Result: msg.ResResult{
					Contact: f.Contact(),
				},
			},
			Error: msg.ResErrError{
				Code:    -1,
				Message: "challenge not hex string",
			},
		}
	}
	h.Write(chal)
	if _, err := io.Copy(h, fHandle); err != nil {
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
	//hMsg = crypto.Ripemd160Sha256(hMsg)
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
