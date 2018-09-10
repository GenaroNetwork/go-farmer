package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"

	"github.com/boltdb/bolt"
)

const MagicBytes = "Bitcoin Signed Message:\n"
const BucketContract = "CONTRACT"
const BucketToken = "TOKEN"

var BoltDB *bolt.DB
var Cfg Config

func init() {
	// parse config
	cPath := flag.String("config", "./config.json", "config file path")
	flag.Parse()
	_, err := os.Stat(*cPath)
	if os.IsNotExist(err) {
		log.Fatalf("[CONFIG] file not exist PATH=%v\n", *cPath)
	}
	cnt, err := ioutil.ReadFile(*cPath)
	if err != nil {
		log.Fatalf("[CONFIG] read file error ERROR=%v\n", err)
	}
	err = json.Unmarshal(cnt, &Cfg)
	if err != nil {
		log.Fatalf("[CONFIG] decode file error ERROR=%v\n", err)
	}
	if err = Cfg.Parse(); err != nil {
		log.Fatalf("[CONFIG] file malformatted ERROR=%v\n", err)
	}
	js, _ := json.MarshalIndent(Cfg, "", "  ")
	log.Printf("[CONFIG] parsed CONFIG=%v\n", string(js))
}

func main() {
	// prepare boltdb
	boltDB, err := bolt.Open(Cfg.GetContractDBPath(), 0600, nil)
	BoltDB = boltDB
	if err != nil {
		log.Fatal("[BOLTDB] cannot open boltdb")
	}
	defer BoltDB.Close()
	err = BoltDB.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(BucketContract))
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists([]byte(BucketToken))
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		log.Printf("[BOLTDB] create boltdb bucket error ERROR=%v\n", err)
		return
	}

	var node INode
	node = &Farmer{}
	node.Init(Cfg)

	go node.HeartBeat()
	handler := &RegexpHandler{}
	handler.HandleFunc(regexp.MustCompile(`^/$`), RootHandler(node))
	handler.HandleFunc(regexp.MustCompile(`^/shards/\w+$`), ShardHandler())

	err = http.ListenAndServe(":"+Cfg.GetLocalPortStr(), handler)
	log.Printf("[HTTP] listen error ERROR=%v\n", err)
}
