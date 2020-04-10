/*===----------- nats.go - nats interface written in go  -------------===
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
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"github.com/nats-io/nats.go"
)

////////////////////////////////////////
// Interface Implementations
////////////////////////////////////////

//////////////////////////////////////// NATS
// Connect initiates the primary connection to the range of provided URLs
func (i *NatsService) connect() error {
	err := fmt.Errorf("Could not connect to NATS")

	certFile := i.Configuration.Cert
	keyFile := i.Configuration.Key
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		log.Fatalf("[ERROR] Parsing X509 certificate/key pair: %v", err)
	}

	rootPEM, err := ioutil.ReadFile(i.Configuration.CACert)

	pool := x509.NewCertPool()
	ok := pool.AppendCertsFromPEM([]byte(rootPEM))
	if !ok {
		log.Fatalln("[ERROR] Failed to parse root certificate.")
	}

	config := &tls.Config{
		//ServerName:         i.Configuration.Hosts[0],
		Certificates:       []tls.Certificate{cert},
		RootCAs:            pool,
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: i.Configuration.Secure, //TODO: SECURITY THREAT
	}

	if i.nc, err = nats.Connect(strings.Join(i.Configuration.Hosts[:], ","), nats.Secure(config)); err != nil {
		fmt.Println("[ERROR] Connecting to NATS:", err)
		return err
	}
	if i.ec, err = nats.NewEncodedConn(i.nc, nats.JSON_ENCODER); err != nil {
		fmt.Println("[ERROR] Encoding NATS:", err)
		return err
	}
	i.Configuration.Session = i
	return nil
}

//////////////////////////////////////// NATS
// Close
//will terminate the session to the backend, returning error if an issue arises
func (i *NatsService) close() error {
	i.ec.Drain()
	i.ec.Close()
	i.nc.Drain()
	i.nc.Close()
	return nil
}

//////////////////////////////////////// NATS
// Write
func (i *NatsService) write(w *WriteArgs) error {
	// sendCh := make(chan *map[string]interface{})
	// i.ec.Publish(i.Configuration.Context, w.Values)
	// i.ec.BindSendChan(i.Configuration.Context, sendCh)
	// sendCh <- w.Values
	return i.ec.Publish(i.Configuration.Context, w.Values)
}

func (i *NatsService) serve(w *http.ResponseWriter, r *http.Request, s *ServiceArgs) error {
	//TODO: Nats services
	return fmt.Errorf("[ERROR] Nats service not implemented")
}

//////////////////////////////////////// NATS
// Listen
func (i *NatsService) listen() error {
	for idx := range i.Configuration.Filter {
		f := &i.Configuration.Filter[idx]
		i.nc.QueueSubscribe(f.Id, NATS_QUEUE_GROUP, func(m *nats.Msg) {
			j := make(map[string]interface{})
			if err := json.Unmarshal(m.Data, &j); err == nil {
				wargs := WriteArgs{
					Values: &j,
				}
				switch f.Alias {

				default:
				}
				if wargs.WriteType != 0 {
					for idx2 := range i.AppConfig.Notify {
						n := &i.AppConfig.Notify[idx2]
						if n.Session != nil {
							if err := n.Session.write(&wargs); err != nil {
								fmt.Printf("[ERROR] Writing to %s: %s\n", n.Service, err)
							}
						}
					}
				}
			}
		})
	}
	return nil
}
