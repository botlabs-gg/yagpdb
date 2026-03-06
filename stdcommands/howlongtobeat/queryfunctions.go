package howlongtobeat

import (
	"context"
	"fmt"

	"github.com/forbiddencoding/howlongtobeat"
)

func getGameData(searchTitle string) ([]hltb, error) {
	hltb, err := howlongtobeat.New()
	if err != nil {
		return nil, err
	}
	searchResults, err := hltb.SearchSimple(context.TODO(), searchTitle, howlongtobeat.SearchModifierNone)
	if err != nil {
		return nil, err
	}

	return parseGameData(searchResults), nil
}

func parseGameData(games []*howlongtobeat.SearchGameSimple) []hltb {
	var parsedGames []hltb
	var parsingGame hltb
	for _, game := range games {
		parsingGame = hltb{
			GameTitle:               game.GameName,
			GameID:                  int64(game.GameID),
			ImagePath:               game.GameImage,
			MainStoryTime:           int64(game.CompMain),
			MainStoryTimeHours:      fmt.Sprintf("%.2f Hours ", game.CompMain),
			MainExtraTime:           int64(game.CompPlus),
			MainStoryExtraTimeHours: fmt.Sprintf("%.2f Hours ", game.CompPlus),
			CompletionistTime:       int64(game.CompAll),
			CompletionistTimeHours:  fmt.Sprintf("%.2f Hours ", game.CompAll),
			Similarity:              game.Similarity,
			GameURL:                 fmt.Sprintf("%s://%s/game/%d", hltbScheme, hltbHost, game.GameID),
			ImageURL:                fmt.Sprintf("%s://%s/games/%s", hltbScheme, hltbHost, game.GameImage),
		}
		parsedGames = append(parsedGames, parsingGame)
	}
	return parsedGames
}
