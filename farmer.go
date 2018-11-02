package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"syscall"
	"time"

	"github.com/GenaroNetwork/go-farmer/config"
	"github.com/GenaroNetwork/go-farmer/crypto"
	"github.com/GenaroNetwork/go-farmer/msg"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/p2p/nat"
	log "github.com/inconshreveable/log15"
	"github.com/patrickmn/go-cache"
	"github.com/satori/go.uuid"
	"golang.org/x/crypto/ripemd160"
	"golang.org/x/crypto/ssh/terminal"
)

var contractCache *cache.Cache
var mirrorCache *cache.Cache
var logger log.Logger

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
	contact msg.Contact
	pk      crypto.PrivateKey
}

func (f *Farmer) Init(config config.Config) error {
	if err := f.doLoadKeyfile(config.KeyFile); err != nil {
		return err
	}
	nodeId := f.pk.NodeId()
	f.SetContact(msg.Contact{
		Address:  Cfg.GetLocalAddr(),
		Port:     Cfg.GetLocalPort(),
		NodeID:   nodeId,
		Protocol: "1.2.0-local",
	})

	logger = log.New("module", "farmer")
	return nil
}
func (f *Farmer) doLoadKeyfile(path string) error {
	rawKeyfile, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	const tryN = 3
	var rawPass []byte
	var key *keystore.Key
	for i := 0; i < tryN; i += 1 {
		fmt.Print("Please input password: ")
		rawPass, err = terminal.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			continue
		}
		pass := string(rawPass)
		key, err = keystore.DecryptKey(rawKeyfile, pass)
		if err != nil {
			continue
		}
		err = f.pk.SetKey(key.PrivateKey)
		if err != nil {
			continue
		}
		return nil
	}
	return err
}

func (f *Farmer) Contact() msg.Contact {
	return f.contact
}
func (f *Farmer) SetContact(contact msg.Contact) {
	f.contact = contact
}

func (f *Farmer) PrivateKey() crypto.PrivateKey {
	return f.pk
}

func (f *Farmer) SetPrivateKey(pk crypto.PrivateKey) {
	f.pk = pk
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
			logger.Warn("unknown message", "subject", "message", "message", JsonMarshal(msgStruct))
		} else if len(m.MsgInMap()) != 0 {
			logger.Warn("unknown message", "subject", "message", "message", JsonMarshal(m.MsgInMap()))
		} else {
			logger.Warn("unknown message", "subject", "message", "message", string(m.MsgInRaw()))
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
	// try join network
	joinSucc := f.doJoinNetwork()

	if joinSucc == false {
		// port-forwarding
		logger.Warn("try upnp/pmp port-forwarding", "subject", "heartbeat")
		natm := nat.Any()
		ip, err := natm.ExternalIP()
		if err != nil {
			logger.Warn("get external ip failed", "subject", "heartbeat")
		}
		f.contact.Address = ip.String()
		go f._map(natm, nil, "TCP", int(Cfg.GetLocalPort()), int(Cfg.GetLocalPort()), "Genaro Sharer")

		// try join network
		joinSucc := f.doJoinNetwork()
		if joinSucc == false {
			logger.Warn("join network by port-forwarding failed", "subject", "heartbeat")
		}
	}
	logger.Info("join network success", "subject", "heartbeat")

	// probe periodically
	failCount := 0
	for {
		time.Sleep(time.Second * 10)
		if joinSucc := f.doJoinNetwork(); joinSucc == false {
			failCount += 1
		} else {
			failCount = 0
		}
		if failCount >= 10 {
			logger.Crit("disconnected from network", "subject", "heartbeat")
			return
		}
	}
}

func (f *Farmer) doJoinNetwork() (joinSucc bool) {
	joinSucc = false
	for _, seed := range Cfg.GetSeedList() {
		err := f.probe(seed)
		if err != nil {
			logger.Info("try join network", "subject", "network", "peer", seed.NodeID, "error", err)
			continue
		}
		joinSucc = true
		break
	}
	if joinSucc == false {
		logger.Warn("try join network failed", "subject", "network")
	}
	return
}

func (f *Farmer) _map(m nat.Interface, stop chan struct{}, protocol string, extport, intport int, name string) {
	logger := logger.New("subject", "network")
	const mapUpdateInterval = 15 * time.Minute
	const mapTimeout = 20 * time.Minute
	refresh := time.NewTimer(mapUpdateInterval)
	defer func() {
		refresh.Stop()
		logger.Info("Deleting port mapping")
		_ = m.DeleteMapping(protocol, extport, intport)
	}()
	if err := m.AddMapping(protocol, extport, intport, name, mapTimeout); err != nil {
		logger.Info("Couldn't add port mapping", "error", err)
		return
	}
	log.Info("Mapped network port", )
	for {
		select {
		case _, ok := <-stop:
			if !ok {
				return
			}
		case <-refresh.C:
			logger.Info("Refreshing port mapping")
			if err := m.AddMapping(protocol, extport, intport, name, mapTimeout); err != nil {
				logger.Info("Couldn't add port mapping", "error", err)
			}
			refresh.Reset(mapUpdateInterval)
		}
	}
}

///////////////
// do request
///////////////
func (f *Farmer) ping(contact msg.Contact) error {
	logger := logger.New("subject", "ping")
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
		logger.Warn("ping failed", "peer", contact.NodeID, "error", err)
	} else {
		logger.Info("ping success", "peer", contact.NodeID)
	}
	return err
}

func (f *Farmer) probe(contact msg.Contact) error {
	logger := logger.New("subject", "probe")
	msgProbe := msg.Probe{
		JsonRpc: "2.0",
		Method:  msg.MProbe,
		Params: msg.ProbeParams{
			Contact: f.Contact(),
		},
	}
	f.Sign(&msgProbe)
	msgInOut := MsgInOut{}
	msgInOut.SetMsgOutStruct(&msgProbe)
	err := SendMsg(contact, &msgInOut, time.Second*8, func() error {
		msgInOut.ParseMsgInRaw()
		return nil
	})
	if err != nil {
		logger.Warn("probe failed", "peer", contact.NodeID, "error", err)
	} else {
		logger.Info("probe success", "peer", contact.NodeID)
	}
	return err
}

func (f *Farmer) findNode(contact msg.Contact) {
	logger := logger.New("subject", "find_node")
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
		logger.Warn("find_node failed", "peer", contact.NodeID, "error", err)
	} else {
		logger.Info("find_node success", "peer", contact.NodeID)
	}
}

func (f *Farmer) offer(contact msg.Contact, c msg.Contract) {
	logger := logger.New("subject", "offer")
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
			logger.Warn("offer error", "message", JsonMarshal(msgInStruct))
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
		logger.Warn("offer error", "data_hash", c.DataHash, "error", err)
	} else {
		logger.Info("offer success", "data_hash", c.DataHash)
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
	contact := msgPing.Params.Contact
	logger.Info("on ping", "subject", "on ping", "peer", contact.NodeID, "ip", contact.Address)
	return f._generalRes(m)
}

func (f *Farmer) onProbe(m *MsgInOut) IMessage {
	logger := logger.New("subject", "on probe")

	msgProbe := m.MsgInStruct().(*msg.Probe)
	contact := msgProbe.Params.Contact
	chanProbeSucc := make(chan error)
	go func() {
		err := f.ping(contact)
		chanProbeSucc <- err
	}()
	if err := <-chanProbeSucc; err != nil {
		logger.Warn("on probe failed", "peer", contact.NodeID, "error", err)
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
	logger.Info("on probe success", "peer", contact.NodeID)
	return f._generalRes(m)
}

func (f *Farmer) onPublish(m *MsgInOut) IMessage {
	logger := logger.New("subject", "on publish")

	doOffer := true
	msgPublish := m.MsgInStruct().(*msg.Publish)
	_uuid := msgPublish.Params.Uuid
	_dataHash := msgPublish.Params.Contents.DataHash

	// if contract valid
	contract := msgPublish.Params.Contents
	if contract.IsValid() == false {
		return f._generalRes(m)
	}
	logger.Info("", "uuid", _uuid, "data_hash", _dataHash)

	// if already processed
	// check data_hash
	_, okDataHash := contractCache.Get(_dataHash)
	if okDataHash {
		logger.Info("already processed", "uuid", _uuid, "data_hash", _dataHash)
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
	logger := logger.New("subject", "on find_node")

	c := f.Contact()
	res := msg.FindNodeRes{
		Result: msg.FindNodeResResult{
			Nodes:   []msg.Contact{},
			Contact: c,
		},
	}

	msgFindNode := m.MsgInStruct().(*msg.FindNode)
	peer := msgFindNode.Params.Contact
	logger.Info("on find_node", "peer", peer.NodeID)
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
	logger := logger.New("subject", "on consign")

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
		logger.Warn("save token error", "data_hash", dataHash, "error", err)
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
		logger.Info("token saved", "data_hash", dataHash, "token", token)
	}

	// save trees
	// after token is saved, so that we won't have trees while not supply token
	sItem.Trees = trees
	js, _ := json.Marshal(sItem)
	err = BoltDbSet([]byte(dataHash), js, BucketContract, true)
	if err != nil {
		logger.Warn("save audit trees error", "data_hash", dataHash, "error", err)
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
	logger.Info("success", "data_hash", dataHash, "token", token)
	return &res
}

func (f *Farmer) onRetrieve(m *MsgInOut) IMessage {
	logger := logger.New("subject", "on retrieve")

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
		logger.Warn("save token error", "data_hash", dataHash, "error", err)
	} else {
		logger.Info("saved token", "data_hash", dataHash, "token", token)
	}
	return &res
}

func (f *Farmer) onMirror(m *MsgInOut) IMessage {
	logger := logger.New("subject", "on mirror")

	msgMirror := m.MsgInStruct().(*msg.Mirror)
	// if already processed
	dataHash := msgMirror.Params.DataHash
	_, okDataHash := mirrorCache.Get(dataHash)
	if okDataHash {
		logger.Warn("message already processed", "data_hash", dataHash)
		return f._generalRes(m)
	}
	mirrorCache.Set(dataHash, nil, time.Second*30)

	// if contract exist
	sItemRaw, err := BoltDbGet([]byte(dataHash), BucketContract)
	if err != nil {
		logger.Info("no signed contract", "data_hash", dataHash, "error", err)
		return msg.NewResErr(f.Contact(), "no signed contract")
	}

	// if trees exists
	var sItem storageItem
	err = json.Unmarshal(sItemRaw, &sItem)
	if err != nil {
		logger.Info("sItem bad format", "data_hash", dataHash, "error", err)
		return msg.NewResErr(f.Contact(), "sItem bad format")
	}
	trees := msgMirror.Params.AuditTree
	if sItem.Contract.AuditCount != len(trees) {
		logger.Info("audit trees length != audit count", "data_hash", dataHash)
		return msg.NewResErr(f.Contact(), "mirror message bad format")
	}

	// if shard exist
	fPath := path.Join(Cfg.GetShardsPath(), dataHash)
	_, err = os.Stat(fPath)
	if os.IsExist(err) {
		logger.Warn("shard already exist", "data_hash", dataHash)
		return f._generalRes(m)
	}

	// TODO: lock contract ?
	logger.Info("mirroring shard", "data_hash", dataHash)
	err = DownloadShard(msgMirror.Params.Farmer, dataHash, msgMirror.Params.Token)
	if err != nil {
		logger.Warn("download shard error", "data_hash", dataHash, "error", err)
		return msg.NewResErr(f.Contact(), "mirror shard failed")
	}

	// save trees
	sItem.Trees = trees
	sItemRaw, _ = json.Marshal(sItem)
	err = BoltDbSet([]byte(dataHash), sItemRaw, BucketContract, true)
	if err != nil {
		// TODO: save trees failed, but have shard downloaded
		logger.Warn("save audit trees error", "data_hash", dataHash, "error", err)
		return msg.NewResErr(f.Contact(), "internal error")
	}
	return f._generalRes(m)
}

func (f *Farmer) onAudit(m *MsgInOut) IMessage {
	logger := logger.New("subject", "on audit")

	// check challenge existence
	msgAudit := m.MsgInStruct().(*msg.Audit)
	if len(msgAudit.Params.Audits) == 0 {
		logger.Info("message bad format", "message", JsonMarshal(msgAudit))
		return msg.NewResErr(f.Contact(), "message bad format")
	}
	audit := msgAudit.Params.Audits[0]
	logger.Info("on audit", "data_hash", audit.DataHash)

	// check shard existence
	fPath := path.Join(Cfg.GetShardsPath(), audit.DataHash)
	_, err := os.Stat(fPath)
	if os.IsNotExist(err) {
		logger.Warn("no shard", "data_hash", audit.DataHash)
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
		logger.Warn("get sItem error", "data_hash", audit.DataHash, "error", err)
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
			logger.Warn("parse sItem failed", "data_hash", audit.DataHash, "error", err)
		} else {
			logger.Warn("audit trees length == 0", "data_hash", audit.DataHash)
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
		logger.Warn("open shard error", "data_hash", audit.DataHash, "error", err)
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
		logger.Info("challenge is not hex string", "data_hash", audit.DataHash)
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
		logger.Warn("read shard error", "data_hash", audit.DataHash, "error", err)
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
		logger.Warn("generated tree not found in trees", "data_hash", audit.DataHash)
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
