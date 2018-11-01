package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/GenaroNetwork/go-farmer/config"
	"github.com/boltdb/bolt"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	ui "github.com/gizak/termui"
	"golang.org/x/crypto/ssh/terminal"
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
	sConfigPath := startCmd.String("config", "./config.json", "config file path")

	/* new-account command */
	newAccountCmd := flag.NewFlagSet("new-account", flag.ExitOnError)
	// -path
	sKeyfilePath := newAccountCmd.String("path", "./", "directory where keyfile will be generated")

	if len(os.Args) == 1 {
		fmt.Print(helpMsg)
		os.Exit(2)
	}

	switch os.Args[1] {
	case "start":
		_ = startCmd.Parse(os.Args[2:])
		parseConfigFile(sConfigPath)
	case "new-account":
		_ = newAccountCmd.Parse(os.Args[2:])
		doCreateKeyfile(sKeyfilePath)
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

func doCreateKeyfile(cKeyfilePath *string) {
	// read password
	fmt.Print("Please enter password: ")
	pass0, err := terminal.ReadPassword(int(syscall.Stdin))
	if err != nil {
		fmt.Printf("Read password error: %v\n", err)
		os.Exit(2)
	}
	fmt.Println()
	sPass0 := strings.TrimSpace(string(pass0))
	if len(pass0) == 0 {
		fmt.Println("Empty password disallowed.")
		os.Exit(2)
	}

	// read confirm password
	fmt.Print("Please enter confirm password: ")
	pass1, err := terminal.ReadPassword(int(syscall.Stdin))
	if err != nil {
		fmt.Printf("Read password error: %v\n", err)
		os.Exit(2)
	}
	fmt.Println()

	// check confirm password
	if bytes.Equal(pass0, pass1) == false {
		fmt.Println("Your password and confirmation password do not match.")
		os.Exit(2)
	}

	// generate keyfile
	address, err := keystore.StoreKey(*cKeyfilePath, sPass0, keystore.StandardScryptN, keystore.StandardScryptP)
	if err != nil {
		fmt.Printf("Generate key failed: %v\n", err)
		os.Exit(2)
	}
	fmt.Printf("Keyfile generated.\nAddress: %v\n", address.String())
	os.Exit(0)
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

	handler := &RegexpHandler{}
	handler.HandleFunc(regexp.MustCompile(`^/$`), RootHandler(node))
	handler.HandleFunc(regexp.MustCompile(`^/shards/\w+$`), ShardHandler())

	server := &http.Server{
		Addr:        ":" + Cfg.GetLocalPortStr(),
		Handler:     handler,
		IdleTimeout: 1 * time.Second,
	}

	// start server
	stopServer := make(chan struct{}, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[HTTP] listen error ERROR=%v\n", err)
			stopServer <- struct{}{}
		}
	}()

	// start terminal ui
	stopUi := make(chan struct{}, 1)
	go func() {
		err := UiSetup()
		if err != nil {
			log.Printf("[TERMINAL] init failed ERROR=%v\n", err)
		}
		stopUi <- struct{}{}
	}()

	// heartbeat
	go node.HeartBeat()

	// wait
	select {
	case <-stopServer:
		ui.StopLoop()
	case <-stopUi:
	}

	// shutdown server gracefully
	log.Println("\nShutting down the server...")
	ctx, _ := context.WithTimeout(context.Background(), time.Minute)
	_ = server.Shutdown(ctx)
}
