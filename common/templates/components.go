package templates

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"slices"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

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
	case discordgo.SectionComponent:
		comp := discordgo.Section{}
		err = json.Unmarshal(encoded, &comp)
		component = comp
	case discordgo.TextDisplayComponent:
		comp := discordgo.TextDisplay{}
		err = json.Unmarshal(encoded, &comp)
		component = comp
	case discordgo.ThumbnailComponent:
		comp := discordgo.Thumbnail{}
		err = json.Unmarshal(encoded, &comp)
		component = comp
	case discordgo.MediaGalleryComponent:
		comp := discordgo.MediaGallery{}
		err = json.Unmarshal(encoded, &comp)
		component = comp
	case discordgo.FileComponent:
		comp := discordgo.ComponentFile{}
		err = json.Unmarshal(encoded, &comp)
		component = comp
	case discordgo.SeparatorComponent:
		comp := discordgo.Separator{}
		err = json.Unmarshal(encoded, &comp)
		component = comp
	case discordgo.ContainerComponent:
		comp := discordgo.Container{}
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
			if *menu.MinValues < 0 || *menu.MinValues > 25 {
				return nil, errors.New("invalid min values, must be between 0 and 25")
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

func createThumbnail(values ...interface{}) (discordgo.Thumbnail, error) {
	thumb := discordgo.Thumbnail{}
	var messageSdict map[string]interface{}
	switch t := values[0].(type) {
	case SDict:
		messageSdict = t
	case *SDict:
		messageSdict = *t
	case map[string]interface{}:
		messageSdict = t
	case *discordgo.Thumbnail:
		return *t, nil
	case discordgo.Thumbnail:
		return t, nil
	default:
		dict, err := StringKeyDictionary(values...)
		if err != nil {
			return thumb, err
		}
		messageSdict = dict
	}

	convertedThumbnail := make(map[string]interface{})
	for k, v := range messageSdict {
		switch strings.ToLower(k) {
		case "media":
			convertedThumbnail[k] = createUnfurledMedia(v)
		default:
			convertedThumbnail[k] = v
		}
	}

	s, err := CreateComponent(discordgo.ThumbnailComponent, convertedThumbnail)
	if err == nil {
		thumb = s.(discordgo.Thumbnail)
	}
	return thumb, err
}

func CreateSection(values ...interface{}) (*discordgo.Section, error) {
	var messageSdict map[string]interface{}
	switch t := values[0].(type) {
	case SDict:
		messageSdict = t
	case *SDict:
		messageSdict = *t
	case map[string]interface{}:
		messageSdict = t
	case *discordgo.Section:
		return t, nil
	default:
		dict, err := StringKeyDictionary(values...)
		if err != nil {
			return nil, err
		}
		messageSdict = dict
	}

	convertedSection := make(map[string]interface{})
	for k, v := range messageSdict {
		switch strings.ToLower(k) {
		case "text":
			val, _ := indirect(reflect.ValueOf(v))
			if val.Kind() == reflect.Slice {
				textDisplays := []discordgo.SectionComponentPart{}
				for i := 0; i < val.Len(); i++ {
					display, err := CreateTextDisplay(val.Index(i).Interface())
					if err != nil {
						return nil, err
					}
					textDisplays = append(textDisplays, display)
				}
				convertedSection["components"] = textDisplays
			} else {
				display, err := CreateTextDisplay(val)
				if err != nil {
					return nil, err
				}
				convertedSection["components"] = []discordgo.SectionComponentPart{display}
			}
		case "thumbnail":
			thumb, err := createThumbnail(v)
			if err != nil {
				return nil, err
			}
			convertedSection["accessory"] = thumb
		case "button":
			button, err := CreateButton(v)
			if err != nil {
				return nil, err
			}
			convertedSection["accessory"] = button
		default:
			convertedSection[k] = v
		}
	}

	var section discordgo.Section
	s, err := CreateComponent(discordgo.SectionComponent, convertedSection)
	if err == nil {
		section = s.(discordgo.Section)
	}
	return &section, err
}

func CreateTextDisplay(value interface{}) (*discordgo.TextDisplay, error) {
	var display discordgo.TextDisplay
	d, err := CreateComponent(discordgo.TextDisplayComponent, map[string]interface{}{
		"content": ToString(value),
	})
	if err == nil {
		display = d.(discordgo.TextDisplay)
	}
	return &display, err
}

func createUnfurledMedia(value interface{}) discordgo.UnfurledMediaItem {
	return discordgo.UnfurledMediaItem{
		URL: ToString(value),
	}
}

func createGalleryItem(values ...interface{}) (item discordgo.MediaGalleryItem, err error) {
	var messageSdict map[string]interface{}
	switch t := values[0].(type) {
	case SDict:
		messageSdict = t
	case *SDict:
		messageSdict = *t
	case map[string]interface{}:
		messageSdict = t
	case discordgo.MediaGalleryItem:
		item = t
		return
	default:
		var dict SDict
		dict, err = StringKeyDictionary(values...)
		if err != nil {
			return
		}
		messageSdict = dict
	}

	for k, v := range messageSdict {
		switch strings.ToLower(k) {
		case "media":
			item.Media = createUnfurledMedia(v)
		case "description":
			item.Description = ToString(v)
		case "spoiler":
			if v == nil || v == false {
				continue
			}
			item.Spoiler = true
		}
	}

	return
}

func CreateGallery(values ...interface{}) (*discordgo.MediaGallery, error) {
	convertedGallery := &discordgo.MediaGallery{}
	val, _ := indirect(reflect.ValueOf(values))
	if val.Kind() == reflect.Slice {
		galleryItems := []discordgo.MediaGalleryItem{}
		for i := 0; i < val.Len(); i++ {
			item, err := createGalleryItem(val.Index(i).Interface())
			if err != nil {
				return nil, err
			}
			galleryItems = append(galleryItems, item)
		}
		convertedGallery.Items = galleryItems
	} else {
		item, err := createGalleryItem(val)
		if err != nil {
			return nil, err
		}
		convertedGallery.Items = []discordgo.MediaGalleryItem{item}
	}

	return convertedGallery, nil
}

func CreateFile(msgFiles *[]*discordgo.File, values ...interface{}) (*discordgo.ComponentFile, error) {
	var messageSdict map[string]interface{}
	switch t := values[0].(type) {
	case SDict:
		messageSdict = t
	case *SDict:
		messageSdict = *t
	case map[string]interface{}:
		messageSdict = t
	case *discordgo.ComponentFile:
		return t, nil
	default:
		dict, err := StringKeyDictionary(values...)
		if err != nil {
			return nil, err
		}
		messageSdict = dict
	}

	file := &discordgo.File{
		ContentType: "text/plain",
		Name:        "attachment_" + time.Now().Format("2006-01-02_15-04-05") + ".txt",
	}
	convertedFile := make(map[string]interface{})
	for k, v := range messageSdict {
		switch strings.ToLower(k) {
		case "content":
			stringFile := ToString(v)
			if len(stringFile) > 100000 {
				return nil, errors.New("file length for send message builder exceeded size limit")
			}
			var buf bytes.Buffer
			buf.WriteString(stringFile)

			file.Reader = &buf
		case "name":
			// Cut the filename to a reasonable length if it's too long
			file.Name = common.CutStringShort(ToString(v), 64) + ".txt"
		default:
			convertedFile[k] = v
		}
	}

	convertedFile["file"] = createUnfurledMedia("attachment://" + file.Name)
	*msgFiles = append((*msgFiles), file)

	var cFile discordgo.ComponentFile
	f, err := CreateComponent(discordgo.FileComponent, convertedFile)
	if err == nil {
		cFile = f.(discordgo.ComponentFile)
	}
	return &cFile, err
}

func CreateSeparator(large interface{}) *discordgo.Separator {
	spacing := discordgo.SeparatorSpacingSmall
	if large != nil && large != false {
		spacing = discordgo.SeparatorSpacingLarge
	}
	return &discordgo.Separator{
		Spacing: spacing,
	}
}

func CreateComponentArray(msgFiles *[]*discordgo.File, values ...interface{}) ([]discordgo.TopLevelComponent, error) {
	if len(values) < 1 {
		return nil, nil
	}

	if m, ok := values[0].([]discordgo.TopLevelComponent); len(values) == 1 && ok {
		return m, nil
	}

	compBuilder, err := CreateComponentBuilder(values...)
	if err != nil {
		return nil, err
	}

	components := []discordgo.TopLevelComponent{}

	for i, key := range compBuilder.Components {
		val := compBuilder.Values[i]

		switch strings.ToLower(key) {
		case "interactive_components":
			if val == nil {
				continue
			}
			v, _ := indirect(reflect.ValueOf(val))
			if v.Kind() == reflect.Slice {
				components, err = distributeComponentsIntoActionsRows(v)
				if err != nil {
					return nil, err
				}
			} else {
				var component discordgo.InteractiveComponent
				switch comp := val.(type) {
				case *discordgo.SelectMenu:
					component = comp
				case *discordgo.Button:
					component = comp
				default:
					return nil, errors.New("invalid component passed to send message builder")
				}
				components = append(components, &discordgo.ActionsRow{Components: []discordgo.InteractiveComponent{component}})
			}
		case "buttons":
			if val == nil {
				continue
			}
			v, _ := indirect(reflect.ValueOf(val))
			if v.Kind() == reflect.Slice {
				buttons := []*discordgo.Button{}
				const maxButtons = 25 // Discord limitation
				for i := 0; i < v.Len() && i < maxButtons; i++ {
					button, err := CreateButton(v.Index(i).Interface())
					if err != nil {
						return nil, err
					}
					buttons = append(buttons, button)
				}
				comps, err := distributeComponentsIntoActionsRows(reflect.ValueOf(buttons))
				if err != nil {
					return nil, err
				}
				components = append(components, comps...)
			} else {
				button, err := CreateButton(val)
				if err != nil {
					return nil, err
				}
				if button.Style == discordgo.LinkButton {
					button.CustomID = ""
				}
				components = append(components, &discordgo.ActionsRow{Components: []discordgo.InteractiveComponent{button}})
			}
		case "menus":
			if val == nil {
				continue
			}
			v, _ := indirect(reflect.ValueOf(val))
			if v.Kind() == reflect.Slice {
				menus := []*discordgo.SelectMenu{}
				const maxMenus = 5 // Discord limitation
				for i := 0; i < v.Len() && i < maxMenus; i++ {
					menu, err := CreateSelectMenu(v.Index(i).Interface())
					if err != nil {
						return nil, err
					}
					menus = append(menus, menu)
				}
				comps, err := distributeComponentsIntoActionsRows(reflect.ValueOf(menus))
				if err != nil {
					return nil, err
				}
				components = append(components, comps...)
			} else {
				menu, err := CreateSelectMenu(val)
				if err != nil {
					return nil, err
				}
				components = append(components, &discordgo.ActionsRow{Components: []discordgo.InteractiveComponent{menu}})
			}
		case "section":
			if val == nil {
				continue
			}
			v, _ := indirect(reflect.ValueOf(val))
			if v.Kind() == reflect.Slice {
				sections := []discordgo.TopLevelComponent{}
				for i := 0; i < v.Len(); i++ {
					section, err := CreateSection(v.Index(i).Interface())
					if err != nil {
						return nil, err
					}
					sections = append(sections, section)
				}
				if err != nil {
					return nil, err
				}
				components = append(components, sections...)
			} else {
				section, err := CreateSection(val)
				if err != nil {
					return nil, err
				}
				components = append(components, section)
			}
		case "text":
			if val == nil {
				continue
			}
			v, _ := indirect(reflect.ValueOf(val))
			if v.Kind() == reflect.Slice {
				displays := []discordgo.TopLevelComponent{}
				for i := 0; i < v.Len(); i++ {
					display, err := CreateTextDisplay(v.Index(i).Interface())
					if err != nil {
						return nil, err
					}
					displays = append(displays, display)
				}
				if err != nil {
					return nil, err
				}
				components = append(components, displays...)
			} else {
				display, err := CreateTextDisplay(val)
				if err != nil {
					return nil, err
				}
				components = append(components, display)
			}
		case "gallery":
			if val == nil {
				continue
			}
			gallery, err := CreateGallery(val)
			if err != nil {
				return nil, err
			}
			components = append(components, gallery)
		case "file":
			if val == nil || msgFiles == nil {
				continue
			}
			v, _ := indirect(reflect.ValueOf(val))
			if v.Kind() == reflect.Slice {
				files := []discordgo.TopLevelComponent{}
				for i := 0; i < v.Len(); i++ {
					file, err := CreateFile(msgFiles, v.Index(i).Interface())
					if err != nil {
						return nil, err
					}
					files = append(files, file)
				}
				if err != nil {
					return nil, err
				}
				components = append(components, files...)
			} else {
				file, err := CreateFile(msgFiles, val)
				if err != nil {
					return nil, err
				}
				components = append(components, file)
			}
		case "separator":
			if val == nil {
				continue
			}
			v, _ := indirect(reflect.ValueOf(val))
			if v.Kind() == reflect.Slice {
				separators := []discordgo.TopLevelComponent{}
				for i := 0; i < v.Len(); i++ {
					separator := CreateSeparator(v.Index(i).Interface())
					separators = append(separators, separator)
				}
				if err != nil {
					return nil, err
				}
				components = append(components, separators...)
			} else {
				separator := CreateSeparator(val)
				components = append(components, separator)
			}
		case "container":
			if val == nil {
				continue
			}
			v, _ := indirect(reflect.ValueOf(val))
			if v.Kind() == reflect.Slice {
				containers := []discordgo.TopLevelComponent{}
				for i := 0; i < v.Len(); i++ {
					container, err := CreateContainer(msgFiles, v.Index(i).Interface())
					if err != nil {
						return nil, err
					}
					containers = append(containers, container)
				}
				if err != nil {
					return nil, err
				}
				components = append(components, containers...)
			} else {
				file, err := CreateContainer(msgFiles, val)
				if err != nil {
					return nil, err
				}
				components = append(components, file)
			}
		default:
			return nil, errors.New(`invalid key "` + key + `" passed to component array builder.`)
		}

	}

	return components, nil
}

func CreateContainer(msgFiles *[]*discordgo.File, values ...interface{}) (*discordgo.Container, error) {
	var messageSdict map[string]interface{}
	switch t := values[0].(type) {
	case SDict:
		messageSdict = t
	case *SDict:
		messageSdict = *t
	case map[string]interface{}:
		messageSdict = t
	case *discordgo.Container:
		return t, nil
	default:
		dict, err := StringKeyDictionary(values...)
		if err != nil {
			return nil, err
		}
		messageSdict = dict
	}

	convertedContainer := discordgo.Container{}
	for k, v := range messageSdict {
		switch strings.ToLower(k) {
		case "components":
			components, err := CreateComponentArray(msgFiles, v)
			if err != nil {
				return nil, err
			}
			convertedContainer.Components = append(convertedContainer.Components, components...)
		case "color":
			convertedContainer.AccentColor = tmplToInt(v)
		case "spoiler":
			if v == nil || v == false {
				continue
			}
			convertedContainer.Spoiler = true
		default:
			return nil, errors.New(`invalid key "` + k + `" passed to container builder.`)
		}
	}

	return &convertedContainer, nil
}

func distributeComponentsIntoActionsRows(components reflect.Value) (returnComponents []discordgo.TopLevelComponent, err error) {
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
				var component discordgo.InteractiveComponent
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
		currentComponents := make([]discordgo.InteractiveComponent, 0)
		for i := 0; i < components.Len() && i < maxRows*maxComponents; i++ {
			var component discordgo.InteractiveComponent
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

			availableSpace := 5 - len(currentComponents)
			if !isMenu && availableSpace > 0 || isMenu && availableSpace == 5 {
				currentComponents = append(currentComponents, component)
			} else {
				returnComponents = append(returnComponents, &discordgo.ActionsRow{Components: slices.Clone(currentComponents)})
				currentComponents = []discordgo.InteractiveComponent{component}
			}

			// if it's a menu, the row is full now, append and start a new one
			if isMenu {
				returnComponents = append(returnComponents, &discordgo.ActionsRow{Components: []discordgo.InteractiveComponent{component}})
				currentComponents = []discordgo.InteractiveComponent{}
			}

			if i == components.Len()-1 && len(currentComponents) > 0 { // if we're at the end, append the last row
				returnComponents = append(returnComponents, &discordgo.ActionsRow{Components: slices.Clone(currentComponents)})
			}
		}
	}
	return
}

// validateCustomID sets a unique custom ID based on componentIndex if needed
// and returns an error if id is already in used
func validateCustomID(id string, used map[string]bool) (string, error) {
	if id == "" {
		id = fmt.Sprint(len(used))
	}

	if !strings.HasPrefix(id, "templates-") {
		id = fmt.Sprint("templates-", id)
	}

	const maxCIDLength = 100 // discord limitation
	if len(id) > maxCIDLength {
		return "", errors.New("custom id too long (max 90 chars)") // maxCIDLength - len("templates-")
	}

	if used == nil {
		return id, nil
	}

	if _, ok := used[id]; ok {
		return "", errors.New("duplicate custom ids used")
	}
	return id, nil
}

// validateCustomIDs sets unique custom IDs for any component in the action
// rows provided in the slice, sets any link buttons' ids to an empty string,
// and returns an error if duplicate custom ids are used.
func validateTopLevelComponentsCustomIDs(rows []discordgo.TopLevelComponent, used map[string]bool) error {
	if used == nil {
		used = make(map[string]bool)
	}
	for rowIdx := 0; rowIdx < len(rows); rowIdx++ {
		var rowComps []discordgo.InteractiveComponent
		switch r := rows[rowIdx].(type) {
		case *discordgo.ActionsRow:
			rowComps = r.Components
		case *discordgo.Section:
			if c, ok := r.Accessory.(discordgo.InteractiveComponent); ok {
				rowComps = []discordgo.InteractiveComponent{c}
			} else {
				continue
			}
		case *discordgo.Container:
			err := validateTopLevelComponentsCustomIDs(r.Components, used)
			if err != nil {
				return err
			}
			continue
		default:
			continue
		}
		for compIdx := 0; compIdx < len(rowComps); compIdx++ {
			var err error
			switch c := (rowComps)[compIdx].(type) {
			case *discordgo.Button:
				if c.Style == discordgo.LinkButton {
					c.CustomID = ""
					continue
				}
				c.CustomID, err = validateCustomID(c.CustomID, used)
				used[c.CustomID] = true
			case *discordgo.SelectMenu:
				c.CustomID, err = validateCustomID(c.CustomID, used)
				used[c.CustomID] = true
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
}
