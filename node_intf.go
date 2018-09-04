package main

import (
	"github.com/GenaroNetwork/go-farmer/msg"
	"github.com/GenaroNetwork/go-farmer/crypto"
)

type INode interface {
	Init(Config) error

	Contact() msg.Contact
	SetContact(msg.Contact)

	PrivateKey() crypto.PrivateKey
	SetPrivateKey(crypto.PrivateKey)

	Sign(IMessage)
	ProcessMsgInOut(*MsgInOut)

	HeartBeat()
}
