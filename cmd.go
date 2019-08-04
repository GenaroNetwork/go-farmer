package main

import (
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto/secp256k1"
	log "github.com/inconshreveable/log15"
)

func ParseCmdArgs() {
	const helpMsg = `usage: go-farmer <command> [<args>]
Commands:
	new      create a new configuration file
	start    start a farmer instance

See "go-farmer help <command>" for information on a specific command.
`
	/* start command */
	startCmd := flag.NewFlagSet("start", flag.ExitOnError)
	// -config
	sConfigPath := startCmd.String("config", "./config.json", "config file path")

	/* new-account command */
	newAccountCmd := flag.NewFlagSet("new", flag.ExitOnError)
	// -config
	sNewConfigPath := newAccountCmd.String("config", "./", "config file path")

	if len(os.Args) == 1 {
		fmt.Print(helpMsg)
		os.Exit(2)
	}

	switch os.Args[1] {
	case "start":
		_ = startCmd.Parse(os.Args[2:])
		parseConfigFile(sConfigPath)
	case "new":
		_ = newAccountCmd.Parse(os.Args[2:])
		doCreateCfgfile(sNewConfigPath)
		os.Exit(0)
	case "help":
		if len(os.Args) != 3 {
			fmt.Print(helpMsg)
			os.Exit(2)
		}
		switch os.Args[2] {
		case "start":
			startCmd.Usage()
		case "new":
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

func doCreateCfgfile(cNewConfigPath *string) {
	dir := path.Dir(*cNewConfigPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Printf("make directory failed: %v\n", err)
		os.Exit(2)
	}
	pk, err := ecdsa.GenerateKey(secp256k1.S256(), rand.Reader)
	if err != nil {
		fmt.Printf("generate private key failed: %v\n", err)
		os.Exit(2)
	}
	pkSer := math.PaddedBigBytes(pk.D, pk.Params().BitSize/8)
	pkSerStr := hex.EncodeToString(pkSer)
	defaultCfg := fmt.Sprintf(`{
  "local_addr": "local_public_ip:5003",
  "private_key": "%v",
  "data_dir": "/path/to/data",
  "seed_list": [
    "genaro://renter_ip:4000/337472da3068fa05d415262baf4df5bada8aefdc"
  ],
  "log_dir": "./"
}
`, pkSerStr)
	err = ioutil.WriteFile(*cNewConfigPath, []byte(defaultCfg), 0755)
	if err != nil {
		fmt.Printf("write configuration file failed: %v\n", err)
		os.Exit(2)
	}
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
