package service

type ServiceDaemonNotSocketActivated struct {
}

type ServiceDaemonSocketActivated struct {
	SocketDirectives []string          `json:"socket_directives,omitempty"`
	SocketActivated  bool              `json:"socket_activated,omitempty"`
	SocketProxy      SocketProxyConfig `json:"socket_proxy,omitempty"`
}

type SocketProxyConfig struct {
	Target             string   `json:"target"`
	WaitConnect        bool     `json:"wait_connect,omitempty"`
	WaitConnectTimeout string   `json:"wait_connect_timeout,omitempty"`
	ConnectionsMax     *int     `json:"connections_max,omitempty"`
	ExitIdleTime       string   `json:"exit_idle_time,omitempty"`
	ProxyProtocol      string   `json:"proxy_protocol,omitempty"`
	Args               []string `json:"args,omitempty"`
}

func (part *ServiceDaemonSocketActivated) IsSocketActivated() bool {
	return part.SocketActivated
}

func (part *ServiceDaemonNotSocketActivated) IsSocketActivated() bool {
	return false
}
