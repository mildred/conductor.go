package deployment_public

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/mildred/conductor.go/src/dirs"
)

type FuncFunctionCaddyConfigOpts struct {
	DeploymentName string
	SnippetId      string
	FunctionId     string
	SocketPath     string
	Policies       []string
}

func (opts *FuncFunctionCaddyConfigOpts) setDefaults() error {
	if opts.DeploymentName == "" {
		opts.DeploymentName = os.Getenv("CONDUCTOR_DEPLOYMENT")
	}
	if opts.FunctionId == "" {
		opts.FunctionId = os.Getenv("CONDUCTOR_FUNCTION_ID")
	}
	if opts.SnippetId == "" {
		opts.SnippetId = "conductor-function." + opts.DeploymentName + "." + opts.FunctionId
	}
	if opts.SocketPath == "" {
		opts.SocketPath = os.Getenv("CONDUCTOR_FUNCTION_SOCKET")
	}
	if opts.Policies == nil {
		opts.Policies = strings.Split(os.Getenv("CONDUCTOR_FUNCTION_POLICIES"), " ")
	}
	return nil
}

func FunctionCaddyConfig(opts FuncFunctionCaddyConfigOpts) error {
	err := opts.setDefaults()
	if err != nil {
		return err
	}

	var handlers []interface{}

	if len(opts.Policies) > 0 {
		handlers = append(handlers, map[string]interface{}{
			"handler": "reverse_proxy",
			"transport": map[string]interface{}{
				"protocol": "http",
			},
			"upstreams": []interface{}{
				map[string]interface{}{
					"dial": "unix/" + dirs.Join(dirs.RuntimeDir, "conductor-policy.socket"),
				},
			},
			"rewrite": map[string]interface{}{
				"method": "HEAD",
			},
			"headers": map[string]interface{}{
				"request": map[string]interface{}{
					"set": map[string]interface{}{
						"Conductor-Policy":   opts.Policies,
						"X-Forwarded-Method": []string{"{http.request.method}"},
						"X-Forwarded-Uri":    []string{"{http.request.uri}"},
					},
				},
			},
			"handle_response": []interface{}{
				// When a response handler is invoked, the response from the backend is
				// not written to the client, and the configured handle_response route
				// will be executed instead, and it is up to that route to write a
				// response. If the route does not write a response, then request
				// handling will continue with any handlers that are ordered after this
				// reverse_proxy.
				//
				// - any handle_response matching: the request can continue down the
				//   line of handlers, unless the response handler writes a HTTP
				//   response
				// - no handle_response matching: the response from the auth upstream
				//   is sent directly
				map[string]interface{}{
					"match": map[string]interface{}{
						"status_code": []interface{}{2},
					},
					"routes": []interface{}{
						map[string]interface{}{
							"handle": []interface{}{
								map[string]interface{}{
									"handler": "headers",
									"request": map[string]interface{}{
										"set": map[string]interface{}{
											"Conductor-Policy-Pass": []string{"1"},
										},
									},
								},
							},
						},
					},
				},
			},
		})

	}

	handlers = append(handlers, map[string]interface{}{
		"handler": "reverse_proxy",
		"transport": map[string]interface{}{
			"protocol": "http",
		},
		"upstreams": []interface{}{
			map[string]interface{}{
				"dial": "unix/" + opts.SocketPath,
			},
		},
	})

	cfg := map[string]interface{}{
		"@id": opts.SnippetId,
		"match": []interface{}{
			map[string]interface{}{
				"path": []string{"/cgi/" + opts.FunctionId + "/*"},
			},
		},
		"handle": handlers,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(cfg)
}
