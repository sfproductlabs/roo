package main

import (
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/gocql/gocql"
	"github.com/lni/dragonboat/v4"
	"github.com/lni/goutils/syncutil"
	"github.com/nats-io/nats.go"
	"github.com/patrickmn/go-cache"
)

// //////////////////////////////////////
// Get the system setup from the config.json file:
// //////////////////////////////////////
type session interface {
	connect() error                                                      //Connect/open the service session
	close() error                                                        //Close the service session
	write(w *WriteArgs) error                                            //Writes is for one way outbound - require no feedback and are ultra fast
	listen() error                                                       //Listen is for one way inbound
	serve(w *http.ResponseWriter, r *http.Request, s *ServiceArgs) error //Serve is for bidirectional comms
}

type KVData struct {
	Key string
	Val []byte
}

type KVBatch struct {
	Action string
	Batch  []*KVData
}

type KVAction struct {
	Action string
	Data   *KVData
}

type KeyValue struct {
	Key   string
	Value interface{}
}

type Field struct {
	Type    string
	Id      string
	Default string
}

type Query struct {
	Statement string
	QueryType string
	Fields    []Field
}

type Filter struct {
	Type    string
	Alias   string
	Id      string
	Queries []Query
}

type WriteArgs struct {
	WriteType int
	Values    *map[string]interface{}
	IsServer  bool
	IP        string
	Browser   string
	Language  string
	URI       string
	Host      string
	EventID   gocql.UUID
}

type ServiceArgs struct {
	ServiceType int
	Values      *map[string]string
	IsServer    bool
	IP          string
	Browser     string
	Language    string
	URI         string
	EventID     gocql.UUID
}

type Service struct {
	Service  string
	Hosts    []string
	CACert   string
	Cert     string
	Key      string
	Secure   bool
	Critical bool

	Context      string
	Filter       []Filter
	Retry        bool
	Format       string
	MessageLimit int
	ByteLimit    int
	Timeout      time.Duration
	Connections  int

	Consumer  bool
	Ephemeral bool
	Note      string

	Session      session
	Instantiated int64
	Started      int64
}

type Cluster struct {
	Service           *Service
	DNS               string
	Resolver          string
	BootstrapHosts    bool
	Binding           string
	ReplicaID         uint64
	ShardID           uint64
	StartDelaySeconds int64
}

type KvService struct { //Implements 'session'
	Configuration *Service
	nh            *dragonboat.NodeHost
	swarmUpdater  *syncutil.Stopper
	raftWatcher   *syncutil.Stopper
	AppConfig     *Configuration
}

type NatsService struct { //Implements 'session'
	Configuration *Service
	nc            *nats.Conn
	ec            *nats.EncodedConn
	AppConfig     *Configuration
}

type Configuration struct {
	SchemaVersion            int
	ApiVersion               int
	ApiVersionString         string
	DataDirectoryRoot        string
	AcmeStaging              bool
	Tests                    []string
	Swarm                    bool
	SwarmRefreshSeconds      int
	SwarmRole                string
	CheckHostnames           bool
	HostCache                *cache.Cache
	Cluster                  Cluster
	ClusterDNS               string
	Notify                   []Service
	Consume                  []Service
	API                      *Service
	IgnoreInsecureTLS        bool
	ProxyUrlFilter           string
	IgnoreProxyOptions       bool
	PermissonCheckExact      bool
	ProxyForceJson           bool
	ProxyDailyLimit          uint64
	ProxyDailyLimitChecker   string //Service, Ex. casssandra
	ProxyDailyLimitCheck     func(string) uint64
	ProxyCache               *cache.Cache
	Proxies                  map[string]RooProxy
	ProxySharedBufferPool    httputil.BufferPool
	Debug                    bool
	UrlFilter                string
	UrlFilterMatchGroup      int
	AllowOrigin              string
	LogDB                    string
	IsUrlFiltered            bool
	MaximumConnections       int
	ReadTimeoutSeconds       int
	CacheSeconds             int
	ReadHeaderTimeoutSeconds int
	WriteTimeoutSeconds      int
	IdleTimeoutSeconds       int
	MaxHeaderBytes           int
	DefaultRedirect          string
	IgnoreQueryParamsKey     string
	AccountHashMixer         string
}

type ClusterStatus struct {
	Client       string
	Binding      string
	Conns        int
	ReplicaID    uint64
	ShardID      uint64
	Hosts        []string
	Instantiated int64
	Started      int64
}

type Route struct {
	OriginScheme      string
	OriginHost        string
	OriginPort        string
	DestinationScheme string
	DestinationHost   string
	DestinationPort   string
}

type RooProxy struct {
	Route *Route
	Proxy *httputil.ReverseProxy
}
