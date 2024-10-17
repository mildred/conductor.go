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
	Id     string          `json:"id"`
	Config json.RawMessage `json:"config"`
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

	for _, config := range configs {
		var cfg ConfigSnip
		err := json.Unmarshal(config.Config, &cfg)
		if err != nil {
			return err
		}

		if cfg.Id == "" {
			return fmt.Errorf("Cannot detect @id for Caddy configuration")
		}

		var config_id string
		if register {
			config_id = config.Id
		} else {
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
		} else {
			req, err := http.NewRequest(http.MethodDelete, url.String(), nil)
			if err != nil {
				return err
			}

			res, err = http.DefaultClient.Do(req)
			if err != nil {
				return err
			}
			log.Printf("caddy: DELETE /id/%s (delete in %s): %s\n", config_id, config.Id, res.Status)
		}

		if res.StatusCode < 200 || res.StatusCode >= 300 {
			body, err := io.ReadAll(res.Body)
			if err != nil {
				return err
			}

			return fmt.Errorf("cannot update Caddy config; HTTP error %s: %s", res.Status, string(body))
		}
	}
	return nil
}
