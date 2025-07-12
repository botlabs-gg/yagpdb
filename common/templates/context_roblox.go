package templates

import (
	"github.com/RhykerWells/yagpdb/v2/roblox"
)

func (c *Context) tmplGetRobloxUserByID(target interface{}) (interface{}, error) {
	if c.IncreaseCheckGenericAPICall() {
		return nil, ErrTooManyAPICalls
	}

	user, err := roblox.RobloxClient.GetUserByID(ToString(target))
	if err != nil {
		return user, nil
	}

	return user, err
}

func (c *Context) tmplGetRobloxUserByUsername(target interface{}) (interface{}, error) {
	if c.IncreaseCheckGenericAPICall() {
		return nil, ErrTooManyAPICalls
	}

	user, err := roblox.RobloxClient.GetUserByUsername(ToString(target))
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (c *Context) tmplGetRobloxGroupByID(target interface{}) (interface{}, error) {
	if c.IncreaseCheckGenericAPICall() {
		return nil, ErrTooManyAPICalls
	}

	group, err := roblox.RobloxClient.GetGroupByID(ToString(target))
	if err != nil {
		return group, nil
	}

	return group, err
}
