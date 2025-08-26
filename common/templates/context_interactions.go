package templates

import (
	"fmt"
	"reflect"
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
				usedCustomIDs := make(map[string]bool)
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
					field.CustomID, err = validateCustomID(field.CustomID, usedCustomIDs)
					if err != nil {
						return nil, err
					}
					usedCustomIDs[field.CustomID] = true
					modal.Components = append(modal.Components, discordgo.ActionsRow{Components: []discordgo.InteractiveComponent{field}})
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
				field.CustomID, err = validateCustomID(field.CustomID, nil)
				if err != nil {
					return nil, err
				}
				modal.Components = append(modal.Components, discordgo.ActionsRow{Components: []discordgo.InteractiveComponent{field}})
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

		msgEditResponse := &discordgo.WebhookParams{
			AllowedMentions: &discordgo.AllowedMentions{Parse: []discordgo.AllowedMentionType{discordgo.AllowedMentionTypeUsers}},
		}
		var err error

		switch typedMsg := msg.(type) {

		case *discordgo.MessageEmbed:
			msgEditResponse.Embeds = []*discordgo.MessageEmbed{typedMsg}
		case []*discordgo.MessageEmbed:
			msgEditResponse.Embeds = typedMsg
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
				msgEditResponse.Content = *typedMsg.Content
			}
			msgEditResponse.Embeds = typedMsg.Embeds
			msgEditResponse.Components = typedMsg.Components
			msgEditResponse.AllowedMentions = &typedMsg.AllowedMentions
		case *ComponentBuilder:
			msg, err := typedMsg.ToComplexMessageEdit()
			if err != nil {
				return "", err
			}
			msgEditResponse.Components = msg.Components
			msgEditResponse.Flags = int64(msg.Flags)
			msgEditResponse.AllowedMentions = &msg.AllowedMentions

		default:
			temp := fmt.Sprint(msg)
			msgEditResponse.Content = temp
		}

		parseMentions := []discordgo.AllowedMentionType{discordgo.AllowedMentionTypeUsers}
		var repliedUser bool
		if !filterSpecialMentions {
			parseMentions = append(parseMentions, discordgo.AllowedMentionTypeRoles, discordgo.AllowedMentionTypeEveryone)
			repliedUser = true
			msgEditResponse.AllowedMentions = &discordgo.AllowedMentions{Parse: parseMentions, RepliedUser: repliedUser}
		}

		if editOriginal {
			_, err = common.BotSession.EditOriginalInteractionResponse(common.BotApplication.ID, token, msgEditResponse)
			if err == nil && token == c.CurrentFrame.Interaction.Token {
				c.CurrentFrame.Interaction.RespondedTo = true
				c.CurrentFrame.Interaction.Deferred = false
			}
		} else {
			_, err = common.BotSession.EditFollowupMessage(common.BotApplication.ID, token, mID, msgEditResponse)
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
	return func(interactionToken interface{}, msg interface{}) interface{} {
		if c.IncreaseCheckGenericAPICall() {
			return ""
		}

		sendType, token := c.tokenArg(interactionToken)
		if token == "" {
			return ""
		}

		var m *discordgo.Message
		msgReponse := &discordgo.InteractionResponseData{}
		var err error

		switch typedMsg := msg.(type) {
		case *discordgo.MessageEmbed:
			msgReponse.Embeds = []*discordgo.MessageEmbed{typedMsg}
		case []*discordgo.MessageEmbed:
			msgReponse.Embeds = typedMsg
		case *discordgo.MessageSend:
			msgReponse.Content = typedMsg.Content
			msgReponse.Embeds = typedMsg.Embeds
			msgReponse.Components = typedMsg.Components
			msgReponse.Flags = typedMsg.Flags
			msgReponse.Files = typedMsg.Files
			if typedMsg.File != nil {
				msgReponse.Files = []*discordgo.File{typedMsg.File}
			}
		case *ComponentBuilder:
			msg, err := typedMsg.ToComplexMessage()
			if err != nil {
				return ""
			}
			msgReponse.Components = msg.Components
			msgReponse.Flags = msg.Flags
			msgReponse.Files = msg.Files
			msgReponse.AllowedMentions = &msg.AllowedMentions
			if msg.File != nil {
				msgReponse.Files = []*discordgo.File{msg.File}
			}
		default:
			msgReponse.Content = ToString(msg)
		}

		parseMentions := []discordgo.AllowedMentionType{discordgo.AllowedMentionTypeUsers}
		var repliedUser bool
		if !filterSpecialMentions {
			parseMentions = append(parseMentions, discordgo.AllowedMentionTypeRoles, discordgo.AllowedMentionTypeEveryone)
			repliedUser = true
			msgReponse.AllowedMentions = &discordgo.AllowedMentions{Parse: parseMentions, RepliedUser: repliedUser}
		}

		switch sendType {
		case sendMessageInteractionResponse:
			if c.IncreaseCheckCallCounter("interaction_response", 1) {
				return ""
			}
			err = common.BotSession.CreateInteractionResponse(c.CurrentFrame.Interaction.ID, token, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: msgReponse,
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
			if len(msgReponse.Files) > 0 {
				file = msgReponse.Files[0]
			}

			m, err = common.BotSession.CreateFollowupMessage(common.BotApplication.ID, token, &discordgo.WebhookParams{
				Content:         msgReponse.Content,
				Components:      msgReponse.Components,
				Embeds:          msgReponse.Embeds,
				AllowedMentions: msgReponse.AllowedMentions,
				Flags:           int64(msgReponse.Flags),
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

		msgResponseEdit := &discordgo.InteractionResponseData{
			AllowedMentions: &discordgo.AllowedMentions{Parse: []discordgo.AllowedMentionType{discordgo.AllowedMentionTypeUsers}},
		}
		var err error

		switch typedMsg := msg.(type) {

		case *discordgo.MessageEmbed:
			msgResponseEdit.Embeds = []*discordgo.MessageEmbed{typedMsg}
		case []*discordgo.MessageEmbed:
			msgResponseEdit.Embeds = typedMsg
		case *discordgo.MessageEdit:
			embeds := make([]*discordgo.MessageEmbed, 0, len(typedMsg.Embeds))
			//If there are no Embeds and string are explicitly set as null, give an error message.
			if typedMsg.Flags&discordgo.MessageFlagsIsComponentsV2 == 0 && typedMsg.Content != nil && strings.TrimSpace(*typedMsg.Content) == "" {
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
				msgResponseEdit.Content = *typedMsg.Content
			}
			msgResponseEdit.Embeds = typedMsg.Embeds
			msgResponseEdit.Components = typedMsg.Components
			msgResponseEdit.AllowedMentions = &typedMsg.AllowedMentions
		case *ComponentBuilder:
			msg, err := typedMsg.ToComplexMessageEdit()
			if err != nil {
				return "", err
			}
			msgResponseEdit.Components = msg.Components
			msgResponseEdit.Flags = msg.Flags
			msgResponseEdit.AllowedMentions = &msg.AllowedMentions
		default:
			temp := fmt.Sprint(msg)
			msgResponseEdit.Content = temp
		}

		parseMentions := []discordgo.AllowedMentionType{discordgo.AllowedMentionTypeUsers}
		repliedUser := false
		if !filterSpecialMentions {
			parseMentions = append(parseMentions, discordgo.AllowedMentionTypeRoles, discordgo.AllowedMentionTypeEveryone)
			repliedUser = true
			msgResponseEdit.AllowedMentions = &discordgo.AllowedMentions{Parse: parseMentions, RepliedUser: repliedUser}
		}

		err = common.BotSession.CreateInteractionResponse(c.CurrentFrame.Interaction.ID, c.CurrentFrame.Interaction.Token, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: msgResponseEdit,
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
