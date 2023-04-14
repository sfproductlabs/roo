package main

import "github.com/lni/dragonboat/v4/logger"

// ////////////////////////////////////// Constants
const (
	ENV_ROO_DNS          string = "ROO_DNS"
	ENV_ROO_RESOLVER     string = "ROO_RESOLVER"
	ENV_ROO_START_DELAY  string = "ROO_START_DELAY"
	ENV_ROO_ACME_STAGING string = "ROO_ACME_STAGING"
	ENV_ROO_SWARM        string = "ROO_SWARM"

	BOOTSTRAP_DELAY_MS int    = 7000 //In milliseconds
	BOOTSTRAP_WAIT_S   int    = 60
	PONG               string = "pong"
	API_LIMIT_REACHED  string = "API Limit Reached"
	HOST_NOT_FOUND     string = "Host Not Found"

	MEMORY_CHECKER    string = "memory"
	SERVICE_TYPE_NATS string = "nats"
	SERVICE_TYPE_KV   string = "kv"
	KV_PORT           string = ":6300"
	API_PORT          string = ":6299"
	NATS_QUEUE_GROUP         = "roo"
	ACME_STAGING      string = "https://acme-staging-v02.api.letsencrypt.org/directory"
	ACME_PRODUCTION   string = "https://acme-v02.api.letsencrypt.org/directory"

	CACHE_PREFIX         = "com.roo.cache:"
	HOST_PREFIX          = "com.roo.host:"
	CLUSTER_PREFIX       = "com.roo.cluster:"
	PEER_PREFIX          = "com.roo.peer:"
	SWARM_MANAGER_PREFIX = "com.roo.swarm.manager:"
	SWARM_WORKER_PREFIX  = "com.roo.swarm.worker:"
	ROO_STARTED          = "com.roo.started"

	GROUP_POSTFIX = ":group"
	NODE_POSTFIX  = ":node"
	ROLE_POSTFIX  = ":role"
)

// Service calls
const (
	SERVE_GET_PING    = iota + 1
	SERVE_GET_KV      = iota
	SERVE_PUT_KV      = iota
	SERVE_GET_KVS     = iota
	SERVE_POST_JOIN   = iota
	SERVE_POST_SWARM  = iota
	SERVE_POST_REMOVE = iota
	SERVE_POST_RESCUE = iota
	SERVE_POST_PERM   = iota
	SERVE_PUT_PERM    = iota
)

var (
	rlog = logger.GetLogger("roo")
)

const (
	PUT    string = "PUT"
	GET    string = "GET"
	DELETE string = "DELETE"
	UPDATE string = "UPDATE"
	SCAN   string = "SCAN"
	JOIN   string = "JOIN"
	LEAVE  string = "LEAVE"
	RESCUE string = "RESCUE"
)

const (
	appliedIndexKey    string = "disk_kv_applied_index"
	testDBDirName      string = "cluster-data"
	currentDBFilename  string = "current"
	updatingDBFilename string = "current.updating"
)
