package models

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/vattle/sqlboiler/boil"
	"github.com/vattle/sqlboiler/randomize"
	"github.com/vattle/sqlboiler/strmangle"
)

func testReputationLogs(t *testing.T) {
	t.Parallel()

	query := ReputationLogs(nil)

	if query.Query == nil {
		t.Error("expected a query, got nothing")
	}
}
func testReputationLogsDelete(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	reputationLog := &ReputationLog{}
	if err = randomize.Struct(seed, reputationLog, reputationLogDBTypes, true); err != nil {
		t.Errorf("Unable to randomize ReputationLog struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = reputationLog.Insert(tx); err != nil {
		t.Error(err)
	}

	if err = reputationLog.Delete(tx); err != nil {
		t.Error(err)
	}

	count, err := ReputationLogs(tx).Count()
	if err != nil {
		t.Error(err)
	}

	if count != 0 {
		t.Error("want zero records, got:", count)
	}
}

func testReputationLogsQueryDeleteAll(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	reputationLog := &ReputationLog{}
	if err = randomize.Struct(seed, reputationLog, reputationLogDBTypes, true); err != nil {
		t.Errorf("Unable to randomize ReputationLog struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = reputationLog.Insert(tx); err != nil {
		t.Error(err)
	}

	if err = ReputationLogs(tx).DeleteAll(); err != nil {
		t.Error(err)
	}

	count, err := ReputationLogs(tx).Count()
	if err != nil {
		t.Error(err)
	}

	if count != 0 {
		t.Error("want zero records, got:", count)
	}
}

func testReputationLogsSliceDeleteAll(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	reputationLog := &ReputationLog{}
	if err = randomize.Struct(seed, reputationLog, reputationLogDBTypes, true); err != nil {
		t.Errorf("Unable to randomize ReputationLog struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = reputationLog.Insert(tx); err != nil {
		t.Error(err)
	}

	slice := ReputationLogSlice{reputationLog}

	if err = slice.DeleteAll(tx); err != nil {
		t.Error(err)
	}

	count, err := ReputationLogs(tx).Count()
	if err != nil {
		t.Error(err)
	}

	if count != 0 {
		t.Error("want zero records, got:", count)
	}
}
func testReputationLogsExists(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	reputationLog := &ReputationLog{}
	if err = randomize.Struct(seed, reputationLog, reputationLogDBTypes, true, reputationLogColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize ReputationLog struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = reputationLog.Insert(tx); err != nil {
		t.Error(err)
	}

	e, err := ReputationLogExists(tx, reputationLog.ID)
	if err != nil {
		t.Errorf("Unable to check if ReputationLog exists: %s", err)
	}
	if !e {
		t.Errorf("Expected ReputationLogExistsG to return true, but got false.")
	}
}
func testReputationLogsFind(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	reputationLog := &ReputationLog{}
	if err = randomize.Struct(seed, reputationLog, reputationLogDBTypes, true, reputationLogColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize ReputationLog struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = reputationLog.Insert(tx); err != nil {
		t.Error(err)
	}

	reputationLogFound, err := FindReputationLog(tx, reputationLog.ID)
	if err != nil {
		t.Error(err)
	}

	if reputationLogFound == nil {
		t.Error("want a record, got nil")
	}
}
func testReputationLogsBind(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	reputationLog := &ReputationLog{}
	if err = randomize.Struct(seed, reputationLog, reputationLogDBTypes, true, reputationLogColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize ReputationLog struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = reputationLog.Insert(tx); err != nil {
		t.Error(err)
	}

	if err = ReputationLogs(tx).Bind(reputationLog); err != nil {
		t.Error(err)
	}
}

func testReputationLogsOne(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	reputationLog := &ReputationLog{}
	if err = randomize.Struct(seed, reputationLog, reputationLogDBTypes, true, reputationLogColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize ReputationLog struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = reputationLog.Insert(tx); err != nil {
		t.Error(err)
	}

	if x, err := ReputationLogs(tx).One(); err != nil {
		t.Error(err)
	} else if x == nil {
		t.Error("expected to get a non nil record")
	}
}

func testReputationLogsAll(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	reputationLogOne := &ReputationLog{}
	reputationLogTwo := &ReputationLog{}
	if err = randomize.Struct(seed, reputationLogOne, reputationLogDBTypes, false, reputationLogColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize ReputationLog struct: %s", err)
	}
	if err = randomize.Struct(seed, reputationLogTwo, reputationLogDBTypes, false, reputationLogColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize ReputationLog struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = reputationLogOne.Insert(tx); err != nil {
		t.Error(err)
	}
	if err = reputationLogTwo.Insert(tx); err != nil {
		t.Error(err)
	}

	slice, err := ReputationLogs(tx).All()
	if err != nil {
		t.Error(err)
	}

	if len(slice) != 2 {
		t.Error("want 2 records, got:", len(slice))
	}
}

func testReputationLogsCount(t *testing.T) {
	t.Parallel()

	var err error
	seed := randomize.NewSeed()
	reputationLogOne := &ReputationLog{}
	reputationLogTwo := &ReputationLog{}
	if err = randomize.Struct(seed, reputationLogOne, reputationLogDBTypes, false, reputationLogColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize ReputationLog struct: %s", err)
	}
	if err = randomize.Struct(seed, reputationLogTwo, reputationLogDBTypes, false, reputationLogColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize ReputationLog struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = reputationLogOne.Insert(tx); err != nil {
		t.Error(err)
	}
	if err = reputationLogTwo.Insert(tx); err != nil {
		t.Error(err)
	}

	count, err := ReputationLogs(tx).Count()
	if err != nil {
		t.Error(err)
	}

	if count != 2 {
		t.Error("want 2 records, got:", count)
	}
}
func reputationLogBeforeInsertHook(e boil.Executor, o *ReputationLog) error {
	*o = ReputationLog{}
	return nil
}

func reputationLogAfterInsertHook(e boil.Executor, o *ReputationLog) error {
	*o = ReputationLog{}
	return nil
}

func reputationLogAfterSelectHook(e boil.Executor, o *ReputationLog) error {
	*o = ReputationLog{}
	return nil
}

func reputationLogBeforeUpdateHook(e boil.Executor, o *ReputationLog) error {
	*o = ReputationLog{}
	return nil
}

func reputationLogAfterUpdateHook(e boil.Executor, o *ReputationLog) error {
	*o = ReputationLog{}
	return nil
}

func reputationLogBeforeDeleteHook(e boil.Executor, o *ReputationLog) error {
	*o = ReputationLog{}
	return nil
}

func reputationLogAfterDeleteHook(e boil.Executor, o *ReputationLog) error {
	*o = ReputationLog{}
	return nil
}

func reputationLogBeforeUpsertHook(e boil.Executor, o *ReputationLog) error {
	*o = ReputationLog{}
	return nil
}

func reputationLogAfterUpsertHook(e boil.Executor, o *ReputationLog) error {
	*o = ReputationLog{}
	return nil
}

func testReputationLogsHooks(t *testing.T) {
	t.Parallel()

	var err error

	empty := &ReputationLog{}
	o := &ReputationLog{}

	seed := randomize.NewSeed()
	if err = randomize.Struct(seed, o, reputationLogDBTypes, false); err != nil {
		t.Errorf("Unable to randomize ReputationLog object: %s", err)
	}

	AddReputationLogHook(boil.BeforeInsertHook, reputationLogBeforeInsertHook)
	if err = o.doBeforeInsertHooks(nil); err != nil {
		t.Errorf("Unable to execute doBeforeInsertHooks: %s", err)
	}
	if !reflect.DeepEqual(o, empty) {
		t.Errorf("Expected BeforeInsertHook function to empty object, but got: %#v", o)
	}
	reputationLogBeforeInsertHooks = []ReputationLogHook{}

	AddReputationLogHook(boil.AfterInsertHook, reputationLogAfterInsertHook)
	if err = o.doAfterInsertHooks(nil); err != nil {
		t.Errorf("Unable to execute doAfterInsertHooks: %s", err)
	}
	if !reflect.DeepEqual(o, empty) {
		t.Errorf("Expected AfterInsertHook function to empty object, but got: %#v", o)
	}
	reputationLogAfterInsertHooks = []ReputationLogHook{}

	AddReputationLogHook(boil.AfterSelectHook, reputationLogAfterSelectHook)
	if err = o.doAfterSelectHooks(nil); err != nil {
		t.Errorf("Unable to execute doAfterSelectHooks: %s", err)
	}
	if !reflect.DeepEqual(o, empty) {
		t.Errorf("Expected AfterSelectHook function to empty object, but got: %#v", o)
	}
	reputationLogAfterSelectHooks = []ReputationLogHook{}

	AddReputationLogHook(boil.BeforeUpdateHook, reputationLogBeforeUpdateHook)
	if err = o.doBeforeUpdateHooks(nil); err != nil {
		t.Errorf("Unable to execute doBeforeUpdateHooks: %s", err)
	}
	if !reflect.DeepEqual(o, empty) {
		t.Errorf("Expected BeforeUpdateHook function to empty object, but got: %#v", o)
	}
	reputationLogBeforeUpdateHooks = []ReputationLogHook{}

	AddReputationLogHook(boil.AfterUpdateHook, reputationLogAfterUpdateHook)
	if err = o.doAfterUpdateHooks(nil); err != nil {
		t.Errorf("Unable to execute doAfterUpdateHooks: %s", err)
	}
	if !reflect.DeepEqual(o, empty) {
		t.Errorf("Expected AfterUpdateHook function to empty object, but got: %#v", o)
	}
	reputationLogAfterUpdateHooks = []ReputationLogHook{}

	AddReputationLogHook(boil.BeforeDeleteHook, reputationLogBeforeDeleteHook)
	if err = o.doBeforeDeleteHooks(nil); err != nil {
		t.Errorf("Unable to execute doBeforeDeleteHooks: %s", err)
	}
	if !reflect.DeepEqual(o, empty) {
		t.Errorf("Expected BeforeDeleteHook function to empty object, but got: %#v", o)
	}
	reputationLogBeforeDeleteHooks = []ReputationLogHook{}

	AddReputationLogHook(boil.AfterDeleteHook, reputationLogAfterDeleteHook)
	if err = o.doAfterDeleteHooks(nil); err != nil {
		t.Errorf("Unable to execute doAfterDeleteHooks: %s", err)
	}
	if !reflect.DeepEqual(o, empty) {
		t.Errorf("Expected AfterDeleteHook function to empty object, but got: %#v", o)
	}
	reputationLogAfterDeleteHooks = []ReputationLogHook{}

	AddReputationLogHook(boil.BeforeUpsertHook, reputationLogBeforeUpsertHook)
	if err = o.doBeforeUpsertHooks(nil); err != nil {
		t.Errorf("Unable to execute doBeforeUpsertHooks: %s", err)
	}
	if !reflect.DeepEqual(o, empty) {
		t.Errorf("Expected BeforeUpsertHook function to empty object, but got: %#v", o)
	}
	reputationLogBeforeUpsertHooks = []ReputationLogHook{}

	AddReputationLogHook(boil.AfterUpsertHook, reputationLogAfterUpsertHook)
	if err = o.doAfterUpsertHooks(nil); err != nil {
		t.Errorf("Unable to execute doAfterUpsertHooks: %s", err)
	}
	if !reflect.DeepEqual(o, empty) {
		t.Errorf("Expected AfterUpsertHook function to empty object, but got: %#v", o)
	}
	reputationLogAfterUpsertHooks = []ReputationLogHook{}
}
func testReputationLogsInsert(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	reputationLog := &ReputationLog{}
	if err = randomize.Struct(seed, reputationLog, reputationLogDBTypes, true, reputationLogColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize ReputationLog struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = reputationLog.Insert(tx); err != nil {
		t.Error(err)
	}

	count, err := ReputationLogs(tx).Count()
	if err != nil {
		t.Error(err)
	}

	if count != 1 {
		t.Error("want one record, got:", count)
	}
}

func testReputationLogsInsertWhitelist(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	reputationLog := &ReputationLog{}
	if err = randomize.Struct(seed, reputationLog, reputationLogDBTypes, true); err != nil {
		t.Errorf("Unable to randomize ReputationLog struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = reputationLog.Insert(tx, reputationLogColumns...); err != nil {
		t.Error(err)
	}

	count, err := ReputationLogs(tx).Count()
	if err != nil {
		t.Error(err)
	}

	if count != 1 {
		t.Error("want one record, got:", count)
	}
}

func testReputationLogsReload(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	reputationLog := &ReputationLog{}
	if err = randomize.Struct(seed, reputationLog, reputationLogDBTypes, true, reputationLogColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize ReputationLog struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = reputationLog.Insert(tx); err != nil {
		t.Error(err)
	}

	if err = reputationLog.Reload(tx); err != nil {
		t.Error(err)
	}
}

func testReputationLogsReloadAll(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	reputationLog := &ReputationLog{}
	if err = randomize.Struct(seed, reputationLog, reputationLogDBTypes, true, reputationLogColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize ReputationLog struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = reputationLog.Insert(tx); err != nil {
		t.Error(err)
	}

	slice := ReputationLogSlice{reputationLog}

	if err = slice.ReloadAll(tx); err != nil {
		t.Error(err)
	}
}
func testReputationLogsSelect(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	reputationLog := &ReputationLog{}
	if err = randomize.Struct(seed, reputationLog, reputationLogDBTypes, true, reputationLogColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize ReputationLog struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = reputationLog.Insert(tx); err != nil {
		t.Error(err)
	}

	slice, err := ReputationLogs(tx).All()
	if err != nil {
		t.Error(err)
	}

	if len(slice) != 1 {
		t.Error("want one record, got:", len(slice))
	}
}

var (
	reputationLogDBTypes = map[string]string{`Amount`: `bigint`, `CreatedAt`: `timestamp with time zone`, `GuildID`: `bigint`, `ID`: `bigint`, `ReceiverID`: `bigint`, `SenderID`: `bigint`, `SetFixedAmount`: `boolean`}
	_                    = bytes.MinRead
)

func testReputationLogsUpdate(t *testing.T) {
	t.Parallel()

	if len(reputationLogColumns) == len(reputationLogPrimaryKeyColumns) {
		t.Skip("Skipping table with only primary key columns")
	}

	seed := randomize.NewSeed()
	var err error
	reputationLog := &ReputationLog{}
	if err = randomize.Struct(seed, reputationLog, reputationLogDBTypes, true); err != nil {
		t.Errorf("Unable to randomize ReputationLog struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = reputationLog.Insert(tx); err != nil {
		t.Error(err)
	}

	count, err := ReputationLogs(tx).Count()
	if err != nil {
		t.Error(err)
	}

	if count != 1 {
		t.Error("want one record, got:", count)
	}

	if err = randomize.Struct(seed, reputationLog, reputationLogDBTypes, true, reputationLogColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize ReputationLog struct: %s", err)
	}

	if err = reputationLog.Update(tx); err != nil {
		t.Error(err)
	}
}

func testReputationLogsSliceUpdateAll(t *testing.T) {
	t.Parallel()

	if len(reputationLogColumns) == len(reputationLogPrimaryKeyColumns) {
		t.Skip("Skipping table with only primary key columns")
	}

	seed := randomize.NewSeed()
	var err error
	reputationLog := &ReputationLog{}
	if err = randomize.Struct(seed, reputationLog, reputationLogDBTypes, true); err != nil {
		t.Errorf("Unable to randomize ReputationLog struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = reputationLog.Insert(tx); err != nil {
		t.Error(err)
	}

	count, err := ReputationLogs(tx).Count()
	if err != nil {
		t.Error(err)
	}

	if count != 1 {
		t.Error("want one record, got:", count)
	}

	if err = randomize.Struct(seed, reputationLog, reputationLogDBTypes, true, reputationLogPrimaryKeyColumns...); err != nil {
		t.Errorf("Unable to randomize ReputationLog struct: %s", err)
	}

	// Remove Primary keys and unique columns from what we plan to update
	var fields []string
	if strmangle.StringSliceMatch(reputationLogColumns, reputationLogPrimaryKeyColumns) {
		fields = reputationLogColumns
	} else {
		fields = strmangle.SetComplement(
			reputationLogColumns,
			reputationLogPrimaryKeyColumns,
		)
	}

	value := reflect.Indirect(reflect.ValueOf(reputationLog))
	updateMap := M{}
	for _, col := range fields {
		updateMap[col] = value.FieldByName(strmangle.TitleCase(col)).Interface()
	}

	slice := ReputationLogSlice{reputationLog}
	if err = slice.UpdateAll(tx, updateMap); err != nil {
		t.Error(err)
	}
}
func testReputationLogsUpsert(t *testing.T) {
	t.Parallel()

	if len(reputationLogColumns) == len(reputationLogPrimaryKeyColumns) {
		t.Skip("Skipping table with only primary key columns")
	}

	seed := randomize.NewSeed()
	var err error
	// Attempt the INSERT side of an UPSERT
	reputationLog := ReputationLog{}
	if err = randomize.Struct(seed, &reputationLog, reputationLogDBTypes, true); err != nil {
		t.Errorf("Unable to randomize ReputationLog struct: %s", err)
	}

	tx := MustTx(boil.Begin())
	defer tx.Rollback()
	if err = reputationLog.Upsert(tx, false, nil, nil); err != nil {
		t.Errorf("Unable to upsert ReputationLog: %s", err)
	}

	count, err := ReputationLogs(tx).Count()
	if err != nil {
		t.Error(err)
	}
	if count != 1 {
		t.Error("want one record, got:", count)
	}

	// Attempt the UPDATE side of an UPSERT
	if err = randomize.Struct(seed, &reputationLog, reputationLogDBTypes, false, reputationLogPrimaryKeyColumns...); err != nil {
		t.Errorf("Unable to randomize ReputationLog struct: %s", err)
	}

	if err = reputationLog.Upsert(tx, true, nil, nil); err != nil {
		t.Errorf("Unable to upsert ReputationLog: %s", err)
	}

	count, err = ReputationLogs(tx).Count()
	if err != nil {
		t.Error(err)
	}
	if count != 1 {
		t.Error("want one record, got:", count)
	}
}
