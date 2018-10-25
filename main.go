package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"time"

	"github.com/GenaroNetwork/go-farmer/config"
	"github.com/boltdb/bolt"
)

const MagicBytes = "Bitcoin Signed Message:\n"
const BucketContract = "CONTRACT"
const BucketToken = "TOKEN"

var BoltDB *bolt.DB
var Cfg config.Config

func parseCmdArgs() {
	const helpMsg = `usage: go-farmer <command> [<args>]
Commands:
	start			start a farmer instance
	new-account		generate keyfile for a new farmer account

See "go-farmer help <command>" for information on a specific command.
`
	/* start command */
	startCmd := flag.NewFlagSet("start", flag.ExitOnError)
	// -config
	cPath := startCmd.String("config", "./config.json", "config file path")

	/* new-account command */
	newAccountCmd := flag.NewFlagSet("new-account", flag.ExitOnError)
	// -path
	newAccountCmd.String("path", "./", "keyfile path")

	if len(os.Args) == 1 {
		fmt.Print(helpMsg)
		os.Exit(2)
	}

	switch os.Args[1] {
	case "start":
		_ = startCmd.Parse(os.Args[2:])
		parseConfigFile(cPath)
	case "new-account":
		_ = newAccountCmd.Parse(os.Args[2:])
		os.Exit(0)
	case "help":
		if len(os.Args) != 3 {
			fmt.Print(helpMsg)
			os.Exit(2)
		}
		switch os.Args[2] {
		case "start":
			startCmd.Usage()
		case "new-account":
			newAccountCmd.Usage()
		default:
			fmt.Print(helpMsg)
			os.Exit(2)
		}
		os.Exit(0)
	default:
		fmt.Print(helpMsg)
		os.Exit(2)
	}
}

func parseConfigFile(cPath *string) {
	// check if config file exists
	_, err := os.Stat(*cPath)
	if os.IsNotExist(err) {
		log.Fatalf("[CONFIG] file not exist PATH=%v\n", *cPath)
	}

	// read config file
	cnt, err := ioutil.ReadFile(*cPath)
	if err != nil {
		log.Fatalf("[CONFIG] read file error ERROR=%v\n", err)
	}

	// parse config file
	err = json.Unmarshal(cnt, &Cfg)
	if err != nil {
		log.Fatalf("[CONFIG] decode file error ERROR=%v\n", err)
	}
	if err := Cfg.Parse(); err != nil {
		log.Fatalf("[CONFIG] file malformatted ERROR=%v\n", err)
	}
	js, _ := json.MarshalIndent(Cfg, "", "  ")
	log.Printf("[CONFIG] parsed CONFIG=%v\n", string(js))
}

func main() {
	parseCmdArgs()
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
	if err := node.Init(Cfg); err != nil {
		log.Fatalf("[MAIN] node init failed ERROR=%v\n", err)
	}

	go node.HeartBeat()
	handler := &RegexpHandler{}
	handler.HandleFunc(regexp.MustCompile(`^/$`), RootHandler(node))
	handler.HandleFunc(regexp.MustCompile(`^/shards/\w+$`), ShardHandler())

	server := &http.Server{
		Addr:        ":" + Cfg.GetLocalPortStr(),
		Handler:     handler,
		IdleTimeout: 1 * time.Second,
	}
	err = server.ListenAndServe()
	log.Printf("[HTTP] listen error ERROR=%v\n", err)
}
