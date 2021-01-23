package main

import (
	"github.com/haveachin/infrared/mc"
	"log"
)

type Modifier interface {
	Modify(src, dst mc.Conn, data *[]byte)
}

type LoggerModifier struct {
	Prefix string
}

func (modifier LoggerModifier) Modify(src, dst mc.Conn, data *[]byte) {
	log.Println(modifier.Prefix, data)
}
