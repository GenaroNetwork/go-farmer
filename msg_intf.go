package main

// IMessage message
type IMessage interface {
	IsValid() bool
	GetId() string
	SetId(string)
	SetNonce(int)
	SetSignature(string)
}
