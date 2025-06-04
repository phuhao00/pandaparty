package config

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
)

type RedisConfig struct {
	Addr          string   `yaml:"addr"` // Used for single node or as one of sentinel's addrs (though sentinel_addrs is preferred for sentinels)
	Password      string   `yaml:"password,omitempty"`
	DB            int      `yaml:"db,omitempty"`
	MasterName    string   `yaml:"master_name,omitempty"`    // For Sentinel
	SentinelAddrs []string `yaml:"sentinel_addrs,omitempty"` // For Sentinel: list of "host:port"
}

type MongoConfig struct {
	URI              string   `yaml:"uri"`             // Primary connection string, can contain all options
	Hosts            []string `yaml:"hosts,omitempty"` // Alternative: list of "host:port" for mongos or replica set members
	ReplicaSet       string   `yaml:"replica_set,omitempty"`
	Username         string   `yaml:"username,omitempty"`
	Password         string   `yaml:"password,omitempty"`    // Consider using a more secure way to handle passwords in real deployments
	AuthSource       string   `yaml:"auth_source,omitempty"` // e.g., "admin" or the database name
	Database         string   `yaml:"database"`              // The default database to use
	Collection       string   `yaml:"collection"`            // Default collection (current design of NewMongoClient uses this)
	ConnectTimeoutMS int64    `yaml:"connect_timeout_ms,omitempty"`
	MaxPoolSize      uint64   `yaml:"max_pool_size,omitempty"`
}

type ConsulConfig struct {
	Addr string `yaml:"addr"`
}

type NSQConfig struct {
	NSQDAddr                string   `yaml:"nsqd_addr,omitempty"`                 // Kept for single-node setup or fallback
	NSQDAddresses           []string `yaml:"nsqd_addresses,omitempty"`            // For producer to connect to a list of nsqd instances
	NSQLookupdHTTPAddresses []string `yaml:"nsqlookupd_http_addresses,omitempty"` // For consumers and optionally for producers to discover nsqds
	Topic                   string   `yaml:"topic,omitempty"`                     // Default topic
	Channel                 string   `yaml:"channel,omitempty"`                   // Default channel for consumers
}

type ServerConfig struct {
	Redis  RedisConfig  `yaml:"redis"`
	Mongo  MongoConfig  `yaml:"mongo"`
	Consul ConsulConfig `yaml:"consul"`
	NSQ    NSQConfig    `yaml:"nsq"`
	Server ServerInfo   `yaml:"server"` // Added ServerInfo for host, port, rpcport
	Friend FriendConfig `yaml:"friend"`
}

// ServerInfo holds basic server address information
type ServerInfo struct {
	Host string `yaml:"host"`
	// Port                int            `yaml:"port"` // Deprecate this in favor of specific ports below
	LoginServerHTTPPort      int            `yaml:"loginserver_http_port,omitempty"`
	GMServerHTTPPort         int            `yaml:"gmserver_http_port,omitempty"`
	GatewayGameServerTCPPort int            `yaml:"gatewayserver_game_tcp_port,omitempty"` // Assuming TCP for now
	GatewayRoomServerTCPPort int            `yaml:"gatewayserver_room_tcp_port,omitempty"` // Assuming TCP for now
	GameServerTCPPort        int            `yaml:"gameserver_tcp_port,omitempty"`         // Assuming TCP for now
	ServiceRpcPorts          map[string]int `yaml:"servicerpcports"`                       // For internal RPC communication, service_name -> port
	RegisterSelfAsHost       bool           `yaml:"register_self_as_host,omitempty"`       // If true, server registers its own name as host with Consul
}

var (
	serverConfigInstance *ServerConfig
)

func GetServerConfig() *ServerConfig {
	if serverConfigInstance == nil {
		var err error
		serverConfigInstance, err = loadConfig("config/server.yaml")
		if err != nil {
			panic(fmt.Sprintf("Failed to load server config: %v", err))
		}
	}
	return serverConfigInstance
}

func loadConfig(path string) (*ServerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err) // Added more context to error
	}

	var cfg ServerConfig
	if err = yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config data from %s: %w", path, err) // Added more context to error
	}

	return &cfg, nil
}
