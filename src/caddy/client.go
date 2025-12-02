package caddy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

var DefaultTimeout time.Duration = 10 * time.Second

type CaddyClient struct {
	endpoint *url.URL
	timeout  time.Duration
}

func NewClient(endpoint string, timeout time.Duration) (*CaddyClient, error) {
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	return &CaddyClient{u, timeout}, nil
}

func (client *CaddyClient) getContextTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if client.timeout > 0 {
		return context.WithTimeout(ctx, client.timeout)
	} else {
		return ctx, func() {}
	}
}

func (client *CaddyClient) getConfig(ctx context.Context, id string) (res *http.Response, config json.RawMessage, present bool, etag string, err error) {
	url, err := client.endpoint.Parse("/id/" + id)
	if err != nil {
		return nil, nil, false, "", err
	}

	reqCtx, cancel := client.getContextTimeout(ctx)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, "GET", url.String(), nil)
	if err != nil {
		return nil, nil, false, "", err
	}

	res, err = http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil, false, "", err
	}

	etag = res.Header.Get("Etag")

	present = res.StatusCode >= 200 && res.StatusCode < 300

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return res, nil, present, etag, err
	}

	config = body

	return res, config, present, etag, nil
}

func (client *CaddyClient) postConfig(ctx context.Context, etag string, locationId string, config json.RawMessage) (res *http.Response, retry bool, err error) {
	return client.sendConfig(ctx, http.MethodPost, etag, locationId, config)
}

func (client *CaddyClient) patchConfig(ctx context.Context, etag string, locationId string, config json.RawMessage) (res *http.Response, retry bool, err error) {
	return client.sendConfig(ctx, http.MethodPatch, etag, locationId, config)
}

func (client *CaddyClient) sendConfig(ctx context.Context, httpMethod string, etag string, locationId string, config json.RawMessage) (res *http.Response, retry bool, err error) {
	url, err := client.endpoint.Parse("/id/" + locationId)
	if err != nil {
		return nil, false, err
	}

	reqCtx, cancel := client.getContextTimeout(ctx)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, httpMethod, url.String(), bytes.NewBuffer(config))
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Content-Type", "application/json")
	if etag != "" {
		req.Header.Set("If-Match", etag)
	}

	res, err = http.DefaultClient.Do(req.WithContext(reqCtx))
	if err != nil {
		return nil, false, err
	}

	if res.StatusCode == 412 {
		return res, true, nil
	}

	code_valid := res.StatusCode >= 200 && res.StatusCode < 300
	if !code_valid {
		body, err := io.ReadAll(res.Body)
		if err != nil {
			return res, false, err
		}

		return res, false, fmt.Errorf("cannot update Caddy config; HTTP error %s: %s", res.Status, string(body))
	}
	return res, false, nil
}

func (client *CaddyClient) deleteConfig(ctx context.Context, etag string, locationId string) (res *http.Response, retry bool, err error) {
	url, err := client.endpoint.Parse("/id/" + locationId + "/")
	if err != nil {
		return nil, false, err
	}

	reqCtx, cancel := client.getContextTimeout(ctx)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodDelete, url.String(), nil)
	if err != nil {
		return nil, false, err
	}
	if etag != "" {
		req.Header.Set("If-Match", etag)
	}

	res, err = http.DefaultClient.Do(req.WithContext(reqCtx))
	if err != nil {
		return nil, false, err
	}

	if res.StatusCode == 412 {
		return res, true, nil
	}

	code_valid := res.StatusCode >= 200 && res.StatusCode < 300 || res.StatusCode == 404
	if !code_valid {
		body, err := io.ReadAll(res.Body)
		if err != nil {
			return res, false, err
		}

		return res, false, fmt.Errorf("cannot update Caddy config; HTTP error %s: %s", res.Status, string(body))
	}
	return res, false, nil
}

func (client *CaddyClient) getScalarConfig(ctx context.Context, id_parent string, value json.RawMessage) (res *http.Response, present bool, etag string, err error) {
	res, body, present, etag, err := client.getConfig(ctx, id_parent)

	var arr []json.RawMessage
	err = json.Unmarshal(body, &arr)
	if err != nil {
		return res, false, etag, fmt.Errorf("cannot unmarshal actual config in an array for register_only proxy config, %v", err)
	}

	present = false
	for _, item := range arr {
		if bytes.Equal(item, value) {
			present = true
		}
	}

	return res, present, etag, nil
}
