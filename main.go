package main

import (
	"flag"
	"os"
	"fmt"
	"io/ioutil"
	"encoding/json"
	"regexp"
	"log"
	"net/http"
	"github.com/boltdb/bolt"
)

const MagicBytes = "Bitcoin Signed Message:\n"
const BucketContract = "CONTRACT"
const BucketToken = "TOKEN"

var BoltDB *bolt.DB
var Cfg Config

func init() {
	cPath := flag.String("config", "./config.json", "config file path")
	flag.Parse()
	_, err := os.Stat(*cPath)
	if os.IsNotExist(err) {
		fmt.Printf("file not exists: %v\n", *cPath)
		os.Exit(1)
	}
	cnt, err := ioutil.ReadFile(*cPath)
	if err != nil {
		fmt.Printf("read config file err: %v\n", err)
		os.Exit(2)
	}
	err = json.Unmarshal(cnt, &Cfg)
	if err != nil {
		fmt.Printf("decode config file err: %v\n", err)
		os.Exit(3)
	}
	if err = Cfg.Parse(); err != nil {
		fmt.Printf("config file malformatted: %v\n", err)
		os.Exit(4)
	}
	js, _ := json.MarshalIndent(Cfg, "", "  ")
	fmt.Printf("Cfg: %v\n", string(js))
}

func main() {
	boltDB, err := bolt.Open(Cfg.GetContractDBPath(), 0600, nil)
	BoltDB = boltDB
	if err != nil {
		panic("cannot open boltdb")
	}
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
		fmt.Printf("create boltdb bucket err: %v\n", err)
		return
	}
	defer BoltDB.Close()

	var node INode
	node = &Farmer{}
	node.Init(Cfg)

	go node.HeartBeat()
	handler := &RegexpHandler{}
	handler.HandleFunc(regexp.MustCompile(`^/$`), RootHandler(node))
	handler.HandleFunc(regexp.MustCompile(`^/shards/\w+$`), ShardHandler())

	log.Fatal(http.ListenAndServe(":"+Cfg.GetLocalPortStr(), handler))
}
