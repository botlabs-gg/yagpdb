package web

import (
	"bytes"
	"errors"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common/templates"
	"html/template"
	"time"
)

func prettyTime(t time.Time) string {
	return t.Format(time.RFC822)
}

// mTemplate combines "template" with dictionary. so you can specify multiple variables
// and call templates almost as if they were functions with arguments
// makes certain templates a lot simpler
func mTemplate(name string, values ...interface{}) (template.HTML, error) {

	data, err := templates.Dictionary(values...)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	err = Templates.ExecuteTemplate(&buf, name, data)
	if err != nil {
		return "", err
	}

	return template.HTML(buf.String()), nil
}

var permsString = map[string]int{
	"ManageRoles":    discordgo.PermissionManageRoles,
	"ManageMessages": discordgo.PermissionManageMessages,
}

func hasPerm(botPerms int, checkPerm string) (bool, error) {
	p, ok := permsString[checkPerm]
	if !ok {
		return false, errors.New("Unknown permission: " + checkPerm)
	}

	return botPerms&p != 0, nil
}
