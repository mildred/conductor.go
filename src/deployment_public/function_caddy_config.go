package deployment_public

import (
	"encoding/json"
	"os"
)

type FunctionCaddyConfigMatch struct {
	Path []string `json:"path"`
}

type FunctionCaddyConfigUpstream struct {
	Dial string `json:"dial"`
}

type FunctionCaddyConfigTransport struct {
	Protocol string `json:"protocol"`
}

type FunctionCaddyConfigHandle struct {
	Handler   string                        `json:"handler"`
	Transport FunctionCaddyConfigTransport  `json:"transport"`
	Upstreams []FunctionCaddyConfigUpstream `json:"upstreams"`
}

type FunctionCaddyConfigRoot struct {
	Id     string                      `json:"@id"`
	Match  []FunctionCaddyConfigMatch  `json:"match"`
	Handle []FunctionCaddyConfigHandle `json:"handle"`
}

type FuncFunctionCaddyConfigOpts struct {
	DeploymentName string
	SnippetId      string
	FunctionId     string
	SocketPath     string
}

func (opts *FuncFunctionCaddyConfigOpts) setDefaults() error {
	if opts.DeploymentName == "" {
		opts.DeploymentName = os.Getenv("CONDUCTOR_DEPLOYMENT")
	}
	if opts.SnippetId == "" {
		opts.SnippetId = "conductor-function-" + opts.DeploymentName
	}
	if opts.FunctionId == "" {
		opts.FunctionId = os.Getenv("CONDUCTOR_FUNCTION_ID")
	}
	if opts.SocketPath == "" {
		opts.SocketPath = os.Getenv("CONDUCTOR_FUNCTION_SOCKET")
	}
	return nil
}

func FunctionCaddyConfig(opts FuncFunctionCaddyConfigOpts) error {
	err := opts.setDefaults()
	if err != nil {
		return err
	}

	cfg := FunctionCaddyConfigRoot{
		Id: opts.SnippetId,
		Match: []FunctionCaddyConfigMatch{
			{
				Path: []string{"/cgi/" + opts.FunctionId + "/*"},
			},
		},
		Handle: []FunctionCaddyConfigHandle{
			{
				Handler: "reverse_proxy",
				Transport: FunctionCaddyConfigTransport{
					Protocol: "http",
				},
				Upstreams: []FunctionCaddyConfigUpstream{
					{
						Dial: "unix/" + opts.SocketPath,
					},
				},
			},
		},
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(cfg)
}
