package templates

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

var ErrTooManyInteractionResponses = errors.New("cannot respond to an interaction > 1 time; consider using a followup")

func interactionContextFuncs(c *Context) {
	c.addContextFunc("deleteInteractionResponse", c.tmplDeleteInteractionResponse)
	c.addContextFunc("editResponse", c.tmplEditInteractionResponse(true))
	c.addContextFunc("editResponseNoEscape", c.tmplEditInteractionResponse(false))
	c.addContextFunc("ephemeralResponse", c.tmplEphemeralResponse)
	c.addContextFunc("getResponse", c.tmplGetResponse)
	c.addContextFunc("sendModal", c.tmplSendModal)
	c.addContextFunc("sendResponse", c.tmplSendInteractionResponse(true, false))
	c.addContextFunc("sendResponseNoEscape", c.tmplSendInteractionResponse(false, false))
	c.addContextFunc("sendResponseNoEscapeRetID", c.tmplSendInteractionResponse(false, true))
	c.addContextFunc("sendResponseRetID", c.tmplSendInteractionResponse(true, true))
	c.addContextFunc("updateMessage", c.tmplUpdateMessage(true))
	c.addContextFunc("updateMessageNoEscape", c.tmplUpdateMessage(false))
}

func CreateComponent(expectedType discordgo.ComponentType, values ...interface{}) (discordgo.MessageComponent, error) {
	if len(values) < 1 && expectedType != discordgo.ActionsRowComponent {
		return discordgo.ActionsRow{}, errors.New("no values passed to component builder")
	}

	var m map[string]interface{}
	switch t := values[0].(type) {
	case SDict:
		m = t
	case *SDict:
		m = *t
	case map[string]interface{}:
		m = t
	default:
		dict, err := StringKeyDictionary(values...)
		if err != nil {
			return nil, err
		}
		m = dict
	}

	encoded, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}

	var component discordgo.MessageComponent
	switch expectedType {
	case discordgo.ActionsRowComponent:
		component = discordgo.ActionsRow{}
	case discordgo.ButtonComponent:
		var comp discordgo.Button
		err = json.Unmarshal(encoded, &comp)
		component = comp
	case discordgo.SelectMenuComponent:
		var comp discordgo.SelectMenu
		err = json.Unmarshal(encoded, &comp)
		component = comp
	case discordgo.TextInputComponent:
		var comp discordgo.TextInput
		err = json.Unmarshal(encoded, &comp)
		component = comp
	case discordgo.UserSelectMenuComponent:
		comp := discordgo.SelectMenu{MenuType: discordgo.UserSelectMenu}
		err = json.Unmarshal(encoded, &comp)
		component = comp
	case discordgo.RoleSelectMenuComponent:
		comp := discordgo.SelectMenu{MenuType: discordgo.RoleSelectMenu}
		err = json.Unmarshal(encoded, &comp)
		component = comp
	case discordgo.MentionableSelectMenuComponent:
		comp := discordgo.SelectMenu{MenuType: discordgo.MentionableSelectMenu}
		err = json.Unmarshal(encoded, &comp)
		component = comp
	case discordgo.ChannelSelectMenuComponent:
		comp := discordgo.SelectMenu{MenuType: discordgo.ChannelSelectMenu}
		err = json.Unmarshal(encoded, &comp)
		component = comp
	}

	if err != nil {
		return nil, err
	}

	return component, nil
}

func CreateButton(values ...interface{}) (*discordgo.Button, error) {
	var messageSdict map[string]interface{}
	switch t := values[0].(type) {
	case SDict:
		messageSdict = t
	case *SDict:
		messageSdict = *t
	case map[string]interface{}:
		messageSdict = t
	case *discordgo.Button:
		return t, nil
	default:
		dict, err := StringKeyDictionary(values...)
		if err != nil {
			return nil, err
		}
		messageSdict = dict
	}

	convertedButton := make(map[string]interface{})
	for k, v := range messageSdict {
		switch strings.ToLower(k) {
		case "style":
			var val string
			switch typed := v.(type) {
			case string:
				val = typed
			case discordgo.ButtonStyle:
				val = strconv.Itoa(int(typed))
			case *discordgo.ButtonStyle:
				val = strconv.Itoa(int(*typed))
			default:
				num := tmplToInt(typed)
				if num < 1 || num > 5 {
					return nil, errors.New("invalid button style")
				}
				val = strconv.Itoa(num)
			}

			switch strings.ToLower(val) {
			case "primary", "blue", "purple", "blurple", "1":
				convertedButton["style"] = discordgo.PrimaryButton
			case "secondary", "grey", "2":
				convertedButton["style"] = discordgo.SecondaryButton
			case "success", "green", "3":
				convertedButton["style"] = discordgo.SuccessButton
			case "danger", "destructive", "red", "4":
				convertedButton["style"] = discordgo.DangerButton
			case "link", "url", "5":
				convertedButton["style"] = discordgo.LinkButton
			default:
				return nil, errors.New("invalid button style")
			}
		case "link":
			// discord made a button style named "link" but it needs a "url"
			// not a "link" field. this makes it a bit more user friendly
			convertedButton["url"] = v
		default:
			convertedButton[k] = v
		}
	}

	var button discordgo.Button
	b, err := CreateComponent(discordgo.ButtonComponent, convertedButton)
	if err == nil {
		button = b.(discordgo.Button)
		// validation
		if button.Style == discordgo.LinkButton && button.URL == "" {
			return nil, errors.New("a url field is required for a link button")
		}
		if button.Label == "" && button.Emoji == nil {
			return nil, errors.New("button must have a label or emoji")
		}
	}
	return &button, err
}

func CreateSelectMenu(values ...interface{}) (*discordgo.SelectMenu, error) {
	var messageSdict map[string]interface{}
	switch t := values[0].(type) {
	case SDict:
		messageSdict = t
	case *SDict:
		messageSdict = *t
	case map[string]interface{}:
		messageSdict = t
	case *discordgo.SelectMenu:
		return t, nil
	default:
		dict, err := StringKeyDictionary(values...)
		if err != nil {
			return nil, err
		}
		messageSdict = dict
	}

	menuType := discordgo.SelectMenuComponent

	convertedMenu := make(map[string]interface{})
	for k, v := range messageSdict {
		switch strings.ToLower(k) {
		case "type":
			val, ok := v.(string)
			if !ok {
				return nil, errors.New("invalid select menu type")
			}
			switch strings.ToLower(val) {
			case "string", "text":
			case "user":
				menuType = discordgo.UserSelectMenuComponent
			case "role":
				menuType = discordgo.RoleSelectMenuComponent
			case "mentionable":
				menuType = discordgo.MentionableSelectMenuComponent
			case "channel":
				menuType = discordgo.ChannelSelectMenuComponent
			default:
				return nil, errors.New("invalid select menu type")
			}
		default:
			convertedMenu[k] = v
		}
	}

	var menu discordgo.SelectMenu
	m, err := CreateComponent(menuType, convertedMenu)
	if err == nil {
		menu = m.(discordgo.SelectMenu)

		// validation
		if menu.MenuType == discordgo.StringSelectMenu && len(menu.Options) < 1 || len(menu.Options) > 25 {
			return nil, errors.New("invalid number of menu options, must have between 1 and 25")
		}
		if menu.MinValues != nil {
			if *menu.MinValues < 1 || *menu.MinValues > 25 {
				return nil, errors.New("invalid min values, must be between 1 and 25")
			}
		}
		if menu.MaxValues > 25 {
			return nil, errors.New("invalid max values, max 25")
		}
		checked := []string{}
		for _, o := range menu.Options {
			if in(checked, o.Value) {
				return nil, errors.New("select menu options must have unique values")
			}
			checked = append(checked, o.Value)
		}
	}
	return &menu, err
}

func CreateModal(values ...interface{}) (*discordgo.InteractionResponse, error) {
	if len(values) < 1 {
		return &discordgo.InteractionResponse{}, errors.New("no values passed to component builder")
	}

	var m map[string]interface{}
	switch t := values[0].(type) {
	case SDict:
		m = t
	case *SDict:
		m = *t
	case map[string]interface{}:
		m = t
	default:
		dict, err := StringKeyDictionary(values...)
		if err != nil {
			return nil, err
		}
		m = dict
	}

	modal := &discordgo.InteractionResponseData{CustomID: "templates-0"} // default cID if not set

	for key, val := range m {
		switch key {
		case "title":
			modal.Title = ToString(val)
		case "custom_id":
			modal.CustomID = "templates-" + ToString(val)
		case "fields":
			if val == nil {
				continue
			}
			v, _ := indirect(reflect.ValueOf(val))
			if v.Kind() == reflect.Slice {
				const maxRows = 5 // Discord limitation
				usedCustomIDs := []string{}
				for i := 0; i < v.Len() && i < maxRows; i++ {
					f, err := CreateComponent(discordgo.TextInputComponent, v.Index(i).Interface())
					if err != nil {
						return nil, err
					}
					field := f.(discordgo.TextInput)
					// validation
					if field.Style == 0 {
						field.Style = discordgo.TextInputShort
					}
					err = validateCustomID(&field.CustomID, i, &usedCustomIDs)
					if err != nil {
						return nil, err
					}
					modal.Components = append(modal.Components, discordgo.ActionsRow{[]discordgo.MessageComponent{field}})
				}
			} else {
				f, err := CreateComponent(discordgo.TextInputComponent, val)
				if err != nil {
					return nil, err
				}
				field := f.(discordgo.TextInput)
				if field.Style == 0 {
					field.Style = discordgo.TextInputShort
				}
				validateCustomID(&field.CustomID, 0, nil)
				modal.Components = append(modal.Components, discordgo.ActionsRow{[]discordgo.MessageComponent{field}})
			}
		default:
			return nil, errors.New(`invalid key "` + key + `" passed to send message builder`)
		}

	}

	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: modal,
	}, nil
}

func distributeComponents(components reflect.Value) (returnComponents []discordgo.MessageComponent, err error) {
	if components.Len() < 1 {
		return
	}

	const maxRows = 5       // Discord limitation
	const maxComponents = 5 // (per action row) Discord limitation
	v, _ := indirect(reflect.ValueOf(components.Index(0).Interface()))
	if v.Kind() == reflect.Slice {
		// slice within a slice. user is defining their own action row
		// layout; treat each slice as an action row
		for rowIdx := 0; rowIdx < components.Len() && rowIdx < maxRows; rowIdx++ {
			currentInputRow := reflect.ValueOf(components.Index(rowIdx).Interface())
			tempRow := discordgo.ActionsRow{}
			for compIdx := 0; compIdx < currentInputRow.Len() && compIdx < maxComponents; compIdx++ {
				var component discordgo.MessageComponent
				switch val := currentInputRow.Index(compIdx).Interface().(type) {
				case *discordgo.Button:
					component = val
				case *discordgo.SelectMenu:
					component = val
				default:
					return nil, errors.New("invalid component passed to send message builder")
				}
				if component.Type() == discordgo.SelectMenuComponent && len(tempRow.Components) > 0 {
					return nil, errors.New("a select menu cannot share an action row with other components")
				}
				tempRow.Components = append(tempRow.Components, component)
				if component.Type() == discordgo.SelectMenuComponent {
					break // move on to next row
				}
			}
			returnComponents = append(returnComponents, &tempRow)
		}
	} else {
		// user just slapped a bunch of components into a slice. we need to organize ourselves
		tempRow := discordgo.ActionsRow{Components: []discordgo.MessageComponent{}}
		for i := 0; i < components.Len() && i < maxRows*maxComponents; i++ {
			var component discordgo.MessageComponent
			var isMenu bool

			switch val := components.Index(i).Interface().(type) {
			case *discordgo.Button:
				component = val
			case *discordgo.SelectMenu:
				isMenu = true
				component = val
			default:
				return nil, errors.New("invalid component passed to send message builder")
			}

			availableSpace := 5 - len(tempRow.Components)
			if !isMenu && availableSpace > 0 || isMenu && availableSpace == 5 {
				tempRow.Components = append(tempRow.Components, component)
			} else {
				returnComponents = append(returnComponents, tempRow)
				tempRow.Components = []discordgo.MessageComponent{component}
			}

			// if it's a menu, the row is full now, append and start a new one
			if isMenu {
				returnComponents = append(returnComponents, tempRow)
				tempRow.Components = []discordgo.MessageComponent{}
			}

			if i == components.Len()-1 && len(tempRow.Components) > 0 { // if we're at the end, append the last row
				returnComponents = append(returnComponents, tempRow)
			}
		}
	}
	return
}

// validateCustomID sets a unique custom ID based on componentIndex if needed
// and returns an error if id is already in used
func validateCustomID(id *string, componentIndex int, used *[]string) error {
	if id == nil {
		return nil
	}

	if *id == "" {
		*id = fmt.Sprint(componentIndex)
	}

	if !strings.HasPrefix(*id, "templates-") {
		*id = fmt.Sprint("templates-", *id)
	}

	const maxCIDLength = 100 // discord limitation
	if len(*id) > maxCIDLength {
		return errors.New("custom id too long (max 90 chars)") // maxCIDLength - len("templates-")
	}

	if used == nil {
		return nil
	}

	if in(*used, *id) {
		return errors.New("duplicate custom ids used")
	}
	return nil
}

// validateCustomIDs sets unique custom IDs for any component in the action
// rows provided in the slice, sets any link buttons' ids to an empty string,
// and returns an error if duplicate custom ids are used.
func validateActionRowsCustomIDs(rows *[]discordgo.MessageComponent) error {
	used := []string{}
	newComponents := []discordgo.MessageComponent{}
	for rowIdx := 0; rowIdx < len(*rows); rowIdx++ {
		var row *discordgo.ActionsRow
		switch r := (*rows)[rowIdx].(type) {
		case discordgo.ActionsRow:
			row = &r
		case *discordgo.ActionsRow:
			row = r
		}
		rowComps := row.Components
		for compIdx := 0; compIdx < len(rowComps); compIdx++ {
			componentCount := rowIdx*5 + compIdx
			var err error
			switch c := rowComps[compIdx].(type) {
			case *discordgo.Button:
				if c.Style == discordgo.LinkButton {
					c.CustomID = ""
					continue
				}
				err = validateCustomID(&c.CustomID, componentCount, &used)
				used = append(used, c.CustomID)
			case *discordgo.SelectMenu:
				err = validateCustomID(&c.CustomID, componentCount, &used)
				used = append(used, c.CustomID)
			}
			if err != nil {
				return err
			}
		}
		newComponents = append(newComponents, discordgo.ActionsRow{rowComps})
	}
	*rows = newComponents
	return nil
}

func (c *Context) tmplDeleteInteractionResponse(interactionToken, msgID interface{}, delaySeconds ...interface{}) (interface{}, error) {
	if c.IncreaseCheckGenericAPICall() {
		return "", ErrTooManyAPICalls
	}

	_, token := c.tokenArg(interactionToken)
	if token == "" {
		return "", errors.New("invalid interaction token")
	}

	dur := 10
	if len(delaySeconds) > 0 {
		dur = int(ToInt64(delaySeconds[0]))
	}

	// MaybeScheduledDeleteMessage limits delete delays for interaction
	// responses/followups to 10 seconds, so no need to do it here too

	// guild/channel IDs irrelevant when deleting responses or followups
	MaybeScheduledDeleteMessage(0, 0, ToInt64(msgID), dur, token)

	return "", nil
}

func (c *Context) tmplEditInteractionResponse(filterSpecialMentions bool) func(interactionToken, msgID, msg interface{}) (interface{}, error) {
	parseMentions := []discordgo.AllowedMentionType{discordgo.AllowedMentionTypeUsers}
	if !filterSpecialMentions {
		parseMentions = append(parseMentions, discordgo.AllowedMentionTypeRoles, discordgo.AllowedMentionTypeEveryone)
	}
	return func(interactionToken, msgID, msg interface{}) (interface{}, error) {
		if c.IncreaseCheckGenericAPICall() {
			return "", ErrTooManyAPICalls
		}

		_, token := c.tokenArg(interactionToken)
		if token == "" {
			return "", errors.New("invalid interaction token")
		}

		var editOriginal bool
		mID := ToInt64(msgID)
		if mID == 0 {
			editOriginal = true
		}

		msgEdit := &discordgo.WebhookParams{
			AllowedMentions: &discordgo.AllowedMentions{Parse: parseMentions},
		}
		var err error

		switch typedMsg := msg.(type) {

		case *discordgo.MessageEmbed:
			msgEdit.Embeds = []*discordgo.MessageEmbed{typedMsg}
		case []*discordgo.MessageEmbed:
			msgEdit.Embeds = typedMsg
		case *discordgo.MessageEdit:
			embeds := make([]*discordgo.MessageEmbed, 0, len(typedMsg.Embeds))
			//If there are no Embeds and string are explicitly set as null, give an error message.
			if typedMsg.Content != nil && strings.TrimSpace(*typedMsg.Content) == "" {
				if len(typedMsg.Embeds) == 0 && len(typedMsg.Components) == 0 {
					return "", errors.New("both content and embed cannot be null")
				}

				//only keep valid embeds
				for _, e := range typedMsg.Embeds {
					if e != nil && !e.GetMarshalNil() {
						embeds = append(typedMsg.Embeds, e)
					}
				}
				if len(embeds) == 0 && len(typedMsg.Components) == 0 {
					return "", errors.New("both content and embed cannot be null")
				}
			}
			if typedMsg.Content != nil {
				msgEdit.Content = *typedMsg.Content
			}
			msgEdit.Embeds = typedMsg.Embeds
			msgEdit.Components = typedMsg.Components
			msgEdit.AllowedMentions = &typedMsg.AllowedMentions
		default:
			temp := fmt.Sprint(msg)
			msgEdit.Content = temp
		}

		if !filterSpecialMentions {
			msgEdit.AllowedMentions = &discordgo.AllowedMentions{
				Parse: []discordgo.AllowedMentionType{discordgo.AllowedMentionTypeUsers, discordgo.AllowedMentionTypeRoles, discordgo.AllowedMentionTypeEveryone},
			}
		}

		if editOriginal {
			_, err = common.BotSession.EditOriginalInteractionResponse(common.BotApplication.ID, token, msgEdit)
			if err == nil && token == c.CurrentFrame.Interaction.Token {
				c.CurrentFrame.Interaction.RespondedTo = true
			}
		} else {
			_, err = common.BotSession.EditFollowupMessage(common.BotApplication.ID, token, mID, msgEdit)
		}

		if err != nil {
			return "", err
		}

		return "", nil
	}
}

func (c *Context) tmplEphemeralResponse() string {
	if c.CurrentFrame.Interaction != nil {
		c.CurrentFrame.EphemeralResponse = true
	}
	return ""
}

func (c *Context) tmplGetResponse(interactionToken, msgID interface{}) (message *discordgo.Message, err error) {
	if c.IncreaseCheckGenericAPICall() {
		return nil, ErrTooManyAPICalls
	}

	_, token := c.tokenArg(interactionToken)
	if token == "" {
		return nil, errors.New("invalid interaction token")
	}

	var getOriginal bool
	mID := ToInt64(msgID)
	if mID == 0 {
		getOriginal = true
	}

	if getOriginal {
		message, err = common.BotSession.GetOriginalInteractionResponse(common.BotApplication.ID, token)
	} else {
		message, err = common.BotSession.WebhookMessage(common.BotApplication.ID, token, mID)
	}

	return
}

func (c *Context) tmplSendModal(modal interface{}) (interface{}, error) {
	if c.CurrentFrame.Interaction == nil {
		return "", errors.New("no interaction data in context")
	}

	if c.IncreaseCheckGenericAPICall() {
		return "", ErrTooManyAPICalls
	}

	if c.IncreaseCheckCallCounter("modal", 1) {
		return "", errors.New("cannot send multiple modals to the same interaction")
	}

	if c.IncreaseCheckCallCounter("interaction_response", 1) {
		return "", ErrTooManyInteractionResponses
	}

	var typedModal *discordgo.InteractionResponse
	var err error
	switch m := modal.(type) {
	case *discordgo.InteractionResponse:
		typedModal = m
	case discordgo.InteractionResponse:
		typedModal = &m
	case SDict, *SDict, map[string]interface{}:
		typedModal, err = CreateModal(m)
	default:
		return "", errors.New("invalid modal passed to sendModal")
	}
	if err != nil {
		return "", err
	}

	if typedModal.Type != discordgo.InteractionResponseModal {
		return "", errors.New("invalid modal passed to sendModal")
	}

	err = common.BotSession.CreateInteractionResponse(c.CurrentFrame.Interaction.ID, c.CurrentFrame.Interaction.Token, typedModal)
	if err != nil {
		return "", err
	}
	c.CurrentFrame.Interaction.RespondedTo = true
	return "", nil
}

func (c *Context) tmplSendInteractionResponse(filterSpecialMentions bool, returnID bool) func(interactionToken interface{}, msg interface{}) interface{} {
	var repliedUser bool
	parseMentions := []discordgo.AllowedMentionType{discordgo.AllowedMentionTypeUsers}
	if !filterSpecialMentions {
		parseMentions = append(parseMentions, discordgo.AllowedMentionTypeRoles, discordgo.AllowedMentionTypeEveryone)
		repliedUser = true
	}

	return func(interactionToken interface{}, msg interface{}) interface{} {
		if c.IncreaseCheckGenericAPICall() {
			return ""
		}

		sendType, token := c.tokenArg(interactionToken)
		if token == "" {
			return ""
		}

		var m *discordgo.Message
		msgSend := &discordgo.InteractionResponseData{
			AllowedMentions: &discordgo.AllowedMentions{
				Parse:       parseMentions,
				RepliedUser: repliedUser,
			},
		}
		var err error

		switch typedMsg := msg.(type) {
		case *discordgo.MessageEmbed:
			msgSend.Embeds = []*discordgo.MessageEmbed{typedMsg}
		case []*discordgo.MessageEmbed:
			msgSend.Embeds = typedMsg
		case *discordgo.MessageSend:
			msgSend.Content = typedMsg.Content
			msgSend.Embeds = typedMsg.Embeds
			msgSend.Components = typedMsg.Components
			msgSend.Flags = typedMsg.Flags
			msgSend.Files = typedMsg.Files
			if typedMsg.File != nil {
				msgSend.Files = []*discordgo.File{typedMsg.File}
			}
			if !filterSpecialMentions {
				msgSend.AllowedMentions = &discordgo.AllowedMentions{Parse: parseMentions, RepliedUser: repliedUser}
			}
		default:
			msgSend.Content = ToString(msg)
		}

		switch sendType {
		case sendMessageInteractionResponse:
			if c.IncreaseCheckCallCounter("interaction_response", 1) {
				return ""
			}
			err = common.BotSession.CreateInteractionResponse(c.CurrentFrame.Interaction.ID, token, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: msgSend,
			})
			if err == nil {
				if token == c.CurrentFrame.Interaction.Token {
					c.CurrentFrame.Interaction.RespondedTo = true
				}
				if returnID {
					m, err = common.BotSession.GetOriginalInteractionResponse(common.BotApplication.ID, token)
				}
			}
		case sendMessageInteractionFollowup:
			var file *discordgo.File
			if len(msgSend.Files) > 0 {
				file = msgSend.Files[0]
			}

			m, err = common.BotSession.CreateFollowupMessage(common.BotApplication.ID, token, &discordgo.WebhookParams{
				Content:         msgSend.Content,
				Components:      msgSend.Components,
				Embeds:          msgSend.Embeds,
				AllowedMentions: msgSend.AllowedMentions,
				Flags:           int64(msgSend.Flags),
				File:            file,
			})
		}

		if err == nil && returnID {
			return m.ID
		}

		return ""
	}
}

func (c *Context) tmplUpdateMessage(filterSpecialMentions bool) func(msg interface{}) (interface{}, error) {
	parseMentions := []discordgo.AllowedMentionType{discordgo.AllowedMentionTypeUsers}
	if !filterSpecialMentions {
		parseMentions = append(parseMentions, discordgo.AllowedMentionTypeRoles, discordgo.AllowedMentionTypeEveryone)
	}
	return func(msg interface{}) (interface{}, error) {
		if c.CurrentFrame.Interaction == nil {
			return "", errors.New("no interaction data in context; consider editMessage or editResponse")
		}

		if c.IncreaseCheckGenericAPICall() {
			return "", ErrTooManyAPICalls
		}

		if c.IncreaseCheckCallCounter("interaction_response", 1) {
			return "", ErrTooManyInteractionResponses
		}

		msgEdit := &discordgo.InteractionResponseData{
			AllowedMentions: &discordgo.AllowedMentions{Parse: parseMentions},
		}
		var err error

		switch typedMsg := msg.(type) {

		case *discordgo.MessageEmbed:
			msgEdit.Embeds = []*discordgo.MessageEmbed{typedMsg}
		case []*discordgo.MessageEmbed:
			msgEdit.Embeds = typedMsg
		case *discordgo.MessageEdit:
			embeds := make([]*discordgo.MessageEmbed, 0, len(typedMsg.Embeds))
			//If there are no Embeds and string are explicitly set as null, give an error message.
			if typedMsg.Content != nil && strings.TrimSpace(*typedMsg.Content) == "" {
				if len(typedMsg.Embeds) == 0 && len(typedMsg.Components) == 0 {
					return "", errors.New("both content and embed cannot be null")
				}

				//only keep valid embeds
				for _, e := range typedMsg.Embeds {
					if e != nil && !e.GetMarshalNil() {
						embeds = append(typedMsg.Embeds, e)
					}
				}
				if len(embeds) == 0 && len(typedMsg.Components) == 0 {
					return "", errors.New("both content and embed cannot be null")
				}
			}
			if typedMsg.Content != nil {
				msgEdit.Content = *typedMsg.Content
			}
			msgEdit.Embeds = typedMsg.Embeds
			msgEdit.Components = typedMsg.Components
			msgEdit.AllowedMentions = &typedMsg.AllowedMentions
		default:
			temp := fmt.Sprint(msg)
			msgEdit.Content = temp
		}

		if !filterSpecialMentions {
			msgEdit.AllowedMentions = &discordgo.AllowedMentions{
				Parse: []discordgo.AllowedMentionType{discordgo.AllowedMentionTypeUsers, discordgo.AllowedMentionTypeRoles, discordgo.AllowedMentionTypeEveryone},
			}
		}

		err = common.BotSession.CreateInteractionResponse(c.CurrentFrame.Interaction.ID, c.CurrentFrame.Interaction.Token, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: msgEdit,
		})

		if err != nil {
			return "", err
		}

		c.CurrentFrame.Interaction.RespondedTo = true
		return "", nil
	}
}

// tokenArg validates the interaction token, or falls back to the one in
// context if it exists. it returns an empty string on failure of both of
// these. also returns the sendMessageType.
func (c *Context) tokenArg(interactionToken interface{}) (sendType sendMessageType, token string) {
	sendType = sendMessageInteractionFollowup

	token, ok := interactionToken.(string)
	if !ok {
		if interactionToken == nil && c.CurrentFrame.Interaction != nil {
			// no token provided, assume current interaction
			token = c.CurrentFrame.Interaction.Token
		} else {
			return
		}
	}

	if c.CurrentFrame.Interaction != nil && token == c.CurrentFrame.Interaction.Token && !c.CurrentFrame.Interaction.RespondedTo {
		sendType = sendMessageInteractionResponse
	}
	return
}
