package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"syscall"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	log "github.com/inconshreveable/log15"
	"golang.org/x/crypto/ssh/terminal"
)

func ParseCmdArgs() {
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
	logger := log.New("module", "cmd")
	// check if config file exists
	_, err := os.Stat(*cPath)
	if os.IsNotExist(err) {
		logger.Crit("file not exist", "subject", "config", "path", *cPath)
		os.Exit(2)
	}

	// read config file
	cnt, err := ioutil.ReadFile(*cPath)
	if err != nil {
		logger.Crit("read file error", "subject", "config", "error", err)
		os.Exit(2)
	}

	// parse config file
	err = json.Unmarshal(cnt, &Cfg)
	if err != nil {
		logger.Crit("decode file error", "subject", "config", "error", err)
		os.Exit(2)
	}
	if err := Cfg.Parse(); err != nil {
		logger.Crit("file bad format", "subject", "config", "error", err)
		os.Exit(2)
	}
	js, _ := json.MarshalIndent(Cfg, "", "  ")
	logger.Debug("successfully parsed", "subject", "config")
	fmt.Println(string(js))
}
