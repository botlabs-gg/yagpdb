package automod

import (
	"encoding/json"
	"github.com/jonas747/yagpdb/automod/models"
	"github.com/jonas747/yagpdb/common"
	"github.com/sirupsen/logrus"
	"strconv"
)

//go:generate sqlboiler --no-hooks psql

type Plugin struct {
}

func (p *Plugin) Name() string {
	return "Automod v2"
}

func RegisterPlugin() {
	_, err := common.PQ.Exec(DBSchema)
	if err != nil {
		logrus.WithError(err).Error("Failed setting up automod postgres tables, plugin will be disabled.")
		return
	}

	p := &Plugin{}
	common.RegisterPlugin(p)
}

type ErrUnknownTypeID struct {
	TypeID int
}

func (e *ErrUnknownTypeID) Error() string {
	return "Unknown TypeID: " + strconv.Itoa(e.TypeID)
}

func ParseRulePartData(model *models.AutomodRuleDatum) (interface{}, error) {
	part, ok := RulePartMap[model.TypeID]
	if !ok {
		return nil, &ErrUnknownTypeID{model.TypeID}
	}

	settingsDestination := part.DataType()
	if settingsDestination == nil {
		// No user settings for this part
		return nil, nil
	}

	err := json.Unmarshal(model.Settings, settingsDestination)
	return settingsDestination, err
}

func ParseAllRulePartData(dataModels []*models.AutomodRuleDatum) ([]interface{}, error) {
	dst := make([]interface{}, len(dataModels))
	for i, v := range dataModels {
		parsed, err := ParseRulePartData(v)
		if err != nil {
			return nil, err
		}

		dst[i] = parsed
	}

	return dst, nil
}
