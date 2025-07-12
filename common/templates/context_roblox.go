package templates

import (
	"github.com/RhykerWells/yagpdb/v2/roblox"
)

func (c *Context) tmplGetRobloxUserByID(target interface{}) (interface{}, error) {
	user, err := roblox.RobloxClient.GetUserByID(ToString(target))
	if err != nil {
		return user, nil
	}

	return user, err
}

func (c *Context) tmplGetRobloxUserByUsername(target interface{}) (interface{}, error) {
	user, err := roblox.RobloxClient.GetUserByUsername(ToString(target))
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (c *Context) tmplGetRobloxGroupByID(target interface{}) (interface{}, error) {
	group, _ := roblox.RobloxClient.GetGroupByID(ToString(target))

	return group, nil // Don't return err, a nil output should indicate unknown/invalid group
}

func (c *Context) tmplUpdateGroupRole(group interface{}, target interface{}, role interface{}) (interface{}, error) {
	robloxGroup, err := roblox.RobloxClient.GetGroupByID(ToString(group))
	if err != nil {
		return nil, err
	}

	groupRole, _ := robloxGroup.UpdateUserRole(ToString(target), ToString(role))

	return groupRole, nil // Don't return err, a nil output should indicate unknown/invalid group
}