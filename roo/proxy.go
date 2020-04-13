package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/patrickmn/go-cache"
)

func Proxy(writer *http.ResponseWriter, r *http.Request, configuration *Configuration) {
	w := *writer

	//Check the local cached proxy list
	if p, found := configuration.ProxyCache.Get(r.Host); found {
		p.(*httputil.ReverseProxy).ServeHTTP(w, r)
		return
	}
	//Then check the kv-cluster
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(3*time.Second))
	defer cancel()
	if dest, err := configuration.Cluster.Service.Session.(*KvService).get(ctx, r.Host, "com.roo.domain:"); err != nil || dest == nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(HOST_NOT_FOUND))
	} else {
		//Proxy
		w.Header().Set("Strict-Transport-Security", "max-age=15768000 ; includeSubDomains")
		w.Header().Set("access-control-allow-origin", configuration.AllowOrigin)
		destination, err := url.Parse(string(dest))
		if err != nil {
			fmt.Println("Could not route from %s to %s, bad destination", r.Host, string(dest))
		}
		director := func(req *http.Request) {
			req.Header.Add("X-Forwarded-Host", req.Host)
			req.Header.Add("X-Origin-Host", destination.Host)
			if configuration.ProxyForceJson {
				req.Header.Set("content-type", "application/json")
			}
			req.URL.Scheme = destination.Scheme
			req.URL.Host = destination.Host
		}
		proxy := &httputil.ReverseProxy{Director: director, BufferPool: configuration.ProxySharedBufferPool}
		configuration.ProxyCache.Set(r.Host, proxy, cache.DefaultExpiration)
		proxy.ServeHTTP(w, r)
	}

}
