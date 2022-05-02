package rest

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
)

type Client struct {
	addr string
}

func NewClient(addr string) *Client {
	return &Client{
		addr: addr,
	}
}

func (c *Client) do(method string, path string, body []byte, respData interface{}) error {
	r := bytes.NewReader(body)

	req, err := http.NewRequest(method, c.addr+path, r)
	if err != nil {
		return err
	}

	req.Header.Add("content-type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	if respData != nil {
		// decoder := json.NewDecoder(resp.Body)
		// err = decoder.Decode(respData)
		fullBody, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		err = json.Unmarshal(fullBody, respData)
		return err
	}

	return nil
}

func (c *Client) GetStatus() (status *StatusResponse, err error) {
	err = c.do("GET", "/status", nil, &status)
	return
}

func (c *Client) handleBasicResponse(br *BasicResponse) (msg string, err error) {
	if br.Error {
		return "", errors.New(br.Message)
	}

	return br.Message, nil
}

func (c *Client) StartNewNode() (msg string, err error) {
	var resp BasicResponse
	err = c.do("POST", "/startnode", nil, &resp)
	if err != nil {
		return "", err
	}

	return c.handleBasicResponse(&resp)
}

func (c *Client) ShutdownNode(nodeID string) (msg string, err error) {
	var resp BasicResponse

	body := url.Values{
		"node_id": []string{nodeID},
	}.Encode()

	err = c.do("POST", "/shutdownnode", []byte(body), &resp)
	if err != nil {
		return "", err
	}

	return c.handleBasicResponse(&resp)
}

func (c *Client) MigrateShard(newNodeID string, shard int) (msg string, err error) {
	body := url.Values{
		"destination_node": []string{newNodeID},
		"shard":            []string{strconv.Itoa(shard)},
	}.Encode()

	var resp BasicResponse
	err = c.do("POST", "/migrateshard", []byte(body), &resp)
	if err != nil {
		return "", err
	}

	return c.handleBasicResponse(&resp)
}

func (c *Client) MigrateNode(originNodeID, newNodeID string, shutdown bool) (msg string, err error) {
	bodyVals := url.Values{
		"destination_node": []string{newNodeID},
		"origin_node":      []string{originNodeID},
	}

	if shutdown {
		bodyVals["shutdown"] = []string{"true"}
	}

	body := bodyVals.Encode()

	var resp BasicResponse
	err = c.do("POST", "/migratenode", []byte(body), &resp)
	if err != nil {
		return "", err
	}

	return c.handleBasicResponse(&resp)
}

func (c *Client) MigrateAllNodesToNewNodes() (msg string, err error) {
	var resp BasicResponse
	err = c.do("POST", "/fullmigration", nil, &resp)
	if err != nil {
		return "", err
	}

	return c.handleBasicResponse(&resp)
}

func (c *Client) StopShard(shard int) (msg string, err error) {
	body := url.Values{
		"shard": []string{strconv.Itoa(shard)},
	}.Encode()

	var resp BasicResponse
	err = c.do("POST", "/stopshard", []byte(body), &resp)
	if err != nil {
		return "", err
	}

	return c.handleBasicResponse(&resp)
}

func (c *Client) BlacklistNode(node string) (msg string, err error) {
	bodyVals := url.Values{
		"node": []string{node},
	}

	body := bodyVals.Encode()

	var resp BasicResponse
	err = c.do("POST", "/blacklistnode", []byte(body), &resp)
	if err != nil {
		return "", err
	}

	return c.handleBasicResponse(&resp)
}

func (c *Client) PullNewVersion() (newVersion string, err error) {
	var resp BasicResponse
	err = c.do("POST", "/pullnewversion", nil, &resp)
	if err != nil {
		return "", err
	}

	return c.handleBasicResponse(&resp)
}

func (c *Client) GetDeployedVersion() (version string, err error) {
	var resp BasicResponse
	err = c.do("GET", "/deployedversion", nil, &resp)
	if err != nil {
		return "", err
	}

	return c.handleBasicResponse(&resp)
}
