package main

//////////////////////////////////////// Constants
const (
	ENV_ROO_DNS string = "ROO_DNS"

	PONG              string = "pong"
	API_LIMIT_REACHED string = "API Limit Reached"
	HOST_NOT_FOUND    string = "Host Not Found"

	MEMORY_CHECKER    string = "memory"
	SERVICE_TYPE_NATS string = "nats"
	SERVICE_TYPE_KV   string = "kv"
	KV_PORT           string = ":6300"
	API_PORT          string = ":6299"
	NATS_QUEUE_GROUP         = "roo"
	ACME_STAGING      string = "https://acme-staging-v02.api.letsencrypt.org/directory"
	ACME_PRODUCTION   string = "https://acme-v02.api.letsencrypt.org/directory"
)

//Service calls
const (
	SERVE_GET_PING = iota      //1
	SERVE_GET_KV   = 1 << iota //2 etc.
	WRITE_PUT_KV   = 1 << iota

	//CONSUME
	SERVE_GET_PING_DESC = "getPing"
	SERVE_GET_KV_DESC   = "getKV"

	//NOTIFY ONLY
	WRITE_PUT_KV_DESC = "putKV"
)
