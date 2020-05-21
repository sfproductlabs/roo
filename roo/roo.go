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
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/patrickmn/go-cache"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

////////////////////////////////////////
// Start here
////////////////////////////////////////
func main() {
	fmt.Println("\n\n//////////////////////////////////////////////////////////////")
	fmt.Println("Roo. Version 68")
	fmt.Println("Transparent proxy suitable for clusters and swarm")
	fmt.Println("https://github.com/sfproductlabs/roo")
	fmt.Println("(c) Copyright 2018 SF Product Labs LLC.")
	fmt.Println("Use of this software is subject to the LICENSE agreement.")
	fmt.Println("//////////////////////////////////////////////////////////////\n\n ")

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
		log.Fatalf("[ERROR] Configuration file has errors or file missing %s", err)
	}

	////////////////////////////////////////SECURITY OVERRIDES
	http.DefaultServeMux = http.NewServeMux() //This prevents profiling info available on public routes

	////////////////////////////////////////CONFIGURATION OVERRIDES
	if envDNS := os.Getenv(ENV_ROO_DNS); envDNS != "" {
		configuration.Cluster.DNS = envDNS
	}
	if envResolver := os.Getenv(ENV_ROO_RESOLVER); envResolver != "" {
		configuration.Cluster.Resolver = envResolver
	}

	//////////////////////////////////////// SETUP CONFIG VARIABLES & OVERRIDES
	configuration.ApiVersionString = "v" + strconv.Itoa(configuration.ApiVersion)
	if configuration.AllowOrigin == "" {
		configuration.AllowOrigin = "*"
	}
	if configuration.SwarmRefreshSeconds == 0 {
		configuration.SwarmRefreshSeconds = 60
	}

	////////////////////////////////////////FIXED DELAY

	if envStartDelay, _ := strconv.ParseInt(os.Getenv(ENV_ROO_START_DELAY), 10, 64); envStartDelay > 0 {
		configuration.Cluster.StartDelaySeconds = envStartDelay
	}

	if configuration.Cluster.DNS != "" && configuration.Cluster.DNS != "localhost" {
		if configuration.Cluster.StartDelaySeconds == 0 {
			configuration.Cluster.StartDelaySeconds = 10
		}
		rlog.Infof("[INFO] Sleeping this node for %d seconds before boot. Letting DNS settle.\n", configuration.Cluster.StartDelaySeconds)
		time.Sleep(time.Duration(configuration.Cluster.StartDelaySeconds) * time.Second)
	}

	////////////////////////////////////////DNS QUERY
	rlog.Infof("Cluster: Looking up hosts at: %s\n", configuration.Cluster.DNS)
	if configuration.Cluster.Resolver == "" {
		rlog.Infof("Cluster: DNS Resolver: Using the OS default\n")
		configuration.Cluster.Service.Hosts, _ = net.LookupHost(configuration.Cluster.DNS)
	} else {
		r := &net.Resolver{
			PreferGo: false,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{
					Timeout: time.Millisecond * time.Duration(10000),
				}
				return d.DialContext(ctx, "udp", configuration.Cluster.Resolver+":53")
			},
		}
		rlog.Infof("Cluster: DNS Resolver: %s\n", configuration.Cluster.Resolver)
		var err error
		if configuration.Cluster.Service.Hosts, err = r.LookupHost(context.Background(), configuration.Cluster.DNS); err != nil {
			log.Fatalf("[CRITICAL] Cluster: DNS Resolver failed %v", err)
		}
	}
	rlog.Infof("Cluster: Possible Roo Peer IPs: %s\n", configuration.Cluster.Service.Hosts)

	//////////////////////////////////////// MAX CHANNELS
	connc := make(chan struct{}, configuration.MaximumConnections)
	for i := 0; i < configuration.MaximumConnections; i++ {
		connc <- struct{}{}
	}

	//////////////////////////////////////// MAX CALLS
	// This should prevent DoS - or help
	if configuration.ProxyDailyLimit > 0 && configuration.ProxyDailyLimitChecker == MEMORY_CHECKER {
		c := cache.New(24*time.Hour, 10*time.Minute)
		configuration.ProxyDailyLimitCheck = func(ip string) uint64 {
			var total uint64
			if temp, found := c.Get(ip); found {
				total = temp.(uint64)
			}
			total = total + 1 //Just by checking it we increment
			c.Set(ip, total, cache.DefaultExpiration)
			return total
		}
	}

	/// API Must load before everything else so we can get status updates
	//////////////////////////////////////// API ON :6299 (by default)
	rtr := mux.NewRouter()
	rtr.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	rtr.HandleFunc("/debug/pprof/profile", pprof.Profile)
	rtr.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	rtr.HandleFunc("/debug/pprof/trace", pprof.Trace)
	rtr.HandleFunc("/debug/pprof/{page:.*}", pprof.Index)
	//////////////////////////////////////// OPTIONS ROUTE DEFAULT - EVERYTHING OK
	rtr.HandleFunc("/roo/"+configuration.ApiVersionString, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("access-control-allow-origin", configuration.AllowOrigin)
		w.Header().Set("access-control-allow-credentials", "true")
		w.Header().Set("access-control-allow-headers", "Authorization,Accept,X-CSRFToken,User,Content-Type,Header")
		w.Header().Set("access-control-allow-methods", "GET,POST,HEAD,PUT,DELETE")
		w.Header().Set("access-control-max-age", "1728000")
		w.WriteHeader(http.StatusOK)
	}).Methods("OPTIONS")
	//////////////////////////////////////// PING
	rtr.HandleFunc("/roo/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("access-control-allow-origin", configuration.AllowOrigin)
		w.Write([]byte("PONG"))
	}).Methods("GET")
	//////////////////////////////////////// STATUS
	rtr.HandleFunc("/roo/"+configuration.ApiVersionString+"/status", func(w http.ResponseWriter, r *http.Request) {
		status := &ClusterStatus{
			Client:       getIP(r),
			Binding:      configuration.Cluster.Binding,
			Conns:        configuration.MaximumConnections - len(connc),
			NodeID:       configuration.Cluster.NodeID,
			Group:        configuration.Cluster.Group,
			Hosts:        configuration.Cluster.Service.Hosts,
			Instantiated: configuration.Cluster.Service.Instantiated,
			Started:      configuration.Cluster.Service.Started,
		}
		json, _ := json.Marshal(status)
		w.WriteHeader(http.StatusOK)
		w.Header().Set("access-control-allow-origin", configuration.AllowOrigin)
		w.Header().Set("Content-Type", "application/json")
		w.Write(json)
	}).Methods("GET")
	//////////////////////////////////////// JOIN
	rtr.HandleFunc("/roo/"+configuration.ApiVersionString+"/join", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-connc:
			params := mux.Vars(r)
			sargs := ServiceArgs{
				ServiceType: SERVE_POST_JOIN,
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
	}).Methods("POST")
	//////////////////////////////////////// REMOVE NODE
	rtr.HandleFunc("/roo/"+configuration.ApiVersionString+"/remove", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-connc:
			params := mux.Vars(r)
			sargs := ServiceArgs{
				ServiceType: SERVE_POST_REMOVE,
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
	}).Methods("POST")
	//////////////////////////////////////// RESCUE (REMOVE ALL NODES)
	rtr.HandleFunc("/roo/"+configuration.ApiVersionString+"/rescue", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-connc:
			params := mux.Vars(r)
			sargs := ServiceArgs{
				ServiceType: SERVE_POST_RESCUE,
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
	}).Methods("POST")
	//////////////////////////////////////// SCAN KV
	rtr.HandleFunc("/roo/"+configuration.ApiVersionString+"/kvs{key:.*}", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-connc:
			params := mux.Vars(r)
			sargs := ServiceArgs{
				ServiceType: SERVE_GET_KVS,
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
	rtr.HandleFunc("/roo/"+configuration.ApiVersionString+"/kv/{key}", func(w http.ResponseWriter, r *http.Request) {
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
	rtr.HandleFunc("/roo/"+configuration.ApiVersionString+"/kv/{key}", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-connc:
			params := mux.Vars(r)
			sargs := ServiceArgs{
				ServiceType: SERVE_PUT_KV,
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
	//////////////////////////////////////// UPDATE SWARM
	rtr.HandleFunc("/roo/"+configuration.ApiVersionString+"/swarm", func(w http.ResponseWriter, r *http.Request) {
		if err := serveWithArgs(&configuration, &w, r, &ServiceArgs{ServiceType: SERVE_POST_SWARM}); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(err.Error()))
		}
	}).Methods("POST")

	//////////////////////////////////////// SETUP API - INTERNAL NETWORK PORT :6299 (default) AS NOT EXPOSED
	//TODO: ADD SSL SUPPORT
	go http.ListenAndServe(API_PORT, rtr)

	//////////////////////////////////////// LOAD CLUSTER
	{
		s := &configuration.Cluster
		switch s.Service.Service {
		case SERVICE_TYPE_KV:
			s.Service.Instantiated = time.Now().UnixNano()
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
				rlog.Infof("Cluster: Connected to RAFT: %s\n", s.Service.Hosts)
			}
			//SET THE DEFAULT API TO RUN THROUGH THE KV
			configuration.API = s.Service
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

	//////////////////////////////////////// PROXY EVERYTHING
	configuration.ProxyCache = cache.New(60*time.Second, 90*time.Second)
	configuration.ProxySharedBufferPool = newBufferPool()
	acmeURL := ACME_PRODUCTION
	if configuration.AcmeStaging || os.Getenv(ENV_ROO_ACME_STAGING) == "true" {
		acmeURL = ACME_STAGING
	}
	configuration.HostCache = cache.New(4*time.Minute, 120*time.Second)
	certManager := autocert.Manager{
		Prompt: autocert.AcceptTOS,
		Cache:  configuration.Cluster.Service.Session.(*KvService),
		HostPolicy: func(ctx context.Context, name string) error {
			if !configuration.CheckHostnames {
				return nil
			}
			if configuration.Cluster.Service.Session == nil {
				return fmt.Errorf("Hostname Check Not Ready")
			}
			//First check the cache
			if exists, found := configuration.HostCache.Get(name); found {
				if exists.(bool) {
					return nil
				}
				rlog.Warningf("[HOSTNAME] Invalid check failed (AGAIN) for %s\n", name)
				return fmt.Errorf("Hostname Check Failed (Repeat)")
			}
			//Then check the kv-cluster
			ctx, cancel := context.WithTimeout(ctx, time.Duration(3*time.Second))
			defer cancel()
			if hosts, err := configuration.Cluster.Service.Session.(*KvService).execute(ctx, &KVAction{Action: SCAN, Key: HOST_PREFIX + name}); err != nil || len(hosts.(map[string][]byte)) == 0 {
				configuration.HostCache.Set(name, false, cache.DefaultExpiration)
				rlog.Warningf("[HOSTNAME] Invalid check failed for %s\n", name)
				return fmt.Errorf("Hostname Check Failed")
			}
			configuration.HostCache.Set(name, true, cache.DefaultExpiration)
			return nil
		},
		Client: &acme.Client{
			DirectoryURL: acmeURL,
		},
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
			//InsecureSkipVerify:       configuration.IgnoreInsecureTLS,
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
	//Handle Everything!
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if !configuration.IgnoreProxyOptions && r.Method == http.MethodOptions {
			//Lets just allow requests to this endpoint
			w.Header().Set("access-control-allow-origin", configuration.AllowOrigin)
			w.Header().Set("access-control-allow-credentials", "true")
			w.Header().Set("access-control-allow-headers", "Authorization,Accept,X-CSRFToken,User,Content-Type,Header")
			w.Header().Set("access-control-allow-methods", "GET,POST,HEAD,PUT,DELETE")
			w.Header().Set("access-control-max-age", "1728000")
			w.WriteHeader(http.StatusOK)
			return
		}
		select {
		case <-connc:
			//Check API Limit
			if err := check(&configuration, r); err != nil {
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(API_LIMIT_REACHED))
				return
			}
			Proxy(&w, r, &configuration)
			connc <- struct{}{}
		default:
			w.Header().Set("Retry-After", "1")
			http.Error(w, "Maximum clients reached on this node.", http.StatusServiceUnavailable)
		}
	})

	//////////////////////////////////////// ACTUALLY RUN THE SERVICES
	//TODO: Wait to start until nh is created from raft cluster
	//Redirect HTTP->HTTPS
	go http.ListenAndServe(":http", certManager.HTTPHandler(nil))

	//Start the actual Proxy Service
	log.Fatal(server.ListenAndServeTLS("", ""))

}

////////////////////////////////////////
// Serve APIs
////////////////////////////////////////
func serveWithArgs(c *Configuration, w *http.ResponseWriter, r *http.Request, args *ServiceArgs) error {
	if c != nil && c.API != nil && c.API.Session != nil {
		if err := c.API.Session.serve(w, r, args); err != nil {
			if c.Debug {
				fmt.Printf("[ERROR] Serving to %s: %s\n", c.API.Service, err)
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
