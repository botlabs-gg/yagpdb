package discordgo

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// ComponentType is type of component.
type ComponentType uint

// MessageComponent types.
const (
	ActionsRowComponent            ComponentType = 1
	ButtonComponent                ComponentType = 2
	SelectMenuComponent            ComponentType = 3
	TextInputComponent             ComponentType = 4
	UserSelectMenuComponent        ComponentType = 5
	RoleSelectMenuComponent        ComponentType = 6
	MentionableSelectMenuComponent ComponentType = 7
	ChannelSelectMenuComponent     ComponentType = 8
	SectionComponent               ComponentType = 9
	TextDisplayComponent           ComponentType = 10
	ThumbnailComponent             ComponentType = 11
	MediaGalleryComponent          ComponentType = 12
	FileComponent                  ComponentType = 13
	SeparatorComponent             ComponentType = 14
	ActivityContentComponent       ComponentType = 16
	ContainerComponent             ComponentType = 17
	LabelComponent                 ComponentType = 18
)

// MessageComponent is a base interface for all message components.
type MessageComponent interface {
	json.Marshaler
	Type() ComponentType
}

type unmarshalableMessageComponent struct {
	MessageComponent
}

// UnmarshalJSON is a helper function to unmarshal MessageComponent object.
func (umc *unmarshalableMessageComponent) UnmarshalJSON(src []byte) error {
	var v struct {
		Type ComponentType `json:"type"`
	}
	err := json.Unmarshal(src, &v)
	if err != nil {
		return err
	}

	switch v.Type {
	case ActivityContentComponent:
		umc.MessageComponent = &ActivityContent{}
	case ActionsRowComponent:
		umc.MessageComponent = &ActionsRow{}
	case ButtonComponent:
		umc.MessageComponent = &Button{}
	case SelectMenuComponent, ChannelSelectMenuComponent, UserSelectMenuComponent,
		RoleSelectMenuComponent, MentionableSelectMenuComponent:
		umc.MessageComponent = &SelectMenu{}
	case TextInputComponent:
		umc.MessageComponent = &TextInput{}
	case SectionComponent:
		umc.MessageComponent = &Section{}
	case TextDisplayComponent:
		umc.MessageComponent = &TextDisplay{}
	case ThumbnailComponent:
		umc.MessageComponent = &Thumbnail{}
	case MediaGalleryComponent:
		umc.MessageComponent = &MediaGallery{}
	case FileComponent:
		umc.MessageComponent = &ComponentFile{}
	case SeparatorComponent:
		umc.MessageComponent = &Separator{}
	case ContainerComponent:
		umc.MessageComponent = &Container{}
	case LabelComponent:
		umc.MessageComponent = &Label{}
	default:
		return fmt.Errorf("unknown component type: %d", v.Type)
	}
	return json.Unmarshal(src, umc.MessageComponent)
}

// MessageComponentFromJSON is a helper function for unmarshaling message components
func MessageComponentFromJSON(b []byte) (MessageComponent, error) {
	var u unmarshalableMessageComponent
	err := u.UnmarshalJSON(b)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal into MessageComponent: %w", err)
	}
	return u.MessageComponent, nil
}

// TopLevelComponent is an interface for message components which can be used on the top level of a message.
type TopLevelComponent interface {
	MessageComponent
	IsTopLevel() bool
	IsModalSupported() bool
}

// InteractiveComponent is an interface for message components which can be interacted with.
type InteractiveComponent interface {
	MessageComponent
	IsInteractive() bool
	IsAllowedInLabel() bool
}

type ActivityContentInventoryTrait struct {
	Type            int  `json:"type"`
	DurationSeconds int  `json:"duration_seconds"`
	FirstTime       bool `json:"first_time"`
}

type ActivityContentInventorySignature struct {
	Version   int    `json:"version"`
	Signature string `json:"signature"`
	Kid       string `json:"kid"`
}

type ActivityContentInventoryExtra struct {
	Type          int    `json:"type"`
	Platform      int    `json:"platform"`
	GameName      string `json:"game_name"`
	ApplicationID string `json:"application_id"`
}

type ActivityContentInventoryEntry struct {
	ID           int                               `json:"id"`
	Traits       []ActivityContentInventoryTrait   `json:"traits"`
	StartedAt    time.Time                         `json:"started_at"`
	Signature    ActivityContentInventorySignature `json:"signature"`
	Participants []string                          `json:"participants"`
	EndedAt      time.Time                         `json:"ended_at"`
	ContentType  int                               `json:"content_type"`
	AuthorType   int                               `json:"author_type"`
	AuthorID     string                            `json:"author_id"`
	Extra        ActivityContentInventoryExtra     `json:"extra"`
}

type ActivityContent struct {
	InventoryEntry ActivityContentInventoryEntry `json:"inventory_entry"`
}

// Type is a method to get the type of a component.
func (r ActivityContent) Type() ComponentType {
	return ActionsRowComponent
}

// IsTopLevel is a method to assert the component as top level.
func (ActivityContent) IsTopLevel() bool {
	return true
}

// IsModalSupported is a method to assert the component as modal supported.
func (ActivityContent) IsModalSupported() bool {
	return true
}

// MarshalJSON is a method for marshaling ActionsRow to a JSON object.
func (r ActivityContent) MarshalJSON() ([]byte, error) {
	type activityContent ActivityContent
	return json.Marshal(struct {
		activityContent
		Type ComponentType `json:"type"`
	}{
		activityContent: activityContent(r),
		Type:            r.Type(),
	})
}

// UnmarshalJSON is a helper function to unmarshal ActivityContent
func (r *ActivityContent) UnmarshalJSON(data []byte) error {
	var v struct {
		ActivityContent ActivityContent `json:"components"`
	}
	err := json.Unmarshal(data, &v)
	if err != nil {
		return err
	}
	*r = v.ActivityContent
	return err
}

// ActionsRow is a container for interactive components within one row.
type ActionsRow struct {
	Components []InteractiveComponent `json:"components"`
}

// MarshalJSON is a method for marshaling ActionsRow to a JSON object.
func (r ActionsRow) MarshalJSON() ([]byte, error) {
	type actionsRow ActionsRow

	return json.Marshal(struct {
		actionsRow
		Type ComponentType `json:"type"`
	}{
		actionsRow: actionsRow(r),
		Type:       r.Type(),
	})
}

// UnmarshalJSON is a helper function to unmarshal Actions Row.
func (r *ActionsRow) UnmarshalJSON(data []byte) error {
	var v struct {
		RawComponents []unmarshalableMessageComponent `json:"components"`
	}
	err := json.Unmarshal(data, &v)
	if err != nil {
		return err
	}
	r.Components = make([]InteractiveComponent, len(v.RawComponents))
	for i, v := range v.RawComponents {
		var ok bool
		comp := v.MessageComponent
		r.Components[i], ok = comp.(InteractiveComponent)
		if !ok {
			return errors.New("non interactive component passed to actions row")
		}
	}

	return err
}

// Type is a method to get the type of a component.
func (r ActionsRow) Type() ComponentType {
	return ActionsRowComponent
}

// IsTopLevel is a method to assert the component as top level.
func (ActionsRow) IsTopLevel() bool {
	return true
}

// IsModalSupported is a method to assert the component as modal supported.
func (ActionsRow) IsModalSupported() bool {
	return false
}

// ButtonStyle is style of button.
type ButtonStyle uint

// Button styles.
const (
	// PrimaryButton is a button with blurple color.
	PrimaryButton ButtonStyle = 1
	// SecondaryButton is a button with grey color.
	SecondaryButton ButtonStyle = 2
	// SuccessButton is a button with green color.
	SuccessButton ButtonStyle = 3
	// DangerButton is a button with red color.
	DangerButton ButtonStyle = 4
	// LinkButton is a special type of button which navigates to a URL. Has grey color.
	LinkButton ButtonStyle = 5
)

// ComponentEmoji represents button emoji, if it does have one.
type ComponentEmoji struct {
	Name     string `json:"name,omitempty"`
	ID       int64  `json:"id,string,omitempty"`
	Animated bool   `json:"animated,omitempty"`
}

// Button represents button component.
type Button struct {
	Label    string          `json:"label"`
	Style    ButtonStyle     `json:"style"`
	Disabled bool            `json:"disabled"`
	Emoji    *ComponentEmoji `json:"emoji,omitempty"`

	// NOTE: Only button with LinkButton style can have link. Also, URL is mutually exclusive with CustomID.
	URL      string `json:"url,omitempty"`
	CustomID string `json:"custom_id,omitempty"`
}

// MarshalJSON is a method for marshaling Button to a JSON object.
func (b Button) MarshalJSON() ([]byte, error) {
	type button Button

	if b.Style == 0 {
		b.Style = PrimaryButton
	}

	return json.Marshal(struct {
		button
		Type ComponentType `json:"type"`
	}{
		button: button(b),
		Type:   b.Type(),
	})
}

// Type is a method to get the type of a component.
func (Button) Type() ComponentType {
	return ButtonComponent
}

// IsInteractive is a method to assert the component as interactive.
func (Button) IsInteractive() bool {
	return true
}

func (Button) IsAllowedInLabel() bool {
	return false
}

// IsAccessory is a method to assert the component as an accessory.
func (Button) IsAccessory() bool {
	return true
}

// SelectMenuOption represents an option for a select menu.
type SelectMenuOption struct {
	Label       string          `json:"label,omitempty"`
	Value       string          `json:"value"`
	Description string          `json:"description"`
	Emoji       *ComponentEmoji `json:"emoji,omitempty"`
	// Determines whenever option is selected by default or not.
	Default bool `json:"default"`
}

// SelectMenuDefaultValueType represents the type of an entity selected by default in auto-populated select menus.
type SelectMenuDefaultValueType string

// SelectMenuDefaultValue types.
const (
	SelectMenuDefaultValueUser    SelectMenuDefaultValueType = "user"
	SelectMenuDefaultValueRole    SelectMenuDefaultValueType = "role"
	SelectMenuDefaultValueChannel SelectMenuDefaultValueType = "channel"
)

// SelectMenuDefaultValue represents an entity selected by default in auto-populated select menus.
type SelectMenuDefaultValue struct {
	// ID of the entity.
	ID string `json:"id"`
	// Type of the entity.
	Type SelectMenuDefaultValueType `json:"type"`
}

// SelectMenuType represents select menu type.
type SelectMenuType ComponentType

// SelectMenu types.
const (
	StringSelectMenu      = SelectMenuType(SelectMenuComponent)
	UserSelectMenu        = SelectMenuType(UserSelectMenuComponent)
	RoleSelectMenu        = SelectMenuType(RoleSelectMenuComponent)
	MentionableSelectMenu = SelectMenuType(MentionableSelectMenuComponent)
	ChannelSelectMenu     = SelectMenuType(ChannelSelectMenuComponent)
)

// SelectMenu represents select menu component.
type SelectMenu struct {
	// Type of the select menu.
	MenuType SelectMenuType `json:"type,omitempty"`
	// CustomID is a developer-defined identifier for the select menu.
	CustomID string `json:"custom_id,omitempty"`
	// The text which will be shown in the menu if there's no default options or all options was deselected and component was closed.
	Placeholder string `json:"placeholder"`
	// This value determines the minimal amount of selected items in the menu.
	MinValues *int `json:"min_values,omitempty"`
	// This value determines the maximal amount of selected items in the menu.
	// If MaxValues or MinValues are greater than one then the user can select multiple items in the component.
	MaxValues int `json:"max_values,omitempty"`
	// List of default values for auto-populated select menus.
	// NOTE: Number of entries should be in the range defined by MinValues and MaxValues.
	DefaultValues []SelectMenuDefaultValue `json:"default_values,omitempty"`

	// Values is a list of values selected by the user, only filled when the select menu is submitted.
	Values []string `json:"values,omitempty"`

	Options  []SelectMenuOption `json:"options,omitempty"`
	Disabled bool               `json:"disabled"`
	Required bool               `json:"required"`

	// NOTE: Can only be used in SelectMenu with Channel menu type.
	ChannelTypes []ChannelType `json:"channel_types,omitempty"`
}

// Type is a method to get the type of a component.
func (s SelectMenu) Type() ComponentType {
	if s.MenuType != 0 {
		return ComponentType(s.MenuType)
	}
	return SelectMenuComponent
}

func (SelectMenu) IsAllowedInLabel() bool {
	return true
}

// MarshalJSON is a method for marshaling SelectMenu to a JSON object.
func (s SelectMenu) MarshalJSON() ([]byte, error) {
	type selectMenu SelectMenu

	return json.Marshal(struct {
		selectMenu
		Type ComponentType `json:"type"`
	}{
		selectMenu: selectMenu(s),
		Type:       s.Type(),
	})
}

// IsInteractive is a method to assert the component as interactive.
func (SelectMenu) IsInteractive() bool {
	return true
}

// TextInput represents text input component.
type TextInput struct {
	CustomID    string         `json:"custom_id"`
	Label       string         `json:"label,omitempty"`
	Style       TextInputStyle `json:"style"`
	Placeholder string         `json:"placeholder,omitempty"`
	Value       string         `json:"value,omitempty"`
	Required    bool           `json:"required"`
	MinLength   int            `json:"min_length,omitempty"`
	MaxLength   int            `json:"max_length,omitempty"`
}

// Type is a method to get the type of a component.
func (TextInput) Type() ComponentType {
	return TextInputComponent
}

func (TextInput) IsAllowedInLabel() bool {
	return true
}

// MarshalJSON is a method for marshaling TextInput to a JSON object.
func (m TextInput) MarshalJSON() ([]byte, error) {
	type inputText TextInput

	return json.Marshal(struct {
		inputText
		Type ComponentType `json:"type"`
	}{
		inputText: inputText(m),
		Type:      m.Type(),
	})
}

// TextInputStyle is style of text in TextInput component.
type TextInputStyle uint

// Text styles
const (
	TextInputShort     TextInputStyle = 1
	TextInputParagraph TextInputStyle = 2
)

// IsInteractive is a method to assert the component as interactive.
func (TextInput) IsInteractive() bool {
	return true
}

// SectionComponentPart is an interface for message components which can be used as a component in sections.
type SectionComponentPart interface {
	MessageComponent
	IsSectionComponent() bool
}

// AccessoryComponent is an interface for message components which can be used as an accessory in sections.
type AccessoryComponent interface {
	MessageComponent
	IsAccessory() bool
}

// Section is a top-level layout component that allows you to join text contextually with an accessory.
type Section struct {
	ID         int                    `json:"id,omitempty"`
	Components []SectionComponentPart `json:"components"`
	Accessory  AccessoryComponent     `json:"accessory"`
}

// MarshalJSON is a method for marshaling Section to a JSON object.
func (s Section) MarshalJSON() ([]byte, error) {
	type section Section

	return json.Marshal(struct {
		section
		Type ComponentType `json:"type"`
	}{
		section: section(s),
		Type:    s.Type(),
	})
}

// UnmarshalJSON is a helper function to unmarshal Section.
func (s *Section) UnmarshalJSON(data []byte) error {
	var v struct {
		RawComponents []unmarshalableMessageComponent `json:"components"`
		RawAccessory  *unmarshalableMessageComponent  `json:"accessory"`
	}
	err := json.Unmarshal(data, &v)
	if err != nil {
		return err
	}
	var ok bool
	s.Components = make([]SectionComponentPart, len(v.RawComponents))
	for i, v := range v.RawComponents {
		comp := v.MessageComponent
		s.Components[i], ok = comp.(SectionComponentPart)
		if !ok {
			return errors.New("non text display passed to section component")
		}
	}
	if v.RawAccessory == nil {
		return errors.New("missing accessory component in section")
	}

	accessory := v.RawAccessory.MessageComponent
	s.Accessory, ok = accessory.(AccessoryComponent)
	if !ok {
		return errors.New("non accessory component passed to section component")
	}

	return err
}

// Type is a method to get the type of a component.
func (s Section) Type() ComponentType {
	return SectionComponent
}

// IsTopLevel is a method to assert the component as top level.
func (Section) IsTopLevel() bool {
	return true
}

func (Section) IsModalSupported() bool {
	return false
}

func GetTextDisplayContent(component TopLevelComponent) (contents []string) {
	switch typed := component.(type) {
	case ActionsRow:
		return
	case ActivityContent:
		return
	case Section:
		for _, c := range typed.Components {
			comp, ok := c.(TopLevelComponent)
			if ok {
				contents = append(contents, GetTextDisplayContent(comp)...)
			}
		}
	case Container:
		for _, c := range typed.Components {
			contents = append(contents, GetTextDisplayContent(c)...)
		}
	case TextDisplay:
		contents = append(contents, typed.Content)
	}

	return
}

// Label is a top-level layout component.
// Labels wrap modal components with text as a label and optional description.
type Label struct {
	// Unique identifier for the component; auto populated through increment if not provided.
	ID          int                  `json:"id,omitempty"`
	Label       string               `json:"label"`
	Description string               `json:"description,omitempty"`
	Component   InteractiveComponent `json:"component"`
}

// Type is a method to get the type of a component.

func (Label) Type() ComponentType {
	return LabelComponent
}

// UnmarshalJSON is a method for unmarshaling Label from JSON

func (l *Label) UnmarshalJSON(data []byte) error {
	type label Label
	var v struct {
		label
		RawComponent unmarshalableMessageComponent `json:"component"`
	}

	err := json.Unmarshal(data, &v)
	if err != nil {
		return err
	}

	*l = Label(v.label)
	l.Component = v.RawComponent.MessageComponent.(InteractiveComponent)
	return nil
}

// MarshalJSON is a method for marshaling Label to a JSON object.
func (l Label) MarshalJSON() ([]byte, error) {
	type label Label

	return json.Marshal(struct {
		label
		Type ComponentType `json:"type"`
	}{
		label: label(l),
		Type:  l.Type(),
	})
}

func (Label) IsTopLevel() bool {
	return true
}

func (Label) IsModalSupported() bool {
	return true
}

// TextDisplay represents text display component.
type TextDisplay struct {
	ID      int    `json:"id,omitempty"`
	Content string `json:"content"`
}

// MarshalJSON is a method for marshaling TextDisplay to a JSON object.
func (d TextDisplay) MarshalJSON() ([]byte, error) {
	type textDisplay TextDisplay

	return json.Marshal(struct {
		textDisplay
		Type ComponentType `json:"type"`
	}{
		textDisplay: textDisplay(d),
		Type:        d.Type(),
	})
}

// Type is a method to get the type of a component.
func (TextDisplay) Type() ComponentType {
	return TextDisplayComponent
}

func (TextDisplay) IsModalSupported() bool {
	return true
}

// IsTopLevel is a method to assert the component as top level.
func (TextDisplay) IsTopLevel() bool {
	return true
}

// IsSectionComponent is a method to assert the component as a section component.
func (TextDisplay) IsSectionComponent() bool {
	return true
}

type UnfurledMediaItem struct {
	URL         string `json:"url"`                    // Supports arbitrary urls and attachment://<filename> references
	ProxyURL    string `json:"proxy_url,omitempty"`    // The proxied url of the media item. This field is ignored and provided by the API as part of the response
	Height      int    `json:"height,omitempty"`       // The height of the media item. This field is ignored and provided by the API as part of the response
	Width       int    `json:"width,omitempty"`        // The width of the media item. This field is ignored and provided by the API as part of the response
	ContentType string `json:"content_type,omitempty"` // The media type of the content. This field is ignored and provided by the API as part of the response
}

// Thumbnail represents thumbnail component.
type Thumbnail struct {
	ID          int               `json:"id,omitempty"`
	Media       UnfurledMediaItem `json:"media"`
	Description string            `json:"description,omitempty"`
	Spoiler     bool              `json:"spoiler,omitempty"`
}

// MarshalJSON is a method for marshaling Thumbnail to a JSON object.
func (t Thumbnail) MarshalJSON() ([]byte, error) {
	type thumbnail Thumbnail

	return json.Marshal(struct {
		thumbnail
		Type ComponentType `json:"type"`
	}{
		thumbnail: thumbnail(t),
		Type:      t.Type(),
	})
}

// Type is a method to get the type of a component.
func (Thumbnail) Type() ComponentType {
	return ThumbnailComponent
}

// IsAccessory is a method to assert the component as an accessory.
func (Thumbnail) IsAccessory() bool {
	return true
}

type MediaGalleryItem struct {
	Media       UnfurledMediaItem `json:"media"`
	Description string            `json:"description,omitempty"`
	Spoiler     bool              `json:"spoiler,omitempty"`
}

// MediaGallery represents media gallery component.
type MediaGallery struct {
	ID    int                `json:"id,omitempty"`
	Items []MediaGalleryItem `json:"items"`
}

// MarshalJSON is a method for marshaling MediaGallery to a JSON object.
func (t MediaGallery) MarshalJSON() ([]byte, error) {
	type mediaGallery MediaGallery

	return json.Marshal(struct {
		mediaGallery
		Type ComponentType `json:"type"`
	}{
		mediaGallery: mediaGallery(t),
		Type:         t.Type(),
	})
}

// Type is a method to get the type of a component.
func (MediaGallery) Type() ComponentType {
	return MediaGalleryComponent
}

// IsTopLevel is a method to assert the component as top level.
func (MediaGallery) IsTopLevel() bool {
	return true
}

func (MediaGallery) IsModalSupported() bool {
	return false
}

// ComponentFile represents file component.
type ComponentFile struct {
	ID      int               `json:"id,omitempty"`
	File    UnfurledMediaItem `json:"file"`
	Spoiler bool              `json:"spoiler,omitempty"`
}

// MarshalJSON is a method for marshaling ComponentFile to a JSON object.
func (t ComponentFile) MarshalJSON() ([]byte, error) {
	type file ComponentFile

	return json.Marshal(struct {
		file
		Type ComponentType `json:"type"`
	}{
		file: file(t),
		Type: t.Type(),
	})
}

// Type is a method to get the type of a component.
func (ComponentFile) Type() ComponentType {
	return FileComponent
}

// IsTopLevel is a method to assert the component as top level.
func (ComponentFile) IsTopLevel() bool {
	return true
}

func (ComponentFile) IsModalSupported() bool {
	return false
}

// SeparatorSpacing is the size of padding in a separator
type SeparatorSpacing int

const (
	SeparatorSpacingSmall SeparatorSpacing = 1
	SeparatorSpacingLarge SeparatorSpacing = 2
)

// Separator represents separator component.
type Separator struct {
	ID      int              `json:"id,omitempty"`
	Divider bool             `json:"divider,omitempty"`
	Spacing SeparatorSpacing `json:"spacing,omitempty"`
}

// MarshalJSON is a method for marshaling Separator to a JSON object.
func (t Separator) MarshalJSON() ([]byte, error) {
	type separator Separator

	return json.Marshal(struct {
		separator
		Type ComponentType `json:"type"`
	}{
		separator: separator(t),
		Type:      t.Type(),
	})
}

// Type is a method to get the type of a component.
func (Separator) Type() ComponentType {
	return SeparatorComponent
}

// IsTopLevel is a method to assert the component as top level.
func (Separator) IsTopLevel() bool {
	return true
}

func (Separator) IsModalSupported() bool {
	return false
}

// Container is a top-level layout component that allows you to join text contextually with an accessory.
type Container struct {
	ID          int                 `json:"id,omitempty"`
	Components  []TopLevelComponent `json:"components"`
	AccentColor int                 `json:"accent_color,omitempty"`
	Spoiler     bool                `json:"spoiler,omitempty"`
}

// MarshalJSON is a method for marshaling Container to a JSON object.
func (s Container) MarshalJSON() ([]byte, error) {
	type section Container

	return json.Marshal(struct {
		section
		Type ComponentType `json:"type"`
	}{
		section: section(s),
		Type:    s.Type(),
	})
}

// UnmarshalJSON is a helper function to unmarshal Container.
func (c *Container) UnmarshalJSON(data []byte) error {
	var v struct {
		RawComponents []unmarshalableMessageComponent `json:"components"`
	}
	err := json.Unmarshal(data, &v)
	if err != nil {
		return err
	}
	var ok bool
	c.Components = make([]TopLevelComponent, len(v.RawComponents))
	for i, v := range v.RawComponents {
		comp := v.MessageComponent
		c.Components[i], ok = comp.(TopLevelComponent)
		if !ok {
			return errors.New("non top level component passed to container component unmarshaller")
		}
	}

	return err
}

// Type is a method to get the type of a component.
func (c Container) Type() ComponentType {
	return ContainerComponent
}

// IsTopLevel is a method to assert the component as top level.
func (Container) IsTopLevel() bool {
	return true
}

func (Container) IsModalSupported() bool {
	return false
}
