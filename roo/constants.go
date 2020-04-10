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
)
const (
	//NOTIFY ONLY
	WRITE_JS      = 1 << iota
	WRITE_JS_DESC = "putJS"
)

const (
	appliedIndexKey    string = "disk_kv_applied_index"
	testDBDirName      string = "example-data"
	currentDBFilename  string = "current"
	updatingDBFilename string = "current.updating"
)
