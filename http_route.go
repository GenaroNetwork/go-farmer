package main

import (
	"io"
	"io/ioutil"
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
	logger := logger.New("subject", "shard handler")
	return func(w http.ResponseWriter, r *http.Request) {
		dataHash := r.URL.Path[len("/shards/"):]

		// get token from request
		token := r.URL.Query().Get("token")
		if token == "" {
			logger.Info("request has no token", "data_hash", dataHash)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		// check if token is valid
		_, err := BoltDbGet([]byte(token), BucketToken)
		if err != nil {
			logger.Info("check token existence error", "data_hash", dataHash, "error", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// check if shard already exist
		fPath := path.Join(Cfg.GetShardsPath(), dataHash)
		_, err = os.Stat(fPath)

		// download shard
		if r.Method == "GET" {
			logger := logger.New("method", "GET")
			logger.Info("", "data_hash", dataHash, "token", token)
			if os.IsNotExist(err) {
				logger.Warn("no shard", "data_hash", dataHash, "token", token)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			fHandle, err := os.Open(fPath)
			if err != nil {
				// TODO: close handle?
				logger.Warn("open shard error", "data_hash", dataHash, "token", token, "error", err)
				w.WriteHeader(http.StatusNotFound)
				return
			}
			defer fHandle.Close()
			_, err = io.Copy(w, fHandle)
			if err != nil {
				logger.Warn("copy shard error", "data_hash", dataHash, "token", token, "error", err)
				return
			}
			logger.Info("GET shard success", "data_hash", dataHash, "token", token)
			return
		}
		// upload shard
		if r.Method == "POST" {
			logger := logger.New("method", "POST")
			if os.IsExist(err) {
				logger.Warn("shard already exist", "data_hash", dataHash, "token", token)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			// save shard
			fHandle, err := os.Create(fPath)
			if err != nil {
				logger.Warn("create shard error", "data_hash", dataHash, "token", token, "error", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			_, err = io.Copy(fHandle, r.Body)
			if err != nil {
				// TODO: close handle ?
				if err != nil {
					logger.Warn("save shard error", "data_hash", dataHash, "token", token, "error", err)
				}
				// if exist, remove the broken file
				_, err = os.Stat(fPath)
				if os.IsExist(err) {
					rErr := os.Remove(fPath)
					if rErr != nil {
						logger.Warn("remove broken file error", "data_hash", dataHash, "token", token, "error", err)
					}
				}
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			cErr := fHandle.Close()
			if cErr != nil {
				logger.Warn("close file error", "data_hash", dataHash, "token", token, "error", err)
			}
			logger.Info("POST shard success", "data_hash", dataHash, "token", token)
		}
	}
}
