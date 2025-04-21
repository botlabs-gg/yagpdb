package notes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/notes/models"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

//go:generate sqlboiler --no-hooks --add-soft-deletes psql

type Plugin struct{}

func RegisterPlugin() {
	p := &Plugin{}
	common.RegisterPlugin(p)

	common.InitSchemas("notes", DBSchemas...)
}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "Notes",
		SysName:  "notes",
		Category: common.PluginCategoryModeration,
	}
}

const maxNotesPerUser = 3
const maxCharactersPerNote = 256

var invalidSelectionErr = errors.New("The notes system encountered an error selecting the note, please contact support:")

// at least one of the following is required to use notes
var requiredPerms = []int64{discordgo.PermissionManageMessages, discordgo.PermissionModerateMembers, discordgo.PermissionKickMembers, discordgo.PermissionBanMembers, discordgo.PermissionManageGuild}

type Note struct {
	noteLines []string
	signature string

	authorID  int64
	updatedAt time.Time
}

func (n *Note) formatNote() string {
	if len(n.noteLines) < 1 || n.noteLines[0] == "" || n.authorID == 0 {
		return ""
	}

	return fmt.Sprintf("%s\n\n%d;%d", strings.Join(n.noteLines, "\n"), n.updatedAt.Unix(), n.authorID)
}

type parsedNotes struct {
	guildID, userID int64
	notes           []*Note
}

func parseRawNote(guildID int64, rawNote string) (parsed *Note) {
	parsed = &Note{}

	lines := strings.Split(rawNote, "\n")
	if len(lines) < 3 {
		return &Note{
			noteLines: lines,
		}
	}

	rawSignature := lines[len(lines)-1]

	parsed.noteLines = lines[:len(lines)-2]
	parsed.fillData(guildID, rawSignature)

	return
}

func (n *Note) fillData(guildID int64, rawSignature string) string {
	args := strings.Split(rawSignature, ";")
	timestamp, _ := strconv.ParseInt(args[0], 10, 64)
	n.updatedAt = time.Unix(timestamp, 0)
	n.authorID, _ = strconv.ParseInt(args[1], 10, 64)

	name := args[1]
	maybeMember, _ := bot.GetMember(guildID, n.authorID)
	if maybeMember != nil {
		name = maybeMember.User.String()
	}
	n.signature = name

	return ""
}

var cmds = []*commands.YAGCommand{
	{
		Name:                "notes",
		Default:             true,
		DefaultEnabled:      true,
		CmdCategory:         commands.CategoryModeration,
		HideFromHelp:        true,
		RequireDiscordPerms: requiredPerms,
		CommandType:         discordgo.UserApplicationCommand,
		IsResponseEphemeral: true,
		RunFunc: func(data *dcmd.Data) (interface{}, error) {
			notes, err := getNotes(data.Context(), data.GuildData.GS.ID, data.SlashCommandTriggerData.TargetID)
			if err != nil {
				return nil, err
			}
			return notes.createMessage(), nil
		},
	},
}

func getNotes(ctx context.Context, guildID, userID int64) (*parsedNotes, error) {
	entry, err := models.Notes(qm.Where("guild_id = ? AND member_id = ?", guildID, userID)).OneG(ctx)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	var notes []*Note
	if entry != nil {
		for i := 0; i < maxNotesPerUser && i < len(entry.Notes); i++ {
			notes = append(notes, parseRawNote(guildID, entry.Notes[i]))
		}
	}

	userNotes := &parsedNotes{
		guildID: guildID,
		userID:  userID,
		notes:   notes,
	}

	return userNotes, nil
}

func (p *parsedNotes) add(note string, author *discordgo.User) error {
	var index *int
	for i, n := range p.notes {
		if len(n.noteLines) < 1 || n.noteLines[0] == "" {
			index = &i
			break
		}
	}

	if index == nil {
		i := len(p.notes)
		if i >= maxNotesPerUser {
			return commands.NewUserError("This user has the max amount of notes! Please delete or edit one.")
		}
		p.notes = append(p.notes, &Note{})
		index = &i
	}

	return p.edit(*index, note, author)
}

func (p *parsedNotes) edit(index int, note string, author *discordgo.User) error {
	if len(p.notes) <= index {
		// this shouldn't be possible
		logger.WithField("guild", p.guildID).WithField("index", index).WithField("length", len(p.notes)).Warn("a custom ID tried to select a nonexistent note")
		return invalidSelectionErr
	}
	note = strings.TrimSpace(note)
	lines := strings.Split(note, "\n")
	now := time.Now()
	p.notes[index] = &Note{
		noteLines: lines,
		signature: author.String(),
		authorID:  author.ID,
		updatedAt: now,
	}
	return nil
}

func (p *parsedNotes) delete(index int) error {
	if len(p.notes) <= index {
		// this shouldn't be possible
		logger.WithField("guild", p.guildID).WithField("index", index).WithField("length", len(p.notes)).Warn("a custom ID tried to select a nonexistent note")
		return invalidSelectionErr
	}
	p.notes[index] = &Note{}
	return nil
}

func (p *parsedNotes) save(ctx context.Context) error {
	var formattedNotes []string
	for _, n := range p.notes {
		formattedNotes = append(formattedNotes, n.formatNote())
	}

	logger.Info(formattedNotes)

	model := models.Note{
		GuildID:  p.guildID,
		MemberID: p.userID,
		Notes:    formattedNotes,
	}

	whitelist := boil.Whitelist("notes")
	return model.Upsert(ctx, common.PQ, true, []string{"guild_id", "member_id"}, whitelist, boil.Infer())
}

const noNotesMsg = "No shared staff notes written for this user."

func (p *parsedNotes) createMessageContent() *discordgo.MessageSend {
	msg := &discordgo.MessageSend{}
	if len(p.notes) == 0 {
		msg.Content = noNotesMsg
		return msg
	}

	for i := range maxNotesPerUser {
		if !(len(p.notes) > i && len(p.notes[i].noteLines) > 0 && p.notes[i].noteLines[0] != "") {
			continue
		}

		selectedNote := p.notes[i]
		msg.Embeds = append(msg.Embeds, &discordgo.MessageEmbed{
			Title:       fmt.Sprint("Note #", i+1),
			Description: strings.Join(selectedNote.noteLines, "\n"),
			Timestamp:   selectedNote.updatedAt.Format(time.RFC3339),
			Footer:      &discordgo.MessageEmbedFooter{Text: "by " + selectedNote.signature},
		})
	}

	if len(msg.Embeds) < 1 {
		msg.Content = noNotesMsg
		return msg
	}

	name := strconv.FormatInt(p.userID, 10)
	ms, _ := bot.GetMember(p.guildID, p.userID)
	if ms != nil {
		name = ms.User.String()
	}
	msg.Content = "## Shared Staff Notes for " + name
	return msg
}

type notesButtonType int

const (
	notesButtonTypeNew notesButtonType = iota
	notesButtonTypeEdit
	notesButtonTypeDelete
)

func (p *parsedNotes) customID(index int, buttonType notesButtonType) string {
	args := []string{strconv.FormatInt(p.userID, 10)}
	args = append(args, strconv.Itoa(index))
	switch buttonType {
	case notesButtonTypeNew:
		args[1] = "new"
	case notesButtonTypeEdit:
		args = append(args, "edit")
	case notesButtonTypeDelete:
		args = append(args, "delete")
	}

	id := strings.Join(args, "-")
	id = "notes_" + id
	return id
}

func (p *parsedNotes) createButtons() (rows []discordgo.MessageComponent) {
	var createNewNoteButton bool = len(p.notes) < maxNotesPerUser
	for i, n := range p.notes {
		if len(n.noteLines) < 1 || n.noteLines[0] == "" {
			createNewNoteButton = true
			continue
		}

		editButton := discordgo.Button{
			Label:    fmt.Sprintf("Edit Note #%d", i+1),
			Style:    discordgo.SecondaryButton,
			CustomID: p.customID(i, notesButtonTypeEdit),
		}
		deleteButton := discordgo.Button{
			Label:    fmt.Sprintf("Delete Note #%d", i+1),
			Style:    discordgo.DangerButton,
			CustomID: p.customID(i, notesButtonTypeDelete),
		}

		rows = append(rows, discordgo.ActionsRow{Components: []discordgo.MessageComponent{editButton, deleteButton}})
	}

	if createNewNoteButton {
		rows = append(rows, discordgo.ActionsRow{Components: []discordgo.MessageComponent{discordgo.Button{
			Label:    "Create New Note",
			Style:    discordgo.SuccessButton,
			CustomID: p.customID(0, notesButtonTypeNew),
		}}})
	}

	return
}

func (p *parsedNotes) createMessage() (msg *discordgo.MessageSend) {
	m := p.createMessageContent()
	msg = &discordgo.MessageSend{
		Content:    m.Content,
		Embeds:     m.Embeds,
		Components: p.createButtons(),
		Flags:      discordgo.MessageFlagsEphemeral,
	}
	return
}

func (p *parsedNotes) createModal(index *int) (modal *discordgo.InteractionResponse) {
	title := "Create New Note"
	notesType := notesButtonTypeNew
	safeIndex := 0
	fieldContent := ""
	if index != nil {
		title = fmt.Sprintf("Edit Note #%d", *index+1)
		notesType = notesButtonTypeEdit
		safeIndex = *index
		fieldContent = strings.Join(p.notes[safeIndex].noteLines, "\n")
	}
	modal = &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			Title:    title,
			CustomID: p.customID(safeIndex, notesType),
			Components: []discordgo.MessageComponent{discordgo.ActionsRow{Components: []discordgo.MessageComponent{discordgo.TextInput{
				CustomID:  "new",
				Label:     "Note",
				Style:     discordgo.TextInputParagraph,
				Value:     fieldContent,
				MaxLength: maxCharactersPerNote,
			}}}},
		},
	}
	return
}
