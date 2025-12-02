package caddy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"slices"
)

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
	Item       *ConfigItem     `json:"-"`
}

func (client *CaddyClient) Register(ctx context.Context, register bool, configs ConfigItems) error {
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

		cfgId, err := config.GetId()
		if err != nil {
			return err
		}

		if config.RegisterOnly && cfgId != "" {
			return fmt.Errorf("register_only is only allowed for scalar config values")
		}

		if cfgId == "" {
			if register {
				err = client.registerScalar(ctx, &num, config)
			} else {
				err = client.deregisterScalar(ctx, &num, config)
			}
		} else {
			if register {
				err = client.registerAggregate(ctx, &num, config, cfgId)
			} else {
				err = client.deregisterAggregate(ctx, &num, config, cfgId)
			}
		}
		if err != nil {
			return err
		}
	}

	if register {
		log.Printf("caddy: Register %d config snippets", num)
	} else {
		log.Printf("caddy: Deregister %d config snippets", num)
	}

	return nil
}

func (client *CaddyClient) registerScalar(ctx context.Context, num *int, config *ConfigItem) error {
	for {
		res, actualConfig, present, etag, err := client.getConfig(ctx, config.MountPoint)
		if err != nil {
			return err
		}
		if res != nil {
			log.Printf("caddy: GET /id/%s: %v\n", config.MountPoint, res.Status)
		}

		if !present {
			return fmt.Errorf("cannot add scalar configuration to %s, not found", config.MountPoint)
		}

		if !JSONType(actualConfig).Array {
			return fmt.Errorf("scalar values can only be mounted on arrays, attempt to mount to %v on %s", config.MountPoint, string(actualConfig))
		}

		var scalar interface{}
		err = json.Unmarshal(config.Config, &scalar)
		if err != nil {
			return fmt.Errorf("cannot unmarshal scalar config %s, %v", string(config.Config), err)
		}

		var array []interface{}
		err = json.Unmarshal(actualConfig, &array)
		if err != nil {
			return fmt.Errorf("cannot unmarshal actual config at %s, %v", config.MountPoint, err)
		}

		if slices.Contains(array, scalar) {
			return nil // scalar already present
		}

		res, retry, err := client.postConfig(ctx, etag, config.MountPoint, config.Config)
		if res != nil {
			log.Printf("caddy: POST /id/%s (create scalar %s): %v\n", config.MountPoint, string(config.Config), res.Status)
		}
		if err != nil {
			return err
		}
		if retry {
			continue
		}
		*num = *num + 1

		return nil
	}
}

func (client *CaddyClient) deregisterScalar(ctx context.Context, num *int, config *ConfigItem) error {
	for {
		res, actualConfig, present, etag, err := client.getConfig(ctx, config.MountPoint)
		if err != nil {
			return err
		}
		if res != nil {
			log.Printf("caddy: GET /id/%s: %v\n", config.MountPoint, res.Status)
		}

		if !present {
			return fmt.Errorf("cannot add scalar configuration to %s, not found", config.MountPoint)
		}

		if !JSONType(actualConfig).Array {
			return fmt.Errorf("scalar values can only be mounted on arrays, attempt to mount to %v on %s", config.MountPoint, string(actualConfig))
		}

		var scalar interface{}
		err = json.Unmarshal(config.Config, &scalar)
		if err != nil {
			return fmt.Errorf("cannot unmarshal scalar config %s, %v", string(config.Config), err)
		}

		var array []interface{}
		err = json.Unmarshal(actualConfig, &array)
		if err != nil {
			return fmt.Errorf("cannot unmarshal actual config at %s, %v", config.MountPoint, err)
		}

		if !slices.Contains(array, scalar) {
			return nil // value is not present
		}

		array = slices.DeleteFunc(array, func(val interface{}) bool { return val == scalar })

		newConfig, err := json.Marshal(array)
		if err != nil {
			return err
		}

		res, retry, err := client.patchConfig(ctx, etag, config.MountPoint, newConfig)
		if res != nil {
			log.Printf("caddy: PATCH /id/%s (delete scalar %s): %v\n", config.MountPoint, string(config.Config), res.Status)
		}
		if err != nil {
			return err
		}
		if retry {
			continue
		}
		*num = *num + 1

		return nil
	}
}

func (client *CaddyClient) registerAggregate(ctx context.Context, num *int, config *ConfigItem, cfgId string) error {
	for {
		res, _, present, etag, err := client.getConfig(ctx, cfgId)
		if err != nil {
			return err
		}
		if res != nil {
			log.Printf("caddy: GET /id/%s: %v\n", cfgId, res.Status)
		}

		var retry bool
		if present {
			res, retry, err = client.patchConfig(ctx, etag, cfgId, config.Config)
			if res != nil {
				log.Printf("caddy: PATCH /id/%s (replaces %s): %v\n", cfgId, cfgId, res.Status)
			}
		} else {
			res, retry, err = client.postConfig(ctx, etag, config.MountPoint, config.Config)
			if res != nil {
				log.Printf("caddy: POST /id/%s (creates %s): %v\n", config.MountPoint, cfgId, res.Status)
			}
		}
		if err != nil {
			return err
		}
		if retry {
			continue
		}
		*num = *num + 1

		return nil
	}
}

func (client *CaddyClient) deregisterAggregate(ctx context.Context, num *int, config *ConfigItem, cfgId string) error {
	res, _, err := client.deleteConfig(ctx, "", cfgId)
	if res != nil {
		log.Printf("caddy: DELETE /id/%s/ (delete in %s): %v\n", cfgId, config.MountPoint, res.Status)
	}
	if err != nil {
		return err
	}
	*num = *num + 1
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
	if config.IsScalar() {
		return "", nil
	}

	var snip = &ConfigSnip{}
	err := json.Unmarshal(config.Config, snip)
	if err != nil {
		return "", fmt.Errorf("failed to get @id from: %v", string(config.Config))
	} else if snip.Id == "" {
		return "", fmt.Errorf("missing @id in config: %v", string(config.Config))
	}

	return snip.Id, nil
}

func (cfg *ConfigItem) IsScalar() bool {
	return JSONType(cfg.Config).Scalar
}

func JSONType(data json.RawMessage) (result struct {
	Scalar  bool
	True    bool
	False   bool
	Boolean bool
	Null    bool
	String  bool
	Number  bool
	Array   bool
	Object  bool
}) {
	for _, b := range data { // skips whitespace automatically with range
		switch b {
		case ' ', '\t', '\r', '\n':
			continue
		case '{':
			result.Object = true
			return
		case '[':
			result.Array = true
			return
		case 't':
			result.Scalar = true
			result.Boolean = true
			result.True = true
			return
		case 'f':
			result.Scalar = true
			result.Boolean = true
			result.False = true
			return
		case 'n':
			result.Scalar = true
			result.Null = true
			return
		case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			result.Scalar = true
			result.Number = true
			return
		case '"':
			result.Scalar = true
			result.String = true
			return
		default:
			return // invalid as root
		}
	}
	return // empty or only whitespace
}

func (client *CaddyClient) GetConfig(ctx context.Context, config *ConfigItem) (*ConfigStatus, error) {
	result := &ConfigStatus{
		MountPoint: config.MountPoint,
		Id:         config.Id,
		Item:       config,
	}

	var id_url string
	if config.Id == "" && !config.RegisterOnly {
		return result, nil
	} else if config.Id == "" {
		id_url = config.MountPoint
	} else {
		id_url = config.Id
	}

	_, body, present, _, err := client.getConfig(ctx, id_url)
	if err != nil {
		return nil, err
	}

	result.Present = present
	result.Config = body

	if result.Present && config.RegisterOnly && config.Id == "" {
		var arr []json.RawMessage
		err := json.Unmarshal(body, &arr)
		if err != nil {
			return nil, fmt.Errorf("cannot unmarshal actual config in an array for register_only proxy config, %v", err)
		}

		result.Present = false
		for _, item := range arr {
			if bytes.Equal(item, config.Config) {
				result.Present = true
				result.Config = item
			}
		}
	}

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
