package service

import (
	"encoding/json"
	"fmt"
)

type ServiceFunction struct {
	Name              string   `json:"name"`
	ServiceDirectives []string `json:"service_directives,omitempty"`
	Format            string   `json:"format,omitempty"` // Format: cgi, http-stdio
	Exec              []string `json:"exec,omitempty"`
	StderrAsStdout    bool     `json:"stderr_as_stdout,omitempty"`
	ResponseHeaders   []string `json:"response_headers,omitempty"`    // Additional response headers
	NoResponseHeaders bool     `json:"no_response_headers,omitempty"` // Function does not add response headers
	PathInfoStrip     int      `json:"path_info_strip,omitempty"`     // Strip this number of leading elements from PATH_INFO
	Policies          []string `json:"policies,omitempty"`            // Policies to match
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

func (functions *ServiceFunctions) FillDefaults(service *Service) {
	// for _, f := range *functions {
	// }
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
			return fmt.Errorf("unmarshalling function %q, %v", func_name.Name, err)
		}
	}

	return nil
}
