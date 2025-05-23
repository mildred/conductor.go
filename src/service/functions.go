package service

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/mildred/conductor.go/src/dirs"
)

type ServiceFunction struct {
	Name                string                        `json:"name"`
	ServiceDirectives   []string                      `json:"service_directives,omitempty"`
	Format              string                        `json:"format,omitempty"` // Format: cgi, http-stdio
	Exec                []string                      `json:"exec,omitempty"`
	StderrAsStdout      bool                          `json:"stderr_as_stdout,omitempty"`
	ResponseHeaders     []string                      `json:"response_headers,omitempty"`    // Additional response headers
	NoResponseHeaders   bool                          `json:"no_response_headers,omitempty"` // Function does not add response headers
	PathInfoStrip       int                           `json:"path_info_strip,omitempty"`     // Strip this number of leading elements from PATH_INFO
	Policies            []string                      `json:"policies,omitempty"`            // Policies to match
	ReverseProxy        []*ServiceFunctionProxyConfig `json:"reverse_proxy"`
	DefaultReverseProxy *bool                         `json:"default_reverse_proxy,omitempty"`
}

type ServiceFunctionProxyConfig struct { // TODO
	Name          string          `json:"name"`
	MountPoint    string          `json:"mount_point,omitempty"`
	Route         json.RawMessage `json:"route,omitempty"` // if unspecified, generate default, if false, do not generate Route
	UpstreamsPath string          `json:"upstreams_path"`
}

type ServiceFunctions []*ServiceFunction

func (functions *ServiceFunctions) FindFunction(name string) *ServiceFunction {
	for _, f := range *functions {
		if f.Name == name {
			return f
		}
	}
	return nil
}

func (functions *ServiceFunctions) FindMainFunction() *ServiceFunction {
	if len(*functions) == 1 {
		return (*functions)[0]
	} else {
		return functions.FindFunction("")
	}
}

func (functions *ServiceFunctions) FixPaths(dir string) error {
	for _, f := range *functions {
		if len(f.Exec) > 0 {
			if err := fix_path(dir, &f.Exec[0], false); err != nil {
				return err
			}
		}
	}
	return nil
}

func (functions *ServiceFunctions) FillDefaults(service *Service) error {
	for _, f := range *functions {

		if (f.DefaultReverseProxy == nil && len(f.ReverseProxy) == 0) || *f.DefaultReverseProxy {
			name := "default"
			route, err := f.CaddyConfig(service, name)
			if err != nil {
				return err
			}
			f.ReverseProxy = append(f.ReverseProxy, &ServiceFunctionProxyConfig{
				Name:          "default",
				MountPoint:    "conductor-server/routes",
				Route:         route,
				UpstreamsPath: f.CaddyConfigName(service, name) + ".handler/upstreams",
			})
		}

		for i, proxy := range f.ReverseProxy {
			if proxy.Name == "" {
				proxy.Name = fmt.Sprintf("%d", i)
			}

			if proxy.MountPoint == "" {
				proxy.MountPoint = "conductor-server/routes"
			}
			if len(proxy.Route) == 0 {
				var err error
				proxy.Route, err = f.CaddyConfig(service, proxy.Name)
				if err != nil {
					return err
				}
			} else {
				err := caddyConfigSetId(&proxy.Route, f.CaddyConfigName(service, proxy.Name))
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func caddyConfigSetId(route *json.RawMessage, new_id string) error {
	var props = map[string]interface{}{}
	err := json.Unmarshal(*route, &props)
	if err != nil {
		return err
	}

	if id, has_id := props["@id"]; !has_id || id.(string) == "" {
		props["@id"] = new_id
		*route, err = json.Marshal(props)
		if err != nil {
			return err
		}
	}

	return nil
}

func (f *ServiceFunction) CaddyConfigName(service *Service, name string) string {
	return fmt.Sprintf("conductor-function.%s.%s.%s.%s", service.AppName, service.InstanceName, f.Name, name)
}

func (f *ServiceFunction) CaddyConfig(service *Service, name string) (json.RawMessage, error) {
	config_id := f.CaddyConfigName(service, name)
	part_id, err := service.PartId(f.Name)
	if err != nil {
		return nil, err
	}

	var handlers []interface{}

	if len(f.Policies) > 0 {
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
						"Conductor-Policy":   strings.Join(f.Policies, " "),
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
		"@id":     config_id + ".handler",
		"handler": "reverse_proxy",
		"transport": map[string]interface{}{
			"protocol": "http",
		},
		"upstreams": []interface{}{
			// map[string]interface{}{
			// 	"dial": "unix/" + opts.SocketPath,
			// },
		},
	})

	return json.Marshal(map[string]interface{}{
		"@id": config_id,
		"match": []interface{}{
			map[string]interface{}{
				"path": []string{"/cgi/" + part_id + "/*"},
			},
		},
		"handle": handlers,
	})
}

func (functions *ServiceFunctions) UnmarshalJSON(data []byte) error {
	var raw_functions []json.RawMessage
	err := json.Unmarshal(data, &raw_functions)
	if err != nil {
		return fmt.Errorf("unmarshalling functions, %v", err)
	}

	for _, raw_func := range raw_functions {
		var func_name struct {
			Name string `json:"name"`
		}
		err = json.Unmarshal(raw_func, &func_name)
		if err != nil {
			return fmt.Errorf("unmarshalling function for name, %v", err)
		}
		var existing_func *ServiceFunction = functions.FindFunction(func_name.Name)
		if existing_func == nil {
			existing_func = &ServiceFunction{}
			*functions = append(*functions, existing_func)
		}
		err = json.Unmarshal(raw_func, existing_func)
		if err != nil {
			log.Printf("Failed to parse JSON: %v\n", string(raw_func))
			return fmt.Errorf("unmarshalling function %q, %v", func_name.Name, err)
		}
	}

	return nil
}
