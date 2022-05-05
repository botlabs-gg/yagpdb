package internalapi

import (
	"bytes"
	"encoding/json"
	"net/http"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/common"
)

var (
	ErrServerError     = errors.New("internal api server is having issues")
	ErrCantFindAddress = errors.New("can't find address for provided shard")
)

func GetServerAddrForGuild(guildID int64) string {
	addr, _ := common.ServicePoller.GetGuildAddress(guildID)
	return addr
}

func GetServerAddrForShard(shard int) string {
	addr, _ := common.ServicePoller.GetShardAddress(shard)
	return addr
}

func GetWithGuild(guildID int64, url string, dest interface{}) error {
	serverAddr := GetServerAddrForGuild(guildID)
	if serverAddr == "" {
		return ErrCantFindAddress
	}

	return GetWithAddress(serverAddr, url, dest)
}

func GetWithShard(shard int, url string, dest interface{}) error {
	serverAddr := GetServerAddrForShard(shard)
	if serverAddr == "" {
		return ErrCantFindAddress
	}

	return GetWithAddress(serverAddr, url, dest)
}

func GetWithAddress(addr string, url string, dest interface{}) error {
	resp, err := http.Get("http://" + addr + "/" + url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		var errDest string
		err := json.NewDecoder(resp.Body).Decode(&errDest)
		if err != nil {
			return ErrServerError
		}

		return errors.New(errDest)
	}

	return errors.WithMessage(json.NewDecoder(resp.Body).Decode(dest), "json.Decode")
}

func PostWithGuild(guildID int64, url string, bodyData interface{}, dest interface{}) error {
	serverAddr := GetServerAddrForGuild(guildID)
	if serverAddr == "" {
		return ErrCantFindAddress
	}

	return PostWithAddress(serverAddr, url, bodyData, dest)
}

func PostWithShard(shard int, url string, bodyData interface{}, dest interface{}) error {
	serverAddr := GetServerAddrForShard(shard)
	if serverAddr == "" {
		return ErrCantFindAddress
	}

	return PostWithAddress(serverAddr, url, bodyData, dest)
}

func PostWithAddress(serverAddr string, url string, bodyData interface{}, dest interface{}) error {
	var bodyBuf bytes.Buffer
	if bodyData != nil {
		encoder := json.NewEncoder(&bodyBuf)
		err := encoder.Encode(bodyData)
		if err != nil {
			return err
		}
	}
	resp, err := http.Post("http://"+serverAddr+"/"+url, "application/json", &bodyBuf)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		var errDest string
		err := json.NewDecoder(resp.Body).Decode(&errDest)
		if err != nil {
			return errors.Errorf("Resp code:%d, Error: %v", resp.StatusCode, err)
		}

		return errors.New(errDest)
	}

	if dest == nil {
		return nil
	}

	return errors.WithMessage(json.NewDecoder(resp.Body).Decode(dest), "json.Decode")
}
