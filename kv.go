/*===----------- kv.go - Distributed Key Value interface in go  -------------===
 *
 *
 * This file is licensed under the Apache 2 License. See LICENSE for details.
 *
 *  Copyright (c) 2020 Andrew Grosser. All Rights Reserved.
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
	"encoding/json"
	"fmt"
	"net/http"
)

////////////////////////////////////////
// Interface Implementations
////////////////////////////////////////

//////////////////////////////////////// C*
// Connect initiates the primary connection to the range of provided URLs
func (i *KvService) connect() error {
	return fmt.Errorf("[ERROR] KV listen not implemented")
}

//////////////////////////////////////// C*
// Close will terminate the session to the backend, returning error if an issue arises
func (i *KvService) close() error {
	return fmt.Errorf("[ERROR] KV close not implemented")
}

func (i *KvService) listen() error {
	//TODO: Listen for KV triggers
	return fmt.Errorf("[ERROR] KV listen not implemented")
}

func (i *KvService) auth(s *ServiceArgs) error {
	return fmt.Errorf("Not implemented")
	// //TODO: AG implement JWT
	// //TODO: AG implement creds (check domain level auth)
	// if *s.Values == nil {
	// 	return fmt.Errorf("User not provided")
	// }
	// uid := (*s.Values)["uid"]
	// if uid == "" {
	// 	return fmt.Errorf("User ID not provided")
	// }
	// password := (*s.Values)["password"]
	// if password == "" {
	// 	return fmt.Errorf("User pass not provided")
	// }
}

func (i *KvService) serve(w *http.ResponseWriter, r *http.Request, s *ServiceArgs) error {
	switch s.ServiceType {
	case SERVE_GET_PING:
		json, _ := json.Marshal(map[string]interface{}{"ping": (*s.Values)["pong"]})
		(*w).Header().Set("Content-Type", "application/json")
		(*w).WriteHeader(http.StatusOK)
		(*w).Write(json)
		return nil
	default:
		return fmt.Errorf("[ERROR] KV service not implemented %d", s.ServiceType)
	}

}

//////////////////////////////////////// C*
func (i *KvService) write(w *WriteArgs) error {
	err := fmt.Errorf("Could not write to any kv server in cluster")
	//v := *w.Values
	switch w.WriteType {
	case WRITE_JS:
		//v["data"]?
		return err
	default:
		//TODO: Manually run query via query in config.json
		if i.AppConfig.Debug {
			fmt.Printf("UNHANDLED %s\n", w)
		}
	}

	//TODO: Retries
	return err
}
