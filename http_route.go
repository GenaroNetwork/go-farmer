package main

import (
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"regexp"
)

type route struct {
	pattern *regexp.Regexp
	handler http.Handler
}

type RegexpHandler struct {
	routes []*route
}

func (h *RegexpHandler) Handler(pattern *regexp.Regexp, handler http.Handler) {
	h.routes = append(h.routes, &route{pattern, handler})
}

func (h *RegexpHandler) HandleFunc(pattern *regexp.Regexp, handler func(http.ResponseWriter, *http.Request)) {
	h.routes = append(h.routes, &route{pattern, http.HandlerFunc(handler)})
}

func (h *RegexpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for _, route := range h.routes {
		p := r.URL.Path
		m := route.pattern.MatchString(p)
		if m {
			route.handler.ServeHTTP(w, r)
			return
		}
	}
	// no pattern matched; send 404 response
	http.NotFound(w, r)
}

func RootHandler(node INode) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.Write([]byte("Hello!"))
		}
		if r.Method == "POST" {
			msgInOut := &MsgInOut{}
			body := http.MaxBytesReader(w, r.Body, KB*32)
			// TODO: handle error
			msgInRaw, _ := ioutil.ReadAll(body)
			msgInOut.SetMsgInRaw(msgInRaw)
			node.ProcessMsgInOut(msgInOut)
			w.Header().Set("content-type", "application/json")
			w.Write([]byte(msgInOut.String()))
		}
	}
}

func ShardHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dataHash := r.URL.Path[len("/shards/"):]

		// get token from request
		token := r.URL.Query().Get("token")
		if token == "" {
			log.Printf("[SHARD] request has no token DATA_HASH=%v\n", dataHash)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		// check if token is valid
		_, err := BoltDbGet([]byte(token), BucketToken)
		if err != nil {
			log.Printf("[SHARD] check token existence error DATA_HASH=%v ERROR=%v\n", dataHash, err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// check if shard already exist
		fPath := path.Join(Cfg.GetShardsPath(), dataHash)
		_, err = os.Stat(fPath)

		// download shard
		if r.Method == "GET" {
			log.Printf("[SHARD GET] DATA_HASH=%v TOKEN=%v\n", dataHash, token)
			if os.IsNotExist(err) {
				log.Printf("[SHARD GET] no shard DATA_HASH=%v TOKEN=%v\n", dataHash, token)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			fHandle, err := os.Open(fPath)
			if err != nil {
				// TODO: close handle?
				log.Printf("[SHARD GET] open shard error DATA_HASH=%v TOKEN=%v ERROR=%v\n", dataHash, token, err)
				w.WriteHeader(http.StatusNotFound)
				return
			}
			defer fHandle.Close()
			_, err = io.Copy(w, fHandle)
			if err != nil {
				log.Printf("[SHARD GET] copy shard error DATA_HASH=%v TOKEN=%v ERROR=%v\n", dataHash, token, err)
				return
			}
			log.Printf("[SHARD GET] success DATA_HASH=%v TOKEN=%v\n", dataHash, token)
			return
		}
		// upload shard
		if r.Method == "POST" {
			if os.IsExist(err) {
				log.Printf("[SHARD POST] shard already exist DATA_HASH=%v TOKEN=%v\n", dataHash, token)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			// save shard
			fHandle, err := os.Create(fPath)
			if err != nil {
				log.Printf("[SHARD POST] create shard error DATA_HASH=%v TOKEN=%v ERROR=%v\n", dataHash, token, err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			_, err = io.Copy(fHandle, r.Body)
			if err != nil {
				// TODO: close handle ?
				if err != nil {
					log.Printf("[SHARD POST] save shard error DATA_HASH=%v TOKEN=%v ERROR=%v\n", dataHash, token, err)
				}
				// if exist, remove the broken file
				_, err = os.Stat(fPath)
				if os.IsExist(err) {
					rErr := os.Remove(fPath)
					if rErr != nil {
						log.Printf("[SHARD POST] remove broken file error DATA_HASH=%v TOKEN=%v ERROR=%v\n", dataHash, token, rErr)
					}
				}
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			cErr := fHandle.Close()
			if cErr != nil {
				log.Printf("[SHARD POST] close file error DATA_HASH=%v TOKEN=%v ERROR=%v\n", dataHash, token, cErr)
			}
			log.Printf("[SHARD POST] success DATA_HASH=%v TOKEN%v\n", dataHash, token)
		}
	}
}
