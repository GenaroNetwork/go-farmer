package main

import (
	"gofarmer/crypto"
	"gofarmer/msg"
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
