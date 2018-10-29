package config

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/GenaroNetwork/go-farmer/msg"
)

type Config struct {
	LocalAddr string   `json:"local_addr"`
	KeyFile   string   `json:"keyfile"`
	DataDir   string   `json:"data_dir"`
	SeedList  []string `json:"seed_list"`
	LogFile   string   `json:"log_file"`

	localIP   string
	localPort uint16
	seedList  []msg.Contact
}

func (c *Config) GetLocalPort() uint16 {
	return c.localPort
}

func (c *Config) GetLocalPortStr() string {
	return strconv.FormatUint(uint64(c.localPort), 10)
}

func (c *Config) GetLocalAddr() string {
	return c.localIP
}

func (c *Config) Parse() error {
	// parse local addr
	_, other, suc := splitScheme(c.LocalAddr)
	if suc == false {
		other = c.LocalAddr
	}
	ip, port, err := parseAddr(other)
	if err != nil {
		return fmt.Errorf("local_addr invalid: %v", err)
	}
	c.localPort = port
	c.localIP = ip.String()

	// check if keyfile exists
	fInfo, err := os.Stat(c.KeyFile)
	if os.IsNotExist(err) {
		return errors.New("keyfile not exist")
	}
	if fInfo.IsDir() {
		return errors.New("keyfile is a directory")
	}

	// validate data dir
	if c.DataDir == "" {
		return errors.New("data_dir is empty")
	}
	fInfo, err = os.Stat(c.DataDir)
	if os.IsNotExist(err) {
		return errors.New("data dir not exist")
	}
	if fInfo.IsDir() == false {
		return errors.New("data dir is not directory")
	}
	shardPath := path.Join(c.DataDir, "shards")
	fInfo, err = os.Stat(shardPath)
	if os.IsNotExist(err) {
		err := os.Mkdir(shardPath, 0700)
		if err != nil {
			return fmt.Errorf("create shards dir failed: %v", err)
		}
	}

	// validate seed list
	if c.SeedList == nil {
		return errors.New("seed_list is empty")
	}
	c.seedList = make([]msg.Contact, 0)
	for _, seed := range c.SeedList {
		seed = strings.TrimSpace(seed)
		_, addr, _ := splitScheme(seed)
		if addr[len(addr)-1] == '/' {
			addr = addr[:len(addr)-1]
		}
		seps := strings.Split(addr, "/")
		if len(seps) != 2 {
			return fmt.Errorf("seed bad format: %v", seed)
		}

		// validate addr:port
		ip, port, err := parseAddr(seps[0])
		if err != nil {
			return fmt.Errorf("seed addr error: %v", err)
		}

		// validate seed id
		err = isValidHexStr(seps[1])
		if err != nil {
			return fmt.Errorf("seed id error: %v", err)
		}

		contact := msg.Contact{
			Address: ip.String(),
			Port:    port,
			NodeID:  seps[1],
		}
		c.seedList = append(c.seedList, contact)
	}
	return nil
}

func (c *Config) GetSeedList() []msg.Contact {
	return c.seedList
}

func (c *Config) GetContractDBPath() string {
	return path.Join(c.DataDir, "contract.db")
}

func (c *Config) GetShardsPath() string {
	return path.Join(c.DataDir, "shards")
}

/************* functions ***************/

// http://127.0.0.1:8080 => (http://, 127.0.0.1:8080)
func splitScheme(addr string) (scheme string, other string, succ bool) {
	schemes := []string{"http://", "https://", "genaro://"}
	for _, scheme = range schemes {
		if strings.HasPrefix(addr, scheme) {
			other = addr[len(scheme):]
			succ = true
			return
		}
	}
	return "", addr, false
}

// 110.120.111.23:9089
func parseAddr(addr string) (net.IP, uint16, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return nil, 0, errors.New("addr is empty")
	}
	seps := strings.Split(addr, ":")
	if len(seps) != 2 {
		return nil, 0, errors.New("no port supplied")
	}
	ip := net.ParseIP(seps[0])
	if ip == nil {
		return nil, 0, errors.New("ip is invalid")
	}
	port, err := strconv.ParseUint(seps[1], 10, 16)
	if err != nil {
		return ip, 0, errors.New("port is invalid")
	}
	return ip, uint16(port), nil
}

func isValidHexStr(str string) error {
	if _, err := hex.DecodeString(str); err != nil {
		return err
	}
	return nil
}
