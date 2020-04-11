/*===----------- roo.go - bouncy distributed transparent proxy -------------===
 *
 *
 * This file is licensed under the Apache 2 License. See LICENSE for details.
 *
 *  Copyright (c) 2018 Andrew Grosser. All Rights Reserved.
 *
 *                                     `...
 *                                    yNMMh`
 *                                    dMMMh`
 *                                    dMMMh`
 *                                    dMMMh`
 *                                    dMMMd`
 *                                    dMMMm.
 *                                    dMMMm.
 *                                    dMMMm.               /hdy.
 *                  ohs+`             yMMMd.               yMMM-
 *                 .mMMm.             yMMMm.               oMMM/
 *                 :MMMd`             sMMMN.               oMMMo
 *                 +MMMd`             oMMMN.               oMMMy
 *                 sMMMd`             /MMMN.               oMMMh
 *                 sMMMd`             /MMMN-               oMMMd
 *                 oMMMd`             :NMMM-               oMMMd
 *                 /MMMd`             -NMMM-               oMMMm
 *                 :MMMd`             .mMMM-               oMMMm`
 *                 -NMMm.             `mMMM:               oMMMm`
 *                 .mMMm.              dMMM/               +MMMm`
 *                 `hMMm.              hMMM/               /MMMm`
 *                  yMMm.              yMMM/               /MMMm`
 *                  oMMm.              oMMMo               -MMMN.
 *                  +MMm.              +MMMo               .MMMN-
 *                  +MMm.              /MMMo               .NMMN-
 *           `      +MMm.              -MMMs               .mMMN:  `.-.
 *          /hys:`  +MMN-              -NMMy               `hMMN: .yNNy
 *          :NMMMy` sMMM/              .NMMy                yMMM+-dMMMo
 *           +NMMMh-hMMMo              .mMMy                +MMMmNMMMh`
 *            /dMMMNNMMMs              .dMMd                -MMMMMNm+`
 *             .+mMMMMMN:              .mMMd                `NMNmh/`
 *               `/yhhy:               `dMMd                 /+:`
 *                                     `hMMm`
 *                                     `hMMm.
 *                                     .mMMm:
 *                                     :MMMd-
 *                                     -NMMh.
 *                                      ./:.
 *
 *===----------------------------------------------------------------------===
 */
package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"golang.org/x/crypto/acme/autocert"
)

////////////////////////////////////////
// Start here
////////////////////////////////////////
func main() {
	fmt.Println("\n\n//////////////////////////////////////////////////////////////")
	fmt.Println("Roo.")
	fmt.Println("Transparent proxy suitable for clusters and swarm")
	fmt.Println("https://github.com/sfproductlabs/roo")
	fmt.Println("(c) Copyright 2018 SF Product Labs LLC.")
	fmt.Println("Use of this software is subject to the LICENSE agreement.")
	fmt.Println("//////////////////////////////////////////////////////////////\n\n")

	//////////////////////////////////////// LOAD CONFIG
	fmt.Println("Starting services...")
	configFile := "config.json"
	if len(os.Args) > 1 {
		configFile = os.Args[1]
	}
	fmt.Println("Configuration file: ", configFile)
	file, _ := os.Open(configFile)
	defer file.Close()
	decoder := json.NewDecoder(file)
	configuration := Configuration{}
	err := decoder.Decode(&configuration)
	if err != nil {
		log.Fatalf("[ERROR] Configuration file has errors %s", err)
	}

	////////////////////////////////////////SETUP ORIGIN
	if configuration.AllowOrigin == "" {
		configuration.AllowOrigin = "*"
	}

	//////////////////////////////////////// SETUP CONFIG VARIABLES
	apiVersion := "v" + strconv.Itoa(configuration.ApiVersion)

	//////////////////////////////////////// LOAD CLUSTER
	{
		s := &configuration.Cluster
		switch s.Service.Service {
		case SERVICE_TYPE_KV:
			kv := KvService{
				Configuration: s.Service,
				AppConfig:     &configuration,
			}
			err = kv.connect()
			if err != nil || s.Service.Session == nil {
				if s.Service.Critical {
					log.Fatalf("[CRITICAL] Could not connect to RAFT Cluster. %s\n", err)
				} else {
					fmt.Printf("[ERROR] Could not connect to RAFT Cluster. %s\n", err)
				}

			} else {
				fmt.Printf("Cluster: Connected to RAFT: %s\n", s.Service.Hosts)
			}
			s.Service.Session.listen()
		default:
			panic("[ERROR] Cluster not implemented\n")
		}
	}

	//////////////////////////////////////// LOAD NOTIFIERS (OUTBOUND TRAFFIC)
	for idx := range configuration.Notify {
		s := &configuration.Notify[idx]
		switch s.Service {
		case SERVICE_TYPE_NATS:
			fmt.Printf("Notifier #%d: Connecting to NATS Cluster: %s\n", idx, s.Hosts)
			gonats := NatsService{
				Configuration: s,
				AppConfig:     &configuration,
			}
			err = gonats.connect()
			if err != nil || s.Session == nil {
				if s.Critical {
					log.Fatalf("[CRITICAL] Notifier #%d. Could not connect to NATS Cluster. %s\n", idx, err)
				} else {
					fmt.Printf("[ERROR] Notifier #%d. Could not connect to NATS Cluster. %s\n", idx, err)
					continue
				}

			} else {
				fmt.Printf("Notifier #%d: Connected to NATS.\n", idx)
			}
		default:
			fmt.Printf("[ERROR] %s #%d Notifier not implemented\n", s.Service, idx)
		}
	}

	//////////////////////////////////////// LOAD CONSUMERS (INBOUND TRAFFIC)
	for idx := range configuration.Consume {
		s := &configuration.Consume[idx]
		switch s.Service {
		case SERVICE_TYPE_NATS:
			fmt.Printf("Consume #%d: Connecting to NATS Cluster: %s\n", idx, s.Hosts)
			gonats := NatsService{
				Configuration: s,
				AppConfig:     &configuration,
			}
			err = gonats.connect()
			if err != nil || s.Session == nil {
				if s.Critical {
					log.Fatalf("[CRITICAL] Consumer #%d. Could not connect to NATS Cluster. %s\n", idx, err)
				} else {
					fmt.Printf("[ERROR] Consumer #%d. Could not connect to NATS Cluster. %s\n", idx, err)
					continue
				}

			} else {
				fmt.Printf("Consumer #%d: Connected to NATS.\n", idx)
			}
			s.Session.listen()
		default:
			fmt.Printf("[ERROR] %s #%d Consumer not implemented\n", s.Service, idx)
		}

	}

	/////////////////////////////////////////
	//**************************************
	// THE APP BEGINS HERE IN ERNEST
	//**************************************
	//////////////////////////////////////// SSL CERT MANAGER
	certManager := autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(configuration.Domains...),
		Cache:      configuration.Cluster.Service.Session.(*KvService),
	}
	server := &http.Server{ // HTTP REDIR SSL RENEW
		Addr:              ":https",
		ReadTimeout:       time.Duration(configuration.ReadTimeoutSeconds) * time.Second,
		ReadHeaderTimeout: time.Duration(configuration.ReadHeaderTimeoutSeconds) * time.Second,
		WriteTimeout:      time.Duration(configuration.WriteTimeoutSeconds) * time.Second,
		IdleTimeout:       time.Duration(configuration.IdleTimeoutSeconds) * time.Second,
		MaxHeaderBytes:    configuration.MaxHeaderBytes, //1 << 20 // 1 MB
		TLSConfig: &tls.Config{ // SEC PARAMS
			GetCertificate:           certManager.GetCertificate,
			PreferServerCipherSuites: true,
			InsecureSkipVerify:       configuration.IgnoreInsecureTLS,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, // Required by Go (and HTTP/2 RFC), even if you only present ECDSA certs
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
			},
			//MinVersion:             tls.VersionTLS12,
			//CurvePreferences:       []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
		},
	}

	//////////////////////////////////////// MAX CHANNELS
	connc := make(chan struct{}, configuration.MaximumConnections)
	for i := 0; i < configuration.MaximumConnections; i++ {
		connc <- struct{}{}
	}

	//////////////////////////////////////// PROXY API ROUTES
	// TODO:
	sharedBuffer := newBufferPool()
	if configuration.ProxyUrl != "" {
		fmt.Println("Proxying to:", configuration.ProxyUrl)
		origin, _ := url.Parse(configuration.ProxyUrl)
		director := func(req *http.Request) {
			req.Header.Add("X-Forwarded-Host", req.Host)
			req.Header.Add("X-Origin-Host", origin.Host)
			if configuration.ProxyForceJson {
				req.Header.Set("content-type", "application/json")
			}
			req.URL.Scheme = "http"
			req.URL.Host = origin.Host
		}
		proxy := &httputil.ReverseProxy{Director: director, BufferPool: sharedBuffer}
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if !configuration.IgnoreProxyOptions && r.Method == http.MethodOptions {
				//Lets just allow requests to this endpoint
				w.Header().Set("access-control-allow-origin", configuration.AllowOrigin)
				w.Header().Set("access-control-allow-credentials", "true")
				w.Header().Set("access-control-allow-headers", "Authorization,Accept,X-CSRFToken,User")
				w.Header().Set("access-control-allow-methods", "GET,POST,HEAD,PUT,DELETE")
				w.Header().Set("access-control-max-age", "1728000")
				w.WriteHeader(http.StatusOK)
				return
			}
			//TODO: Check certificate in cookie
			select {
			case <-connc:
				//Check API Limit
				if err := check(&configuration, r); err != nil {
					w.WriteHeader(http.StatusTooManyRequests)
					w.Write([]byte(API_LIMIT_REACHED))
					return
				}
				//Proxy
				w.Header().Set("Strict-Transport-Security", "max-age=15768000 ; includeSubDomains")
				w.Header().Set("access-control-allow-origin", configuration.AllowOrigin)
				proxy.ServeHTTP(w, r)
				connc <- struct{}{}
			default:
				w.Header().Set("Retry-After", "1")
				http.Error(w, "Maximum clients reached on this node.", http.StatusServiceUnavailable)
			}
		})
	}

	rtr := mux.NewRouter()
	//////////////////////////////////////// OPTIONS ROUTE DEFAULT - EVERYTHING OK
	rtr.HandleFunc("/roo/"+apiVersion, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("access-control-allow-origin", configuration.AllowOrigin)
		w.Header().Set("access-control-allow-credentials", "true")
		w.Header().Set("access-control-allow-headers", "Authorization,Accept,User")
		w.Header().Set("access-control-allow-methods", "GET,POST,HEAD,PUT,DELETE")
		w.Header().Set("access-control-max-age", "1728000")
		w.WriteHeader(http.StatusOK)
	}).Methods("OPTIONS")
	//////////////////////////////////////// PING
	rtr.HandleFunc("/roo/"+apiVersion+"/ping{pong: .*}", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-connc:
			params := mux.Vars(r)
			sargs := ServiceArgs{
				ServiceType: SERVE_GET_PING,
				Values:      &params,
			}
			w.Header().Set("access-control-allow-origin", configuration.AllowOrigin)
			if err = serveWithArgs(&configuration, &w, r, &sargs); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(err.Error()))
			}
			connc <- struct{}{}
		default:
			w.Header().Set("Retry-After", "1")
			http.Error(w, "Maximum clients reached on this node.", http.StatusServiceUnavailable)
		}
	}).Methods("GET")
	//////////////////////////////////////// GET KV
	rtr.HandleFunc("/roo/kv/"+apiVersion+"/{key: .*}", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-connc:
			params := mux.Vars(r)
			sargs := ServiceArgs{
				ServiceType: SERVE_GET_KV,
				Values:      &params,
			}
			w.Header().Set("access-control-allow-origin", configuration.AllowOrigin)
			if err = serveWithArgs(&configuration, &w, r, &sargs); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(err.Error()))
			}
			connc <- struct{}{}
		default:
			w.Header().Set("Retry-After", "1")
			http.Error(w, "Maximum clients reached on this node.", http.StatusServiceUnavailable)
		}
	}).Methods("GET")
	//////////////////////////////////////// PUT KV
	rtr.HandleFunc("/roo/kv/"+apiVersion+"/{key: .*}/{value: .*}", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-connc:
			params := mux.Vars(r)
			sargs := ServiceArgs{
				ServiceType: WRITE_PUT_KV,
				Values:      &params,
			}
			w.Header().Set("access-control-allow-origin", configuration.AllowOrigin)
			if err = serveWithArgs(&configuration, &w, r, &sargs); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(err.Error()))
			}
			connc <- struct{}{}
		default:
			w.Header().Set("Retry-After", "1")
			http.Error(w, "Maximum clients reached on this node.", http.StatusServiceUnavailable)
		}
	}).Methods("PUT")
	http.Handle("/roo", rtr)

	go func() {
		http.ListenAndServe(":http", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			http.Redirect(w, req, "https://"+getHost(req)+req.RequestURI, http.StatusFound)
		}))
	}()
	log.Fatal(server.ListenAndServeTLS("", ""))
}

////////////////////////////////////////
// Serve APIs
////////////////////////////////////////
func serveWithArgs(c *Configuration, w *http.ResponseWriter, r *http.Request, args *ServiceArgs) error {
	s := &c.API
	if s != nil && s.Session != nil {
		if err := s.Session.serve(w, r, args); err != nil {
			if c.Debug {
				fmt.Printf("[ERROR] Serving to %s: %s\n", s.Service, err)
			}
			return err
		}
	}
	return nil
}

////////////////////////////////////////
// Check
////////////////////////////////////////
func check(c *Configuration, r *http.Request) error {
	//Precheck
	if c.ProxyDailyLimit > 0 && c.ProxyDailyLimitCheck != nil && c.ProxyDailyLimitCheck(getIP(r)) > c.ProxyDailyLimit {
		return fmt.Errorf("API Limit Reached")
	}
	return nil
}