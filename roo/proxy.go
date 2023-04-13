package main

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/patrickmn/go-cache"
)

func Proxy(writer *http.ResponseWriter, r *http.Request, configuration *Configuration) {
	w := *writer
	if configuration.Cluster.Service.Session == nil {
		w.WriteHeader(http.StatusTooEarly)
		w.Write([]byte(HOST_NOT_FOUND))
		return
	}
	scheme := ":https" //scheme default is https and is empty string
	if r.TLS == nil {
		scheme = ":http"
	}
	requestKey := HOST_PREFIX + r.Host + scheme
	//Check the local cached proxy list
	rp, exists := configuration.Proxies[requestKey]
	if _, found := configuration.ProxyCache.Get(requestKey); found && exists {
		rp.Proxy.ServeHTTP(w, r)
		return
	}

	//Then check the kv-cluster
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(4*time.Second))
	defer cancel()

	//Check for updates/validity
	if destination, err := configuration.Cluster.Service.Session.(*KvService).execute(ctx, &KVAction{Action: GET, Data: &KVData{Key: requestKey}}); err != nil || len(destination.([]byte)) == 0 {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(HOST_NOT_FOUND))
		return
	} else {
		dest := string(destination.([]byte))
		//Proxy
		if !configuration.IgnoreProxyOptions {
			w.Header().Set("Strict-Transport-Security", "max-age=15768000 ; includeSubDomains")
			w.Header().Set("access-control-allow-origin", configuration.AllowOrigin)
		}
		destination, err := url.Parse(string(dest))
		if err != nil {
			rlog.Warningf("Could not route from %s to %s, bad destination\n", r.Host, string(dest))
		}
		if exists {
			//Only update proxy if there are changes
			if destination.Scheme != rp.Route.DestinationScheme || destination.Host != rp.Route.DestinationHost {
				delete(configuration.Proxies, requestKey)
				//TODO: AG, further pruning may be necessary
				rlog.Warningf("Pruning and replacing updated proxy from %s %s to %+v", scheme, r.Host, dest)
			} else {
				configuration.ProxyCache.Set(requestKey, true, cache.DefaultExpiration)
				rp.Proxy.ServeHTTP(w, r)
				return
			}
		}

		director := func(req *http.Request) {
			req.Header.Add("X-Forwarded-For", req.Header.Get("X-Forwarded-For"))
			req.Header.Add("X-Forwarded-Host", req.Host)
			req.Header.Add("X-Origin-Host", destination.Host)
			if configuration.ProxyForceJson {
				req.Header.Set("content-type", "application/json")
			}
			req.URL.Scheme = destination.Scheme
			req.URL.Host = destination.Host

		}
		customTransport := http.DefaultTransport.(*http.Transport).Clone()
		customTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		proxy := &httputil.ReverseProxy{
			Director:   director,
			BufferPool: configuration.ProxySharedBufferPool,
			Transport:  customTransport,
		}
		configuration.Proxies[requestKey] = RooProxy{
			Proxy: proxy,
			Route: &Route{
				OriginScheme:      scheme,
				OriginHost:        r.Host,
				DestinationHost:   destination.Host,
				DestinationScheme: destination.Scheme,
			},
		}
		configuration.ProxyCache.Set(requestKey, true, cache.DefaultExpiration)
		proxy.ServeHTTP(w, r)
	}

}
