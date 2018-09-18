package automod

import (
	"encoding/json"
	"github.com/jonas747/yagpdb/automod/models"
	"github.com/pkg/errors"
)

type ParsedRuleset struct {
	RSModel          *models.AutomodRuleset
	ParsedConditions []*ParsedPart
	Rules            []*ParsedRule
}

type ParsedRule struct {
	Model      *models.AutomodRule
	Triggers   []*ParsedPart
	Conditions []*ParsedPart
	Effects    []*ParsedPart
}

type ParsedPart struct {
	RuleModel        *models.AutomodRuleDatum
	RSConditionModel *models.AutomodRulesetCondition

	Part           RulePart
	ParsedSettings interface{}
}

func ParseRuleset(rs *models.AutomodRuleset) (*ParsedRuleset, error) {
	result := &ParsedRuleset{
		RSModel: rs,
	}

	result.ParsedConditions = make([]*ParsedPart, len(rs.R.RulesetAutomodRulesetConditions))
	result.Rules = make([]*ParsedRule, len(rs.R.RulesetAutomodRules))

	for i, v := range rs.R.RulesetAutomodRulesetConditions {
		partType := RulePartMap[v.TypeID]
		dst := partType.DataType()
		if dst != nil {
			err := json.Unmarshal(v.Settings, dst)
			if err != nil {
				return nil, errors.WithMessage(err, "rsconditions, json")
			}
		}

		result.ParsedConditions[i] = &ParsedPart{
			RSConditionModel: v,
			Part:             partType,
			ParsedSettings:   dst,
		}
	}

	for i, v := range rs.R.RulesetAutomodRules {
		parsed, err := ParseRuleData(v)
		if err != nil {
			return nil, err
		}

		result.Rules[i] = parsed
	}

	return result, nil
}

func ParseRuleData(rule *models.AutomodRule) (*ParsedRule, error) {

	var triggers []*ParsedPart
	var conditions []*ParsedPart
	var effects []*ParsedPart

	for _, v := range rule.R.RuleAutomodRuleData {
		partType := RulePartMap[v.TypeID]
		dst := partType.DataType()
		if dst != nil {
			err := json.Unmarshal(v.Settings, dst)
			if err != nil {
				return nil, errors.WithMessage(err, "rule, json")
			}
		}

		p := &ParsedPart{
			RuleModel:      v,
			Part:           partType,
			ParsedSettings: dst,
		}

		switch RulePartType(v.Kind) {
		case RulePartTrigger:
			triggers = append(triggers, p)
		case RulePartCondition:
			conditions = append(conditions, p)
		case RulePartEffect:
			effects = append(effects, p)
		}
	}

	return &ParsedRule{
		Model:      rule,
		Triggers:   triggers,
		Conditions: conditions,
		Effects:    effects,
	}, nil
}
