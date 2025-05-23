package caddy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"slices"
)

type CaddyClient struct {
	endpoint *url.URL
}

type ConfigSnip struct {
	Id string `json:"@id"`
}

type CaddyHandle struct {
	Handler   string            `json:"handler"`
	Upstreams []json.RawMessage `json:"upstreams"`
}

type CaddyRoute struct {
	Match  []json.RawMessage `json:"match"`
	Handle []CaddyHandle     `json:"handle"`
}

type ConfigItem struct {
	DeprecatedId string          `json:"id"`
	MountPoint   string          `json:"mount_point"`
	Config       json.RawMessage `json:"config"`
	RegisterOnly bool            `json:"register_only"`
	Id           string          `json:"-"`
}

type ConfigItems []*ConfigItem

type ConfigStatus struct {
	MountPoint string          `json:"parent_id"`
	Id         string          `json:"id"`
	Present    bool            `json:"present"`
	Config     json.RawMessage `json:"config"`
}

func NewClient(endpoint string) (*CaddyClient, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	return &CaddyClient{u}, nil
}

func (client *CaddyClient) Register(register bool, configs ConfigItems) error {
	if !register {
		slices.Reverse(configs)
	}

	var num = 0

	for _, config := range configs {
		if register {
			log.Printf("caddy: Register configuration for %s in %s", config.Id, config.MountPoint)
		} else {
			log.Printf("caddy: Deregister configuration for %s in %s", config.Id, config.MountPoint)
		}
		var cfg ConfigSnip

		err := json.Unmarshal(config.Config, &cfg)
		if err != nil {
			if config.RegisterOnly {
				log.Printf("caddy: Failed to detect @id property on JSON: %v", err)
			} else {
				log.Printf("Failed to get @id from: %v", string(config.Config))
				return err
			}
		}

		if !config.RegisterOnly && cfg.Id == "" {
			return fmt.Errorf("Cannot detect @id for Caddy configuration")
		}

		var config_id string
		if register {
			config_id = config.MountPoint
		} else if !config.RegisterOnly {
			config_id = cfg.Id + "/"
		}

		url, err := client.endpoint.Parse("/id/" + config_id)
		if err != nil {
			return err
		}

		var res *http.Response
		var code_valid bool
		if register {
			res, err = http.Post(url.String(), "application/json", bytes.NewBuffer(config.Config))
			if err != nil {
				return err
			}
			log.Printf("caddy: POST /id/%s (create %q): %s\n", config_id, cfg.Id, res.Status)
			num = num + 1

			code_valid = res.StatusCode >= 200 && res.StatusCode < 300
		} else if !config.RegisterOnly {
			req, err := http.NewRequest(http.MethodDelete, url.String(), nil)
			if err != nil {
				return err
			}

			res, err = http.DefaultClient.Do(req)
			if err != nil {
				return err
			}
			log.Printf("caddy: DELETE /id/%s (delete in %s): %s\n", config_id, config.MountPoint, res.Status)
			num = num + 1

			code_valid = res.StatusCode >= 200 && res.StatusCode < 300 || res.StatusCode == 404
		} else {
			log.Printf("caddy: Do not deregister %q in %s (register_only)\n", cfg.Id, config.MountPoint)
			continue
		}

		if !code_valid {
			body, err := io.ReadAll(res.Body)
			if err != nil {
				return err
			}

			return fmt.Errorf("cannot update Caddy config; HTTP error %s: %s", res.Status, string(body))
		}
	}

	if register {
		log.Printf("caddy: Register %d config snippets", num)
	} else {
		log.Printf("caddy: Deregister %d config snippets", num)
	}

	return nil
}

func (configs ConfigItems) SetDefaults() error {
	for _, config := range configs {
		if config.DeprecatedId != "" && config.MountPoint == "" {
			config.MountPoint = config.DeprecatedId
		}
		config.DeprecatedId = ""

		var err error
		config.Id, err = config.GetId()
		if err != nil {
			return err
		}
	}

	return nil
}

func (config *ConfigItem) GetId() (string, error) {
	var snip = &ConfigSnip{}
	err := json.Unmarshal(config.Config, snip)
	if err != nil {
		return "", err
	}

	return snip.Id, nil
}

func (client *CaddyClient) GetConfig(config *ConfigItem) (*ConfigStatus, error) {
	result := &ConfigStatus{
		MountPoint: config.MountPoint,
		Id:         config.Id,
	}

	url, err := client.endpoint.Parse("/id/" + config.Id)
	if err != nil {
		return nil, err
	}

	res, err := http.Get(url.String())
	if err != nil {
		return nil, err
	}
	log.Printf("caddy: GET /id/%s: %s\n", config.Id, res.Status)

	result.Present = res.StatusCode >= 200 && res.StatusCode < 300

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	result.Config = body

	return result, nil
}

func getDial(upstream_json json.RawMessage) (string, error) {
	var upstream struct {
		Dial string `json:"dial"`
	}
	err := json.Unmarshal(upstream_json, &upstream)
	if err != nil {
		return "", err
	}

	return upstream.Dial, nil
}

func (cfg *ConfigStatus) Dial() (string, error) {
	return getDial(cfg.Config)
}

func (cfg *ConfigStatus) MatchConfig() ([]json.RawMessage, error) {
	var route CaddyRoute
	err := json.Unmarshal(cfg.Config, &route)
	if err != nil {
		return nil, err
	}

	return route.Match, nil
}

func (cfg *ConfigStatus) Upstreams() ([]json.RawMessage, error) {
	var upstreams []json.RawMessage
	var route CaddyRoute

	err := json.Unmarshal(cfg.Config, &route)
	if err != nil {
		return nil, err
	}

	for _, handle := range route.Handle {

		if handle.Handler == "reverse_proxy" {
			upstreams = append(upstreams, handle.Upstreams...)
		}
	}

	return upstreams, nil
}

func (cfg *ConfigStatus) UpstreamDials() ([]string, error) {
	var res []string
	upstreams, err := cfg.Upstreams()
	if err != nil {
		return nil, err
	}

	for _, up := range upstreams {
		dial, err := getDial(up)
		if err != nil {
			return nil, err
		}
		res = append(res, dial)
	}
	return res, nil
}
