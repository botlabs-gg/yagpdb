package xkcd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"

	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

type Xkcd struct {
	Month      string
	Num        int64
	Link       string
	Year       string
	News       string
	SafeTitle  string
	Transcript string
	Alt        string
	Img        string
	Title      string
	Day        string
}

var XkcdHost = "https://xkcd.com/"
var XkcdJson = "info.0.json"

var Command = &commands.YAGCommand{
	CmdCategory: commands.CategoryFun,
	Name:        "Xkcd",
	Description: "An xkcd comic, by default returns random comic strip",
	Arguments: []*dcmd.ArgDef{
		{Name: "Comic-number", Type: dcmd.Int},
	},
	ArgSwitches: []*dcmd.ArgDef{
		{Name: "l", Help: "Latest comic"},
	},
	SlashCommandEnabled: true,
	DefaultEnabled:      true,
	RunFunc: func(data *dcmd.Data) (interface{}, error) {

		//first query to get latest number
		latest := false
		xkcd, err := getComic()
		if err != nil {
			return "Something happened whilst getting the comic!", err
		}

		xkcdNum := rand.Int63n(xkcd.Num) + 1

		//latest comic strip flag, already got that data
		if data.Switches["l"].Value != nil && data.Switches["l"].Value.(bool) {
			latest = true
		}

		//specific comic strip number
		if data.Args[0].Value != nil {
			n := data.Args[0].Int64()
			if n >= 1 && n <= xkcd.Num {
				xkcdNum = n
			} else {
				return fmt.Sprintf("There's no comic numbered %d, current range is 1-%d", n, xkcd.Num), nil
			}
		}

		//if no latest flag is set, fetches a comic by number
		if !latest {
			xkcd, err = getComic(xkcdNum)
			if err != nil {
				return "Something happened whilst getting the comic!", err
			}
		}

		message := &discordgo.MessageSend{
			Components: []discordgo.TopLevelComponent{discordgo.Container{
				AccentColor: int(rand.Int63n(16777215)),
				Components: []discordgo.TopLevelComponent{
					discordgo.TextDisplay{Content: fmt.Sprintf("## \\#%d: %s", xkcd.Num, xkcd.Title)},
					discordgo.MediaGallery{Items: []discordgo.MediaGalleryItem{
						{
							Media: discordgo.UnfurledMediaItem{URL: xkcd.Img},
						},
					}},
					discordgo.TextDisplay{Content: fmt.Sprintf("[%s](%s%d/)", xkcd.Alt, XkcdHost, xkcd.Num)},
				},
			}},
			Flags: discordgo.MessageFlagsIsComponentsV2,
		}
		return message, nil
	},
}

func getComic(number ...int64) (*Xkcd, error) {
	xkcd := Xkcd{}
	queryUrl := XkcdHost + XkcdJson

	if len(number) >= 1 {
		queryUrl = fmt.Sprintf(XkcdHost+"%d/"+XkcdJson, number[0])
	}

	req, err := http.NewRequest("GET", queryUrl, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "YAGPDB.xyz (https://github.com/botlabs-gg/yagpdb)")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	queryErr := json.Unmarshal(body, &xkcd)
	if queryErr != nil {
		return nil, queryErr
	}

	return &xkcd, nil
}
