package main

//////////////////////////////////////// Constants
const (
	PONG              string = "pong"
	API_LIMIT_REACHED string = "API Limit Reached"

	SERVICE_TYPE_NATS string = "nats"
	SERVICE_TYPE_KV   string = "kv"
	KV_PORT           string = ":6300"
	API_PORT          string = ":6299"
	NATS_QUEUE_GROUP         = "roo"
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
