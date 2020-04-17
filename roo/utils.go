/*===----------- utils.go - tracking utility written in go  -------------===
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
	"crypto/sha1"
	"encoding/base64"
	"hash/fnv"
	"net"
	"net/http"
	"strings"
)

////////////////////////////////////////
// hash
////////////////////////////////////////
func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

func sha(s string) string {
	hasher := sha1.New()
	hasher.Write([]byte(s))
	return base64.URLEncoding.EncodeToString(hasher.Sum(nil))
}

func cleanInterfaceString(i interface{}) error {
	s := &i
	if temp, ok := (*s).(string); ok {
		*s = strings.ToLower(strings.TrimSpace(temp))
	}
	return nil
}

func cleanString(s *string) error {
	if s != nil && *s != "" {
		*s = strings.ToLower(strings.TrimSpace(*s))
	}
	return nil
}

func getIP(r *http.Request) string {
	ip := r.Header.Get("X-Forwarded-For")
	if ip == "" {
		var err error
		if ip, _, err = net.SplitHostPort(r.RemoteAddr); err != nil {
			ip = r.RemoteAddr
		}
	}
	return cleanIP(ip)
}

func cleanIP(ip string) string {
	ipa := strings.Split(ip, ",")
	for i := len(ipa) - 1; i > -1; i-- {
		ipa[i] = strings.TrimSpace(ipa[i])
		if ipp := net.ParseIP(ipa[i]); ipp != nil {
			return ipp.String()
		}
	}
	return ""
}

func getHost(r *http.Request) string {
	if addr, _, err := net.SplitHostPort(r.Host); err != nil {
		return r.Host
	} else {
		return addr
	}
}

func getMyIPs(filterIPv4 bool) ([]net.IP, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	ips := make([]net.IP, 0, 5)
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			return nil, err
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip != nil && (filterIPv4 && ip.To4() != nil) {
				ips = append(ips, ip)
			}
		}
	}
	return ips, err
}

func getIPsString(ipbs []net.IP) []string {
	ips := make([]string, 0, 5)
	for _, ip := range ipbs {
		ips = append(ips, ip.String())
	}
	return ips
}
