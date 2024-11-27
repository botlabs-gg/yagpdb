package patreonapi

import (
	"encoding/json"
	"errors"
	"reflect"
)

var TypeMap = map[string]interface{}{
	"user": UserAttributes{},
	"tier": TierAttributes{},
}

type Include struct {
	Type string `json:"type"`
	ID   string `json:"id"`

	Attributes json.RawMessage `json:"attributes"`
	Decoded    interface{}     `json:"-"`
}

func DecodeIncludes(includes []*Include) error {
	for _, v := range includes {
		dec, err := DecodeInclude(v)
		if err != nil {
			return err
		}

		v.Decoded = dec
	}

	return nil
}

func DecodeInclude(include *Include) (interface{}, error) {
	t, ok := TypeMap[include.Type]
	if !ok {
		return nil, errors.New("Unknown include: " + include.Type)
	}

	typ := reflect.TypeOf(t)

	dst := reflect.New(typ).Interface()
	err := json.Unmarshal(include.Attributes, dst)
	return dst, err
}

type Relationships struct {
	User  RelationShip      `json:"user"`
	Tiers RelationShipSlice `json:"currently_entitled_tiers"`
}

type RelationShip struct {
	Data *RelationshipData `json:"data"`
}

type RelationShipSlice struct {
	Data []*RelationshipData `json:"data"`
}

type RelationshipData struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}
