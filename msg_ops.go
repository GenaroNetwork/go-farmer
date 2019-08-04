package main

import (
	"encoding/json"

	"github.com/GenaroNetwork/go-farmer/msg"
	log "github.com/inconshreveable/log15"
	"github.com/mitchellh/mapstructure"
)

// MsgInOut fill in received msg,
// and then response msg to be sent.
// or vice versa
type MsgInOut struct {
	msgInRaw     []byte
	msgInMap     map[string]interface{}
	msgInStruct  IMessage
	msgOutStruct IMessage
	// determined by whether SetMsgInRaw or SetMsgOutStruct called first.
	isIn *bool
}

func (mio *MsgInOut) MsgInRaw() []byte {
	return mio.msgInRaw
}
func (mio *MsgInOut) SetMsgInRaw(m []byte) {
	mio.msgInRaw = m
	if mio.isIn == nil {
		_isIn := true
		mio.isIn = &_isIn
	}
}
func (mio *MsgInOut) ParseMsgInRaw() {
	err := json.Unmarshal(mio.msgInRaw, &mio.msgInMap)
	if err != nil {
		return
	}

	// request message
	vMethod, okMethod := mio.msgInMap["method"]
	if okMethod == false {
		return
	}
	vMethod, okString := vMethod.(string)
	if okString == false {
		return
	}

	switch vMethod {
	case msg.MPing:
		mio.msgInStruct = &msg.Ping{}
	case msg.MProbe:
		mio.msgInStruct = &msg.Probe{}
	case msg.MFindNode:
		mio.msgInStruct = &msg.FindNode{}
	case msg.MPublish:
		mio.msgInStruct = &msg.Publish{}
	case msg.MOffer:
		mio.msgInStruct = &msg.Offer{}
	case msg.MConsign:
		mio.msgInStruct = &msg.Consign{}
	case msg.MRetrieve:
		mio.msgInStruct = &msg.Retrieve{}
	case msg.MMirror:
		mio.msgInStruct = &msg.Mirror{}
	case msg.MAudit:
		mio.msgInStruct = &msg.Audit{}
	default:
		log.Warn("unknown message", "message", vMethod)
		return
	}
	mapstructure.Decode(mio.msgInMap, mio.msgInStruct)
}

// parse response
func (mio *MsgInOut) ParseResInRaw(t string) {
	err := json.Unmarshal(mio.msgInRaw, &mio.msgInMap)
	if err != nil {
		return
	}

	// error response
	_, okError := mio.msgInMap["error"]
	if okError {
		mio.msgInStruct = &msg.ResErr{}
		mapstructure.Decode(mio.msgInMap, mio.msgInStruct)
		return
	}

	// other response
	switch t {
	case msg.MPing:
		mio.msgInStruct = &msg.Res{}
	case msg.MProbe:
		mio.msgInStruct = &msg.Res{}
	case msg.MFindNode:
		mio.msgInStruct = &msg.FindNodeRes{}
	case msg.MPublish:
		mio.msgInStruct = &msg.Res{}
	case msg.MOffer:
		mio.msgInStruct = &msg.OfferRes{}
		//case msg.MConsign:
		//	mio.msgInStruct = &msg.Consign{}
		//case msg.MRetrieve:
		//	mio.msgInStruct = &msg.Retrieve{}
	}
	mapstructure.Decode(mio.msgInMap, mio.msgInStruct)
}

func (mio *MsgInOut) MsgInMap() map[string]interface{} {
	return mio.msgInMap
}
func (mio *MsgInOut) MsgInStruct() IMessage {
	return mio.msgInStruct
}

func (mio *MsgInOut) MsgOutStruct() IMessage {
	return mio.msgOutStruct
}
func (mio *MsgInOut) SetMsgOutStruct(msg IMessage) {
	mio.msgOutStruct = msg
	if mio.isIn == nil {
		_isIn := false
		mio.isIn = &_isIn
	}
}

func (mio *MsgInOut) IsIn() *bool {
	return mio.isIn
}

// return marshaled IMessage string returned by MsgOutStruct
func (mio *MsgInOut) String() string {
	js, _ := json.Marshal(mio.msgOutStruct)
	return string(js)
}
