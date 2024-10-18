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

type ConfigItem struct {
	Id           string          `json:"id"`
	Config       json.RawMessage `json:"config"`
	RegisterOnly bool            `json:"register_only"`
}

func NewClient(endpoint string) (*CaddyClient, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	return &CaddyClient{u}, nil
}

func (client *CaddyClient) Register(register bool, configs []ConfigItem) error {
	if !register {
		slices.Reverse(configs)
	}

	var num = 0

	for _, config := range configs {
		if register {
			log.Printf("caddy: Register configuration for %s", config.Id)
		} else {
			log.Printf("caddy: Deregister configuration for %s", config.Id)
		}
		var cfg ConfigSnip

		err := json.Unmarshal(config.Config, &cfg)
		if err != nil {
			if config.RegisterOnly {
				log.Printf("caddy: Failed to detect @id property on JSON: %v", err)
			} else {
				return err
			}
		}

		if !config.RegisterOnly && cfg.Id == "" {
			return fmt.Errorf("Cannot detect @id for Caddy configuration")
		}

		var config_id string
		if register {
			config_id = config.Id
		} else if !config.RegisterOnly {
			config_id = cfg.Id + "/"
		}

		url, err := client.endpoint.Parse("/id/" + config_id)
		if err != nil {
			return err
		}

		var res *http.Response
		if register {
			res, err = http.Post(url.String(), "application/json", bytes.NewBuffer(config.Config))
			if err != nil {
				return err
			}
			log.Printf("caddy: POST /id/%s (create %q): %s\n", config_id, cfg.Id, res.Status)
			num = num + 1
		} else if !config.RegisterOnly {
			req, err := http.NewRequest(http.MethodDelete, url.String(), nil)
			if err != nil {
				return err
			}

			res, err = http.DefaultClient.Do(req)
			if err != nil {
				return err
			}
			log.Printf("caddy: DELETE /id/%s (delete in %s): %s\n", config_id, config.Id, res.Status)
			num = num + 1
		} else {
			log.Printf("caddy: Do not deregister %q in %s (register_only)\n", cfg.Id, config.Id)
			continue
		}

		if res.StatusCode < 200 || res.StatusCode >= 300 {
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
