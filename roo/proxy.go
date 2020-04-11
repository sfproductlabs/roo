package main

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

type Host url.URL

var sharedBuffer httputil.BufferPool = newBufferPool()

func Proxy(w *http.ResponseWriter, r *http.Request, configuration *Configuration) {

	writer := *w
	//Proxy
	writer.Header().Set("Strict-Transport-Security", "max-age=15768000 ; includeSubDomains")
	writer.Header().Set("access-control-allow-origin", configuration.AllowOrigin)
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
	proxy.ServeHTTP(writer, r)
	//or error
	//w.WriteHeader(http.StatusTooManyRequests)
	//w.Write([]byte(API_LIMIT_REACHED))
	//return
}
