package main

import (
	"net/http"
	"time"

	"github.com/gocql/gocql"
	"github.com/lni/dragonboat/v3"
	"github.com/nats-io/nats.go"
)

////////////////////////////////////////
// Get the system setup from the config.json file:
////////////////////////////////////////
type session interface {
	connect() error
	close() error
	write(w *WriteArgs) error
	listen() error
	serve(w *http.ResponseWriter, r *http.Request, s *ServiceArgs) error
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

	Session session
}

type Cluster struct {
	Service *Service
	DNS     string
	Binding string
	Group   uint64
	NodeID  uint64
}

type KvService struct { //Implements 'session'
	Configuration *Service
	nh            *dragonboat.NodeHost
	AppConfig     *Configuration
}

type NatsService struct { //Implements 'session'
	Configuration *Service
	nc            *nats.Conn
	ec            *nats.EncodedConn
	AppConfig     *Configuration
}

type Configuration struct {
	Domains                  []string //Domains in Trust, LetsEncrypt domains
	StaticDirectory          string   //Static FS Directory (./public/)
	UseLocalTLS              bool
	IgnoreInsecureTLS        bool
	Cluster                  Cluster
	ClusterDNS               string
	Notify                   []Service
	Consume                  []Service
	API                      Service
	PrefixPrivateHash        string
	ProxyUrl                 string
	ProxyUrlFilter           string
	IgnoreProxyOptions       bool
	ProxyForceJson           bool
	ProxyPort                string
	ProxyPortTLS             string
	ProxyPortRedirect        string
	ProxyDailyLimit          uint64
	ProxyDailyLimitChecker   string //Service, Ex. casssandra
	ProxyDailyLimitCheck     func(string) uint64
	SchemaVersion            int
	ApiVersion               int
	Debug                    bool
	UrlFilter                string
	UrlFilterMatchGroup      int
	AllowOrigin              string
	IsUrlFiltered            bool
	MaximumConnections       int
	ReadTimeoutSeconds       int
	ReadHeaderTimeoutSeconds int
	WriteTimeoutSeconds      int
	IdleTimeoutSeconds       int
	MaxHeaderBytes           int
	DefaultRedirect          string
	IgnoreQueryParamsKey     string
	AccountHashMixer         string
}
