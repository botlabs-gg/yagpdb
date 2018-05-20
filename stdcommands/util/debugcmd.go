package util

import (
	"github.com/jonas747/dcmd"
	"github.com/jonas747/yagpdb/common"
)

func RequireOwner(inner dcmd.RunFunc) dcmd.RunFunc {
	return func(data *dcmd.Data) (interface{}, error) {
		if data.Msg.Author.ID != common.Conf.Owner {
			return "", nil
		}

		return inner(data)
	}
}

