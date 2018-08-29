package main

import (
	"fmt"
	"net/http"
	"path"
	"os"
	"io"
	"regexp"
	"io/ioutil"
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
		// get token from request
		token := r.URL.Query().Get("token")
		if token == "" {
			fmt.Println("Shard request has no token")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		// check if token is valid
		_, err := BoltDbGet([]byte(token), BucketToken)
		if err != nil {
			fmt.Printf("Token not exist: %v\n", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// check if shard already exist
		hash := r.URL.Path[len("/shards/"):]
		fPath := path.Join(Cfg.GetShardsPath(), hash)
		_, err = os.Stat(fPath)

		// download shard
		if r.Method == "GET" {
			fmt.Printf("retrieving token: %v\n", token)
			if os.IsNotExist(err) {
				fmt.Printf("Shard not exist for download: %v\n", hash)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			fHandle, err := os.Open(fPath)
			if err != nil {
				// TODO: close handle
				fmt.Printf("open file for download error: %v\n", hash)
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_, err = io.Copy(w, fHandle)
			if err != nil {
				fmt.Printf("stream file for download error: %v\n", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			fHandle.Close()
			fmt.Printf("retrieved %v\n", token)
			return
		}
		// upload shard
		if r.Method == "POST" {
			if os.IsExist(err) {
				fmt.Printf("Shard already exist for upload: %v\n", hash)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			// save shard
			fHandle, err := os.Create(fPath)
			_, err = io.Copy(fHandle, r.Body)
			if err != nil {
				// TODO: close handle ?
				if err != nil {
					fmt.Printf("save shard error: %v\n", err)
				}
				// if exist, remove the broken file
				_, err = os.Stat(fPath)
				if os.IsExist(err) {
					rErr := os.Remove(fPath)
					if rErr != nil {
						fmt.Printf("remove broken file: %v\n", rErr)
					}
				}
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			cErr := fHandle.Close()
			if cErr != nil {
				fmt.Printf("shard saved: %v\nclose file err: %v\n", hash, cErr)
			}
		}
	}
}
