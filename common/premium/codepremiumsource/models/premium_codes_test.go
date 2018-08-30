// Code generated by SQLBoiler (https://github.com/volatiletech/sqlboiler). DO NOT EDIT.
// This file is meant to be re-generated in place and/or deleted at any time.

package models

import (
	"bytes"
	"context"
	"reflect"
	"testing"

	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries"
	"github.com/volatiletech/sqlboiler/randomize"
	"github.com/volatiletech/sqlboiler/strmangle"
)

var (
	// Relationships sometimes use the reflection helper queries.Equal/queries.Assign
	// so force a package dependency in case they don't.
	_ = queries.Equal
)

func testPremiumCodes(t *testing.T) {
	t.Parallel()

	query := PremiumCodes()

	if query.Query == nil {
		t.Error("expected a query, got nothing")
	}
}

func testPremiumCodesDelete(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	o := &PremiumCode{}
	if err = randomize.Struct(seed, o, premiumCodeDBTypes, true, premiumCodeColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize PremiumCode struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = o.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}

	if rowsAff, err := o.Delete(ctx, tx); err != nil {
		t.Error(err)
	} else if rowsAff != 1 {
		t.Error("should only have deleted one row, but affected:", rowsAff)
	}

	count, err := PremiumCodes().Count(ctx, tx)
	if err != nil {
		t.Error(err)
	}

	if count != 0 {
		t.Error("want zero records, got:", count)
	}
}

func testPremiumCodesQueryDeleteAll(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	o := &PremiumCode{}
	if err = randomize.Struct(seed, o, premiumCodeDBTypes, true, premiumCodeColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize PremiumCode struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = o.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}

	if rowsAff, err := PremiumCodes().DeleteAll(ctx, tx); err != nil {
		t.Error(err)
	} else if rowsAff != 1 {
		t.Error("should only have deleted one row, but affected:", rowsAff)
	}

	count, err := PremiumCodes().Count(ctx, tx)
	if err != nil {
		t.Error(err)
	}

	if count != 0 {
		t.Error("want zero records, got:", count)
	}
}

func testPremiumCodesSliceDeleteAll(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	o := &PremiumCode{}
	if err = randomize.Struct(seed, o, premiumCodeDBTypes, true, premiumCodeColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize PremiumCode struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = o.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}

	slice := PremiumCodeSlice{o}

	if rowsAff, err := slice.DeleteAll(ctx, tx); err != nil {
		t.Error(err)
	} else if rowsAff != 1 {
		t.Error("should only have deleted one row, but affected:", rowsAff)
	}

	count, err := PremiumCodes().Count(ctx, tx)
	if err != nil {
		t.Error(err)
	}

	if count != 0 {
		t.Error("want zero records, got:", count)
	}
}

func testPremiumCodesExists(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	o := &PremiumCode{}
	if err = randomize.Struct(seed, o, premiumCodeDBTypes, true, premiumCodeColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize PremiumCode struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = o.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}

	e, err := PremiumCodeExists(ctx, tx, o.ID)
	if err != nil {
		t.Errorf("Unable to check if PremiumCode exists: %s", err)
	}
	if !e {
		t.Errorf("Expected PremiumCodeExists to return true, but got false.")
	}
}

func testPremiumCodesFind(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	o := &PremiumCode{}
	if err = randomize.Struct(seed, o, premiumCodeDBTypes, true, premiumCodeColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize PremiumCode struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = o.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}

	premiumCodeFound, err := FindPremiumCode(ctx, tx, o.ID)
	if err != nil {
		t.Error(err)
	}

	if premiumCodeFound == nil {
		t.Error("want a record, got nil")
	}
}

func testPremiumCodesBind(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	o := &PremiumCode{}
	if err = randomize.Struct(seed, o, premiumCodeDBTypes, true, premiumCodeColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize PremiumCode struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = o.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}

	if err = PremiumCodes().Bind(ctx, tx, o); err != nil {
		t.Error(err)
	}
}

func testPremiumCodesOne(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	o := &PremiumCode{}
	if err = randomize.Struct(seed, o, premiumCodeDBTypes, true, premiumCodeColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize PremiumCode struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = o.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}

	if x, err := PremiumCodes().One(ctx, tx); err != nil {
		t.Error(err)
	} else if x == nil {
		t.Error("expected to get a non nil record")
	}
}

func testPremiumCodesAll(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	premiumCodeOne := &PremiumCode{}
	premiumCodeTwo := &PremiumCode{}
	if err = randomize.Struct(seed, premiumCodeOne, premiumCodeDBTypes, false, premiumCodeColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize PremiumCode struct: %s", err)
	}
	if err = randomize.Struct(seed, premiumCodeTwo, premiumCodeDBTypes, false, premiumCodeColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize PremiumCode struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = premiumCodeOne.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}
	if err = premiumCodeTwo.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}

	slice, err := PremiumCodes().All(ctx, tx)
	if err != nil {
		t.Error(err)
	}

	if len(slice) != 2 {
		t.Error("want 2 records, got:", len(slice))
	}
}

func testPremiumCodesCount(t *testing.T) {
	t.Parallel()

	var err error
	seed := randomize.NewSeed()
	premiumCodeOne := &PremiumCode{}
	premiumCodeTwo := &PremiumCode{}
	if err = randomize.Struct(seed, premiumCodeOne, premiumCodeDBTypes, false, premiumCodeColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize PremiumCode struct: %s", err)
	}
	if err = randomize.Struct(seed, premiumCodeTwo, premiumCodeDBTypes, false, premiumCodeColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize PremiumCode struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = premiumCodeOne.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}
	if err = premiumCodeTwo.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}

	count, err := PremiumCodes().Count(ctx, tx)
	if err != nil {
		t.Error(err)
	}

	if count != 2 {
		t.Error("want 2 records, got:", count)
	}
}

func testPremiumCodesInsert(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	o := &PremiumCode{}
	if err = randomize.Struct(seed, o, premiumCodeDBTypes, true, premiumCodeColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize PremiumCode struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = o.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}

	count, err := PremiumCodes().Count(ctx, tx)
	if err != nil {
		t.Error(err)
	}

	if count != 1 {
		t.Error("want one record, got:", count)
	}
}

func testPremiumCodesInsertWhitelist(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	o := &PremiumCode{}
	if err = randomize.Struct(seed, o, premiumCodeDBTypes, true); err != nil {
		t.Errorf("Unable to randomize PremiumCode struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = o.Insert(ctx, tx, boil.Whitelist(premiumCodeColumnsWithoutDefault...)); err != nil {
		t.Error(err)
	}

	count, err := PremiumCodes().Count(ctx, tx)
	if err != nil {
		t.Error(err)
	}

	if count != 1 {
		t.Error("want one record, got:", count)
	}
}

func testPremiumCodesReload(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	o := &PremiumCode{}
	if err = randomize.Struct(seed, o, premiumCodeDBTypes, true, premiumCodeColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize PremiumCode struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = o.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}

	if err = o.Reload(ctx, tx); err != nil {
		t.Error(err)
	}
}

func testPremiumCodesReloadAll(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	o := &PremiumCode{}
	if err = randomize.Struct(seed, o, premiumCodeDBTypes, true, premiumCodeColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize PremiumCode struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = o.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}

	slice := PremiumCodeSlice{o}

	if err = slice.ReloadAll(ctx, tx); err != nil {
		t.Error(err)
	}
}

func testPremiumCodesSelect(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	o := &PremiumCode{}
	if err = randomize.Struct(seed, o, premiumCodeDBTypes, true, premiumCodeColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize PremiumCode struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = o.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}

	slice, err := PremiumCodes().All(ctx, tx)
	if err != nil {
		t.Error(err)
	}

	if len(slice) != 1 {
		t.Error("want one record, got:", len(slice))
	}
}

var (
	premiumCodeDBTypes = map[string]string{`AttachedAt`: `timestamp with time zone`, `Code`: `text`, `CreatedAt`: `timestamp with time zone`, `DurationUsed`: `bigint`, `FullDuration`: `bigint`, `GuildID`: `bigint`, `ID`: `bigint`, `Message`: `text`, `Permanent`: `boolean`, `UsedAt`: `timestamp with time zone`, `UserID`: `bigint`}
	_                  = bytes.MinRead
)

func testPremiumCodesUpdate(t *testing.T) {
	t.Parallel()

	if 0 == len(premiumCodePrimaryKeyColumns) {
		t.Skip("Skipping table with no primary key columns")
	}
	if len(premiumCodeColumns) == len(premiumCodePrimaryKeyColumns) {
		t.Skip("Skipping table with only primary key columns")
	}

	seed := randomize.NewSeed()
	var err error
	o := &PremiumCode{}
	if err = randomize.Struct(seed, o, premiumCodeDBTypes, true, premiumCodeColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize PremiumCode struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = o.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}

	count, err := PremiumCodes().Count(ctx, tx)
	if err != nil {
		t.Error(err)
	}

	if count != 1 {
		t.Error("want one record, got:", count)
	}

	if err = randomize.Struct(seed, o, premiumCodeDBTypes, true, premiumCodePrimaryKeyColumns...); err != nil {
		t.Errorf("Unable to randomize PremiumCode struct: %s", err)
	}

	if rowsAff, err := o.Update(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	} else if rowsAff != 1 {
		t.Error("should only affect one row but affected", rowsAff)
	}
}

func testPremiumCodesSliceUpdateAll(t *testing.T) {
	t.Parallel()

	if len(premiumCodeColumns) == len(premiumCodePrimaryKeyColumns) {
		t.Skip("Skipping table with only primary key columns")
	}

	seed := randomize.NewSeed()
	var err error
	o := &PremiumCode{}
	if err = randomize.Struct(seed, o, premiumCodeDBTypes, true, premiumCodeColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize PremiumCode struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = o.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}

	count, err := PremiumCodes().Count(ctx, tx)
	if err != nil {
		t.Error(err)
	}

	if count != 1 {
		t.Error("want one record, got:", count)
	}

	if err = randomize.Struct(seed, o, premiumCodeDBTypes, true, premiumCodePrimaryKeyColumns...); err != nil {
		t.Errorf("Unable to randomize PremiumCode struct: %s", err)
	}

	// Remove Primary keys and unique columns from what we plan to update
	var fields []string
	if strmangle.StringSliceMatch(premiumCodeColumns, premiumCodePrimaryKeyColumns) {
		fields = premiumCodeColumns
	} else {
		fields = strmangle.SetComplement(
			premiumCodeColumns,
			premiumCodePrimaryKeyColumns,
		)
	}

	value := reflect.Indirect(reflect.ValueOf(o))
	typ := reflect.TypeOf(o).Elem()
	n := typ.NumField()

	updateMap := M{}
	for _, col := range fields {
		for i := 0; i < n; i++ {
			f := typ.Field(i)
			if f.Tag.Get("boil") == col {
				updateMap[col] = value.Field(i).Interface()
			}
		}
	}

	slice := PremiumCodeSlice{o}
	if rowsAff, err := slice.UpdateAll(ctx, tx, updateMap); err != nil {
		t.Error(err)
	} else if rowsAff != 1 {
		t.Error("wanted one record updated but got", rowsAff)
	}
}

func testPremiumCodesUpsert(t *testing.T) {
	t.Parallel()

	if len(premiumCodeColumns) == len(premiumCodePrimaryKeyColumns) {
		t.Skip("Skipping table with only primary key columns")
	}

	seed := randomize.NewSeed()
	var err error
	// Attempt the INSERT side of an UPSERT
	o := PremiumCode{}
	if err = randomize.Struct(seed, &o, premiumCodeDBTypes, true); err != nil {
		t.Errorf("Unable to randomize PremiumCode struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = o.Upsert(ctx, tx, false, nil, boil.Infer(), boil.Infer()); err != nil {
		t.Errorf("Unable to upsert PremiumCode: %s", err)
	}

	count, err := PremiumCodes().Count(ctx, tx)
	if err != nil {
		t.Error(err)
	}
	if count != 1 {
		t.Error("want one record, got:", count)
	}

	// Attempt the UPDATE side of an UPSERT
	if err = randomize.Struct(seed, &o, premiumCodeDBTypes, false, premiumCodePrimaryKeyColumns...); err != nil {
		t.Errorf("Unable to randomize PremiumCode struct: %s", err)
	}

	if err = o.Upsert(ctx, tx, true, nil, boil.Infer(), boil.Infer()); err != nil {
		t.Errorf("Unable to upsert PremiumCode: %s", err)
	}

	count, err = PremiumCodes().Count(ctx, tx)
	if err != nil {
		t.Error(err)
	}
	if count != 1 {
		t.Error("want one record, got:", count)
	}
}
