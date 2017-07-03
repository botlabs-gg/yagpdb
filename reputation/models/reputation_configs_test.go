package models

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/vattle/sqlboiler/boil"
	"github.com/vattle/sqlboiler/randomize"
	"github.com/vattle/sqlboiler/strmangle"
)

func testReputationConfigs(t *testing.T) {
	t.Parallel()

	query := ReputationConfigs(nil)

	if query.Query == nil {
		t.Error("expected a query, got nothing")
	}
}
func testReputationConfigsDelete(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	reputationConfig := &ReputationConfig{}
	if err = randomize.Struct(seed, reputationConfig, reputationConfigDBTypes, true); err != nil {
		t.Errorf("Unable to randomize ReputationConfig struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = reputationConfig.Insert(tx); err != nil {
		t.Error(err)
	}

	if err = reputationConfig.Delete(tx); err != nil {
		t.Error(err)
	}

	count, err := ReputationConfigs(tx).Count()
	if err != nil {
		t.Error(err)
	}

	if count != 0 {
		t.Error("want zero records, got:", count)
	}
}

func testReputationConfigsQueryDeleteAll(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	reputationConfig := &ReputationConfig{}
	if err = randomize.Struct(seed, reputationConfig, reputationConfigDBTypes, true); err != nil {
		t.Errorf("Unable to randomize ReputationConfig struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = reputationConfig.Insert(tx); err != nil {
		t.Error(err)
	}

	if err = ReputationConfigs(tx).DeleteAll(); err != nil {
		t.Error(err)
	}

	count, err := ReputationConfigs(tx).Count()
	if err != nil {
		t.Error(err)
	}

	if count != 0 {
		t.Error("want zero records, got:", count)
	}
}

func testReputationConfigsSliceDeleteAll(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	reputationConfig := &ReputationConfig{}
	if err = randomize.Struct(seed, reputationConfig, reputationConfigDBTypes, true); err != nil {
		t.Errorf("Unable to randomize ReputationConfig struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = reputationConfig.Insert(tx); err != nil {
		t.Error(err)
	}

	slice := ReputationConfigSlice{reputationConfig}

	if err = slice.DeleteAll(tx); err != nil {
		t.Error(err)
	}

	count, err := ReputationConfigs(tx).Count()
	if err != nil {
		t.Error(err)
	}

	if count != 0 {
		t.Error("want zero records, got:", count)
	}
}
func testReputationConfigsExists(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	reputationConfig := &ReputationConfig{}
	if err = randomize.Struct(seed, reputationConfig, reputationConfigDBTypes, true, reputationConfigColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize ReputationConfig struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = reputationConfig.Insert(tx); err != nil {
		t.Error(err)
	}

	e, err := ReputationConfigExists(tx, reputationConfig.GuildID)
	if err != nil {
		t.Errorf("Unable to check if ReputationConfig exists: %s", err)
	}
	if !e {
		t.Errorf("Expected ReputationConfigExistsG to return true, but got false.")
	}
}
func testReputationConfigsFind(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	reputationConfig := &ReputationConfig{}
	if err = randomize.Struct(seed, reputationConfig, reputationConfigDBTypes, true, reputationConfigColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize ReputationConfig struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = reputationConfig.Insert(tx); err != nil {
		t.Error(err)
	}

	reputationConfigFound, err := FindReputationConfig(tx, reputationConfig.GuildID)
	if err != nil {
		t.Error(err)
	}

	if reputationConfigFound == nil {
		t.Error("want a record, got nil")
	}
}
func testReputationConfigsBind(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	reputationConfig := &ReputationConfig{}
	if err = randomize.Struct(seed, reputationConfig, reputationConfigDBTypes, true, reputationConfigColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize ReputationConfig struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = reputationConfig.Insert(tx); err != nil {
		t.Error(err)
	}

	if err = ReputationConfigs(tx).Bind(reputationConfig); err != nil {
		t.Error(err)
	}
}

func testReputationConfigsOne(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	reputationConfig := &ReputationConfig{}
	if err = randomize.Struct(seed, reputationConfig, reputationConfigDBTypes, true, reputationConfigColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize ReputationConfig struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = reputationConfig.Insert(tx); err != nil {
		t.Error(err)
	}

	if x, err := ReputationConfigs(tx).One(); err != nil {
		t.Error(err)
	} else if x == nil {
		t.Error("expected to get a non nil record")
	}
}

func testReputationConfigsAll(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	reputationConfigOne := &ReputationConfig{}
	reputationConfigTwo := &ReputationConfig{}
	if err = randomize.Struct(seed, reputationConfigOne, reputationConfigDBTypes, false, reputationConfigColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize ReputationConfig struct: %s", err)
	}
	if err = randomize.Struct(seed, reputationConfigTwo, reputationConfigDBTypes, false, reputationConfigColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize ReputationConfig struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = reputationConfigOne.Insert(tx); err != nil {
		t.Error(err)
	}
	if err = reputationConfigTwo.Insert(tx); err != nil {
		t.Error(err)
	}

	slice, err := ReputationConfigs(tx).All()
	if err != nil {
		t.Error(err)
	}

	if len(slice) != 2 {
		t.Error("want 2 records, got:", len(slice))
	}
}

func testReputationConfigsCount(t *testing.T) {
	t.Parallel()

	var err error
	seed := randomize.NewSeed()
	reputationConfigOne := &ReputationConfig{}
	reputationConfigTwo := &ReputationConfig{}
	if err = randomize.Struct(seed, reputationConfigOne, reputationConfigDBTypes, false, reputationConfigColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize ReputationConfig struct: %s", err)
	}
	if err = randomize.Struct(seed, reputationConfigTwo, reputationConfigDBTypes, false, reputationConfigColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize ReputationConfig struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = reputationConfigOne.Insert(tx); err != nil {
		t.Error(err)
	}
	if err = reputationConfigTwo.Insert(tx); err != nil {
		t.Error(err)
	}

	count, err := ReputationConfigs(tx).Count()
	if err != nil {
		t.Error(err)
	}

	if count != 2 {
		t.Error("want 2 records, got:", count)
	}
}

func testReputationConfigsInsert(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	reputationConfig := &ReputationConfig{}
	if err = randomize.Struct(seed, reputationConfig, reputationConfigDBTypes, true, reputationConfigColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize ReputationConfig struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = reputationConfig.Insert(tx); err != nil {
		t.Error(err)
	}

	count, err := ReputationConfigs(tx).Count()
	if err != nil {
		t.Error(err)
	}

	if count != 1 {
		t.Error("want one record, got:", count)
	}
}

func testReputationConfigsInsertWhitelist(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	reputationConfig := &ReputationConfig{}
	if err = randomize.Struct(seed, reputationConfig, reputationConfigDBTypes, true); err != nil {
		t.Errorf("Unable to randomize ReputationConfig struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = reputationConfig.Insert(tx, reputationConfigColumns...); err != nil {
		t.Error(err)
	}

	count, err := ReputationConfigs(tx).Count()
	if err != nil {
		t.Error(err)
	}

	if count != 1 {
		t.Error("want one record, got:", count)
	}
}

func testReputationConfigsReload(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	reputationConfig := &ReputationConfig{}
	if err = randomize.Struct(seed, reputationConfig, reputationConfigDBTypes, true, reputationConfigColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize ReputationConfig struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = reputationConfig.Insert(tx); err != nil {
		t.Error(err)
	}

	if err = reputationConfig.Reload(tx); err != nil {
		t.Error(err)
	}
}

func testReputationConfigsReloadAll(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	reputationConfig := &ReputationConfig{}
	if err = randomize.Struct(seed, reputationConfig, reputationConfigDBTypes, true, reputationConfigColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize ReputationConfig struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = reputationConfig.Insert(tx); err != nil {
		t.Error(err)
	}

	slice := ReputationConfigSlice{reputationConfig}

	if err = slice.ReloadAll(tx); err != nil {
		t.Error(err)
	}
}
func testReputationConfigsSelect(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	reputationConfig := &ReputationConfig{}
	if err = randomize.Struct(seed, reputationConfig, reputationConfigDBTypes, true, reputationConfigColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize ReputationConfig struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = reputationConfig.Insert(tx); err != nil {
		t.Error(err)
	}

	slice, err := ReputationConfigs(tx).All()
	if err != nil {
		t.Error(err)
	}

	if len(slice) != 1 {
		t.Error("want one record, got:", len(slice))
	}
}

var (
	reputationConfigDBTypes = map[string]string{`AdminRole`: `character varying`, `BlacklistedGiveRole`: `character varying`, `BlacklistedReceiveRole`: `character varying`, `Cooldown`: `integer`, `Enabled`: `boolean`, `GuildID`: `bigint`, `MaxGiveAmount`: `bigint`, `PointsName`: `character varying`, `RequiredGiveRole`: `character varying`, `RequiredReceiveRole`: `character varying`}
	_                       = bytes.MinRead
)

func testReputationConfigsUpdate(t *testing.T) {
	t.Parallel()

	if len(reputationConfigColumns) == len(reputationConfigPrimaryKeyColumns) {
		t.Skip("Skipping table with only primary key columns")
	}

	seed := randomize.NewSeed()
	var err error
	reputationConfig := &ReputationConfig{}
	if err = randomize.Struct(seed, reputationConfig, reputationConfigDBTypes, true); err != nil {
		t.Errorf("Unable to randomize ReputationConfig struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = reputationConfig.Insert(tx); err != nil {
		t.Error(err)
	}

	count, err := ReputationConfigs(tx).Count()
	if err != nil {
		t.Error(err)
	}

	if count != 1 {
		t.Error("want one record, got:", count)
	}

	if err = randomize.Struct(seed, reputationConfig, reputationConfigDBTypes, true, reputationConfigColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize ReputationConfig struct: %s", err)
	}

	if err = reputationConfig.Update(tx); err != nil {
		t.Error(err)
	}
}

func testReputationConfigsSliceUpdateAll(t *testing.T) {
	t.Parallel()

	if len(reputationConfigColumns) == len(reputationConfigPrimaryKeyColumns) {
		t.Skip("Skipping table with only primary key columns")
	}

	seed := randomize.NewSeed()
	var err error
	reputationConfig := &ReputationConfig{}
	if err = randomize.Struct(seed, reputationConfig, reputationConfigDBTypes, true); err != nil {
		t.Errorf("Unable to randomize ReputationConfig struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = reputationConfig.Insert(tx); err != nil {
		t.Error(err)
	}

	count, err := ReputationConfigs(tx).Count()
	if err != nil {
		t.Error(err)
	}

	if count != 1 {
		t.Error("want one record, got:", count)
	}

	if err = randomize.Struct(seed, reputationConfig, reputationConfigDBTypes, true, reputationConfigPrimaryKeyColumns...); err != nil {
		t.Errorf("Unable to randomize ReputationConfig struct: %s", err)
	}

	// Remove Primary keys and unique columns from what we plan to update
	var fields []string
	if strmangle.StringSliceMatch(reputationConfigColumns, reputationConfigPrimaryKeyColumns) {
		fields = reputationConfigColumns
	} else {
		fields = strmangle.SetComplement(
			reputationConfigColumns,
			reputationConfigPrimaryKeyColumns,
		)
	}

	value := reflect.Indirect(reflect.ValueOf(reputationConfig))
	updateMap := M{}
	for _, col := range fields {
		updateMap[col] = value.FieldByName(strmangle.TitleCase(col)).Interface()
	}

	slice := ReputationConfigSlice{reputationConfig}
	if err = slice.UpdateAll(tx, updateMap); err != nil {
		t.Error(err)
	}
}
func testReputationConfigsUpsert(t *testing.T) {
	t.Parallel()

	if len(reputationConfigColumns) == len(reputationConfigPrimaryKeyColumns) {
		t.Skip("Skipping table with only primary key columns")
	}

	seed := randomize.NewSeed()
	var err error
	// Attempt the INSERT side of an UPSERT
	reputationConfig := ReputationConfig{}
	if err = randomize.Struct(seed, &reputationConfig, reputationConfigDBTypes, true); err != nil {
		t.Errorf("Unable to randomize ReputationConfig struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = reputationConfig.Upsert(tx, false, nil, nil); err != nil {
		t.Errorf("Unable to upsert ReputationConfig: %s", err)
	}

	count, err := ReputationConfigs(tx).Count()
	if err != nil {
		t.Error(err)
	}
	if count != 1 {
		t.Error("want one record, got:", count)
	}

	// Attempt the UPDATE side of an UPSERT
	if err = randomize.Struct(seed, &reputationConfig, reputationConfigDBTypes, false, reputationConfigPrimaryKeyColumns...); err != nil {
		t.Errorf("Unable to randomize ReputationConfig struct: %s", err)
	}

	if err = reputationConfig.Upsert(tx, true, nil, nil); err != nil {
		t.Errorf("Unable to upsert ReputationConfig: %s", err)
	}

	count, err = ReputationConfigs(tx).Count()
	if err != nil {
		t.Error(err)
	}
	if count != 1 {
		t.Error("want one record, got:", count)
	}
}
