package main

//////////////////////////////////////// Constants
const (
	PONG              string = "pong"
	API_LIMIT_REACHED string = "API Limit Reached"

	SERVICE_TYPE_NATS string = "nats"
	SERVICE_TYPE_KV   string = "kv"

	NATS_QUEUE_GROUP = "roo"
)

//Service calls
const (
	//CONSUME
	SERVE_GET_PING      = 1 << iota
	SERVE_GET_PING_DESC = "getPing"
	SERVE_GET_KV        = 1 << iota
	SERVE_GET_KV_DESC   = "getKV"
)
const (
	//NOTIFY ONLY
	WRITE_PUT_KV      = 1 << iota
	WRITE_PUT_KV_DESC = "putKV"
)
