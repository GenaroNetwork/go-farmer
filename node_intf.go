package main

import (
	"github.com/GenaroNetwork/go-farmer/config"
	"github.com/GenaroNetwork/go-farmer/crypto"
	"github.com/GenaroNetwork/go-farmer/msg"
)

type INode interface {
	Init(config.Config) error

	Contact() msg.Contact
	SetContact(msg.Contact)

	PrivateKey() crypto.PrivateKey
	SetPrivateKey(crypto.PrivateKey)

	Sign(IMessage)
	ProcessMsgInOut(*MsgInOut)

	HeartBeat()
}
