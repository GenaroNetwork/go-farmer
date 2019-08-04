package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"sync"
	"time"

	"github.com/GenaroNetwork/go-farmer/msg"
	"github.com/boltdb/bolt"
)

const (
	_        = iota // ignore first value by assigning to blank identifier
	KB int64 = 1 << (10 * iota)
	MB
	GB
	TB
	PB
)

func BoltDbGet(key []byte, bucket string) ([]byte, error) {
	var v []byte
	err := BoltDB.View(func(tx *bolt.Tx) error {
		// ensure bucket
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return errors.New("bucket not found")
		}
		// get value by key
		v = b.Get(key)
		if v != nil {
			return nil
		}
		return errors.New("key not found")
	})
	return v, err
}

func BoltDbSet(key, value []byte, bucket string, over bool) error {
	err := BoltDB.Update(func(tx *bolt.Tx) error {
		// ensure bucket
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			_b, err := tx.CreateBucket([]byte(bucket))
			b = _b
			if err != nil {
				return errors.New("create bucket failed")
			}
		}
		// put key
		v := b.Get([]byte(key))
		if v == nil || over == true {
			err := b.Put([]byte(key), []byte(value))
			if err != nil {
				return err
			}
			return nil
		}
		return errors.New("key already exist")
	})
	return err
}

func MagicHash(msg []byte) [32]byte {
	prefix1 := varintBufNum(len(MagicBytes))
	prefix2 := varintBufNum(len(msg))
	buf := make([]byte, 0, len(prefix1)+len(prefix2)+len(MagicBytes)+len(msg))
	buf = append(buf, prefix1...)
	buf = append(buf, []byte(MagicBytes)...)
	buf = append(buf, prefix2...)
	buf = append(buf, msg...)
	hash := sha256.Sum256(buf)
	hash = sha256.Sum256(hash[:])
	return hash
}

func varintBufNum(n int) (buf []byte) {
	if n < 253 {
		buf = make([]byte, 1)
		buf[0] = byte(n)
	} else if n < 0x10000 {
		buf = make([]byte, 1+2)
		buf[0] = 253
		for i := 1; i <= 2; i++ {
			buf[i] = byte(n % 256)
			n /= 256
		}
	} else if n < 0x100000000 {
		buf = make([]byte, 1+4)
		buf[0] = 254
		for i := 1; i <= 4; i++ {
			buf[i] = byte(n % 256)
			n /= 256
		}
	} else {
		buf = make([]byte, 8)
		buf[0] = 255
		n /= 0x100000000
		for i := 1; i <= 8; i++ {
			buf[i] = byte(n % 256)
			n /= 256
		}
	}
	return buf
}

type SendMsgHandler func() error

var clientCache = sync.Map{}

func SendMsg(c msg.Contact, m *MsgInOut, dur time.Duration, cb SendMsgHandler) error {
	// prepare request payload
	msgStr, _ := json.Marshal(m.MsgOutStruct())
	body := bytes.NewBuffer([]byte(msgStr))

	// prepare request
	req, err := http.NewRequest("POST", fmt.Sprintf("http://%v:%v", c.Address, c.Port), body)
	if err != nil {
		return err
	}
	req.Header.Set("userAgent", "8.7.3")
	req.Header.Set("content-type", "application/json")

	// do send request
	var client *http.Client
	if value, ok := clientCache.Load(dur); ok {
		if c, ok := value.(*http.Client); ok {
			client = c
		}
	}
	if client == nil {
		client = &http.Client{}
		client.Timeout = dur
		clientCache.Store(dur, client)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	// parse response
	rawBody, err := ioutil.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		logger.Warn("get resp raw body error", "subject", "SendMsg", "error", err)
		return err
	}
	m.SetMsgInRaw(rawBody)
	return cb()
}

func DownloadShard(c msg.Contact, dataHash, token string) error {
	logger := logger.New("subject", "DownloadShard")
	// check shard existence
	fPath := path.Join(Cfg.GetShardsPath(), dataHash)
	_, err := os.Stat(fPath)
	if os.IsExist(err) {
		return errors.New("shard already exist")
	}

	// prepare request
	url := fmt.Sprintf("http://%v:%v/shards/%v?token=%v", c.Address, c.Port, dataHash, token)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("userAgent", "8.7.3")

	// do send request
	dur := time.Second * 0
	var client *http.Client
	if value, ok := clientCache.Load(dur); ok {
		if c, ok := value.(*http.Client); ok {
			client = c
		}
	}
	if client == nil {
		client = &http.Client{}
		client.Timeout = dur
		clientCache.Store(dur, client)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	// download shard
	go func(path string) {
		fHandle, err := os.Create(path)
		if err != nil {
			logger.Warn("create shard error", "data_hash", dataHash, "error", err)
			// if exist, remove the broken file
			_, err = os.Stat(path)
			if os.IsExist(err) {
				rErr := os.Remove(path)
				if rErr != nil {
					logger.Warn("remove broken file error", "data_hash", dataHash, "error", rErr)
				}
			}
			return
		}
		defer fHandle.Close()
		logger.Info("downloading shard", "data_hash", dataHash)
		size, err := io.Copy(fHandle, resp.Body)
		if err != nil {
			logger.Warn("download shard error", "data_hash", dataHash, "error", err)
			return
		}
		logger.Info("downloaded shard", "data_hash", dataHash, "size", size)
		// TODO: update db ?
	}(fPath)
	return nil
}

func JsonMarshal(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

func JsonPrettyMarshal(v interface{}) string {
	b, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		return ""
	}
	return string(b)
}
