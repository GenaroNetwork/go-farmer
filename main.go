package main

import (
	"context"
	"net/http"
	"path"
	"regexp"
	"time"

	"github.com/GenaroNetwork/go-farmer/config"
	"github.com/boltdb/bolt"
	ui "github.com/gizak/termui"
	log "github.com/inconshreveable/log15"
)

const MagicBytes = "Bitcoin Signed Message:\n"
const BucketContract = "CONTRACT"
const BucketToken = "TOKEN"

var BoltDB *bolt.DB
var Cfg config.Config
var ChanSize = make(chan int64, 10)

func main() {
	ParseCmdArgs()

	// setup logger
	logFile := path.Join(Cfg.LogDir, "go-farmer.log")
	logHandler, err := log.FileHandler(logFile, log.LogfmtFormat())
	if err != nil {
		log.Crit("setup logger failed", "ERROR", err)
		return
	}
	log.Root().SetHandler(logHandler)

	// prepare boltdb
	boltDB, err := bolt.Open(Cfg.GetContractDBPath(), 0600, nil)
	BoltDB = boltDB
	if err != nil {
		log.Crit("cannot open boltdb", "subject", "boltdb")
		return
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
		log.Crit("create boltdb bucket error", "subject", "boltdb", "ERROR", err)
		return
	}

	var node INode
	node = &Farmer{}
	if err := node.Init(Cfg); err != nil {
		log.Crit("node init failed", "ERROR", err)
		return
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
			log.Crit("listen error", "subject", "http", "error", err)
			stopServer <- struct{}{}
		}
	}()

	// start terminal ui
	stopUi := make(chan struct{}, 1)
	go func() {
		err := UiSetup(ChanSize)
		if err != nil {
			log.Crit("init failed", "subject", "terminal", "error", err)
		}
		stopUi <- struct{}{}
	}()

	// heartbeat
	go func() {
		node.HeartBeat()
		stopServer <- struct{}{}
	}()

	// wait
	select {
	case <-stopServer:
		ui.StopLoop()
	case <-stopUi:
	}

	// shutdown server gracefully
	log.Info("Shutting down the server...")
	log.Info("Shutting down the server...")
	ctx, _ := context.WithTimeout(context.Background(), time.Minute)
	_ = server.Shutdown(ctx)
}
