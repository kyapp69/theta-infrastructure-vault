package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/gorilla/rpc/v2"
	json "github.com/gorilla/rpc/v2/json2"
	_ "github.com/lib/pq"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/thetatoken/vault/db"
	"github.com/thetatoken/vault/faucet"
	"github.com/thetatoken/vault/handler"
	"github.com/thetatoken/vault/keymanager"
	"github.com/thetatoken/vault/util"
	rpcc "github.com/ybbus/jsonrpc"
	"golang.org/x/net/netutil"
)

func decompressMiddleware(handler http.Handler) http.Handler {
	logger := log.WithFields(log.Fields{"method": "rpc.handler.decompress"})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("content-encoding"), "gzip") {
			handler.ServeHTTP(w, r)
			return
		}

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			logger.WithFields(log.Fields{"error": err}).Error("Error reading body")
			http.Error(w, "can't read body", http.StatusBadRequest)
			return
		}
		// And now set a new body, which will simulate the same data we read:
		r.Body, err = gzip.NewReader(bytes.NewBuffer(body))
		if err != nil {
			logger.WithFields(log.Fields{"error": err}).Error("Error decompressing request body")
			http.Error(w, "Error decompressing request body", http.StatusBadRequest)
			return
		}

		handler.ServeHTTP(w, r)
	})
}

func startServer(da *db.DAO) {
	logger := log.WithFields(log.Fields{"method": "rpc.startServer"})

	s := rpc.NewServer()
	s.RegisterCodec(json.NewCodec(), "application/json")
	s.RegisterCodec(json.NewCodec(), "application/json;charset=UTF-8")
	client := rpcc.NewRPCClient("http://localhost:16888/rpc")
	keyManager, err := keymanager.NewSqlKeyManager(da)

	if err != nil {
		logger.Fatal(err)
	}
	defer keyManager.Close()

	handler := handler.NewRPCHandler(client, keyManager)
	s.RegisterService(handler, "theta")
	r := mux.NewRouter()
	r.Use(util.LoggerMiddleware)
	r.Use(decompressMiddleware)
	r.Handle("/rpc", s)

	port := viper.GetString("RPCPort")
	l, err := net.Listen("tcp", ":"+port)
	if err != nil {
		logger.Fatalf("Listen: %v", err)
	}
	defer l.Close()

	logger.Info(fmt.Sprintf("Listening on %s\n", port))
	l = netutil.LimitListener(l, viper.GetInt("MaxConnections"))
	logger.Fatal(http.Serve(l, r))
	return
}

func startFaucet(da *db.DAO) {
	f := faucet.NewFaucetManager(da)
	f.Process()
}

func main() {
	util.SetupLogger()
	util.ReadConfig()

	da, err := db.NewDAO()
	if err != nil {
		log.Fatal(err)
	}
	defer da.Close()

	go startFaucet(da)
	go startServer(da)

	select {}
}
