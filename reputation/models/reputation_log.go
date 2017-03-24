package models

import (
	"bytes"
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/vattle/sqlboiler/boil"
	"github.com/vattle/sqlboiler/queries"
	"github.com/vattle/sqlboiler/queries/qm"
	"github.com/vattle/sqlboiler/strmangle"
)

// ReputationLog is an object representing the database table.
type ReputationLog struct {
	ID             int64     `boil:"id" json:"id" toml:"id" yaml:"id"`
	CreatedAt      time.Time `boil:"created_at" json:"created_at" toml:"created_at" yaml:"created_at"`
	GuildID        int64     `boil:"guild_id" json:"guild_id" toml:"guild_id" yaml:"guild_id"`
	SenderID       int64     `boil:"sender_id" json:"sender_id" toml:"sender_id" yaml:"sender_id"`
	ReceiverID     int64     `boil:"receiver_id" json:"receiver_id" toml:"receiver_id" yaml:"receiver_id"`
	SetFixedAmount bool      `boil:"set_fixed_amount" json:"set_fixed_amount" toml:"set_fixed_amount" yaml:"set_fixed_amount"`
	Amount         int64     `boil:"amount" json:"amount" toml:"amount" yaml:"amount"`

	R *reputationLogR `boil:"-" json:"-" toml:"-" yaml:"-"`
	L reputationLogL  `boil:"-" json:"-" toml:"-" yaml:"-"`
}

// reputationLogR is where relationships are stored.
type reputationLogR struct {
}

// reputationLogL is where Load methods for each relationship are stored.
type reputationLogL struct{}

var (
	reputationLogColumns               = []string{"id", "created_at", "guild_id", "sender_id", "receiver_id", "set_fixed_amount", "amount"}
	reputationLogColumnsWithoutDefault = []string{"created_at", "guild_id", "sender_id", "receiver_id", "set_fixed_amount", "amount"}
	reputationLogColumnsWithDefault    = []string{"id"}
	reputationLogPrimaryKeyColumns     = []string{"id"}
)

type (
	// ReputationLogSlice is an alias for a slice of pointers to ReputationLog.
	// This should generally be used opposed to []ReputationLog.
	ReputationLogSlice []*ReputationLog
	// ReputationLogHook is the signature for custom ReputationLog hook methods
	ReputationLogHook func(boil.Executor, *ReputationLog) error

	reputationLogQuery struct {
		*queries.Query
	}
)

// Cache for insert, update and upsert
var (
	reputationLogType                 = reflect.TypeOf(&ReputationLog{})
	reputationLogMapping              = queries.MakeStructMapping(reputationLogType)
	reputationLogPrimaryKeyMapping, _ = queries.BindMapping(reputationLogType, reputationLogMapping, reputationLogPrimaryKeyColumns)
	reputationLogInsertCacheMut       sync.RWMutex
	reputationLogInsertCache          = make(map[string]insertCache)
	reputationLogUpdateCacheMut       sync.RWMutex
	reputationLogUpdateCache          = make(map[string]updateCache)
	reputationLogUpsertCacheMut       sync.RWMutex
	reputationLogUpsertCache          = make(map[string]insertCache)
)

var (
	// Force time package dependency for automated UpdatedAt/CreatedAt.
	_ = time.Second
	// Force bytes in case of primary key column that uses []byte (for relationship compares)
	_ = bytes.MinRead
)
var reputationLogBeforeInsertHooks []ReputationLogHook
var reputationLogBeforeUpdateHooks []ReputationLogHook
var reputationLogBeforeDeleteHooks []ReputationLogHook
var reputationLogBeforeUpsertHooks []ReputationLogHook

var reputationLogAfterInsertHooks []ReputationLogHook
var reputationLogAfterSelectHooks []ReputationLogHook
var reputationLogAfterUpdateHooks []ReputationLogHook
var reputationLogAfterDeleteHooks []ReputationLogHook
var reputationLogAfterUpsertHooks []ReputationLogHook

// doBeforeInsertHooks executes all "before insert" hooks.
func (o *ReputationLog) doBeforeInsertHooks(exec boil.Executor) (err error) {
	for _, hook := range reputationLogBeforeInsertHooks {
		if err := hook(exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doBeforeUpdateHooks executes all "before Update" hooks.
func (o *ReputationLog) doBeforeUpdateHooks(exec boil.Executor) (err error) {
	for _, hook := range reputationLogBeforeUpdateHooks {
		if err := hook(exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doBeforeDeleteHooks executes all "before Delete" hooks.
func (o *ReputationLog) doBeforeDeleteHooks(exec boil.Executor) (err error) {
	for _, hook := range reputationLogBeforeDeleteHooks {
		if err := hook(exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doBeforeUpsertHooks executes all "before Upsert" hooks.
func (o *ReputationLog) doBeforeUpsertHooks(exec boil.Executor) (err error) {
	for _, hook := range reputationLogBeforeUpsertHooks {
		if err := hook(exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doAfterInsertHooks executes all "after Insert" hooks.
func (o *ReputationLog) doAfterInsertHooks(exec boil.Executor) (err error) {
	for _, hook := range reputationLogAfterInsertHooks {
		if err := hook(exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doAfterSelectHooks executes all "after Select" hooks.
func (o *ReputationLog) doAfterSelectHooks(exec boil.Executor) (err error) {
	for _, hook := range reputationLogAfterSelectHooks {
		if err := hook(exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doAfterUpdateHooks executes all "after Update" hooks.
func (o *ReputationLog) doAfterUpdateHooks(exec boil.Executor) (err error) {
	for _, hook := range reputationLogAfterUpdateHooks {
		if err := hook(exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doAfterDeleteHooks executes all "after Delete" hooks.
func (o *ReputationLog) doAfterDeleteHooks(exec boil.Executor) (err error) {
	for _, hook := range reputationLogAfterDeleteHooks {
		if err := hook(exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doAfterUpsertHooks executes all "after Upsert" hooks.
func (o *ReputationLog) doAfterUpsertHooks(exec boil.Executor) (err error) {
	for _, hook := range reputationLogAfterUpsertHooks {
		if err := hook(exec, o); err != nil {
			return err
		}
	}

	return nil
}

// AddReputationLogHook registers your hook function for all future operations.
func AddReputationLogHook(hookPoint boil.HookPoint, reputationLogHook ReputationLogHook) {
	switch hookPoint {
	case boil.BeforeInsertHook:
		reputationLogBeforeInsertHooks = append(reputationLogBeforeInsertHooks, reputationLogHook)
	case boil.BeforeUpdateHook:
		reputationLogBeforeUpdateHooks = append(reputationLogBeforeUpdateHooks, reputationLogHook)
	case boil.BeforeDeleteHook:
		reputationLogBeforeDeleteHooks = append(reputationLogBeforeDeleteHooks, reputationLogHook)
	case boil.BeforeUpsertHook:
		reputationLogBeforeUpsertHooks = append(reputationLogBeforeUpsertHooks, reputationLogHook)
	case boil.AfterInsertHook:
		reputationLogAfterInsertHooks = append(reputationLogAfterInsertHooks, reputationLogHook)
	case boil.AfterSelectHook:
		reputationLogAfterSelectHooks = append(reputationLogAfterSelectHooks, reputationLogHook)
	case boil.AfterUpdateHook:
		reputationLogAfterUpdateHooks = append(reputationLogAfterUpdateHooks, reputationLogHook)
	case boil.AfterDeleteHook:
		reputationLogAfterDeleteHooks = append(reputationLogAfterDeleteHooks, reputationLogHook)
	case boil.AfterUpsertHook:
		reputationLogAfterUpsertHooks = append(reputationLogAfterUpsertHooks, reputationLogHook)
	}
}

// OneP returns a single reputationLog record from the query, and panics on error.
func (q reputationLogQuery) OneP() *ReputationLog {
	o, err := q.One()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// One returns a single reputationLog record from the query.
func (q reputationLogQuery) One() (*ReputationLog, error) {
	o := &ReputationLog{}

	queries.SetLimit(q.Query, 1)

	err := q.Bind(o)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "models: failed to execute a one query for reputation_log")
	}

	if err := o.doAfterSelectHooks(queries.GetExecutor(q.Query)); err != nil {
		return o, err
	}

	return o, nil
}

// AllP returns all ReputationLog records from the query, and panics on error.
func (q reputationLogQuery) AllP() ReputationLogSlice {
	o, err := q.All()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// All returns all ReputationLog records from the query.
func (q reputationLogQuery) All() (ReputationLogSlice, error) {
	var o ReputationLogSlice

	err := q.Bind(&o)
	if err != nil {
		return nil, errors.Wrap(err, "models: failed to assign all query results to ReputationLog slice")
	}

	if len(reputationLogAfterSelectHooks) != 0 {
		for _, obj := range o {
			if err := obj.doAfterSelectHooks(queries.GetExecutor(q.Query)); err != nil {
				return o, err
			}
		}
	}

	return o, nil
}

// CountP returns the count of all ReputationLog records in the query, and panics on error.
func (q reputationLogQuery) CountP() int64 {
	c, err := q.Count()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return c
}

// Count returns the count of all ReputationLog records in the query.
func (q reputationLogQuery) Count() (int64, error) {
	var count int64

	queries.SetSelect(q.Query, nil)
	queries.SetCount(q.Query)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "models: failed to count reputation_log rows")
	}

	return count, nil
}

// Exists checks if the row exists in the table, and panics on error.
func (q reputationLogQuery) ExistsP() bool {
	e, err := q.Exists()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// Exists checks if the row exists in the table.
func (q reputationLogQuery) Exists() (bool, error) {
	var count int64

	queries.SetCount(q.Query)
	queries.SetLimit(q.Query, 1)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return false, errors.Wrap(err, "models: failed to check if reputation_log exists")
	}

	return count > 0, nil
}

// ReputationLogsG retrieves all records.
func ReputationLogsG(mods ...qm.QueryMod) reputationLogQuery {
	return ReputationLogs(boil.GetDB(), mods...)
}

// ReputationLogs retrieves all the records using an executor.
func ReputationLogs(exec boil.Executor, mods ...qm.QueryMod) reputationLogQuery {
	mods = append(mods, qm.From("\"reputation_log\""))
	return reputationLogQuery{NewQuery(exec, mods...)}
}

// FindReputationLogG retrieves a single record by ID.
func FindReputationLogG(id int64, selectCols ...string) (*ReputationLog, error) {
	return FindReputationLog(boil.GetDB(), id, selectCols...)
}

// FindReputationLogGP retrieves a single record by ID, and panics on error.
func FindReputationLogGP(id int64, selectCols ...string) *ReputationLog {
	retobj, err := FindReputationLog(boil.GetDB(), id, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// FindReputationLog retrieves a single record by ID with an executor.
// If selectCols is empty Find will return all columns.
func FindReputationLog(exec boil.Executor, id int64, selectCols ...string) (*ReputationLog, error) {
	reputationLogObj := &ReputationLog{}

	sel := "*"
	if len(selectCols) > 0 {
		sel = strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, selectCols), ",")
	}
	query := fmt.Sprintf(
		"select %s from \"reputation_log\" where \"id\"=$1", sel,
	)

	q := queries.Raw(exec, query, id)

	err := q.Bind(reputationLogObj)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "models: unable to select from reputation_log")
	}

	return reputationLogObj, nil
}

// FindReputationLogP retrieves a single record by ID with an executor, and panics on error.
func FindReputationLogP(exec boil.Executor, id int64, selectCols ...string) *ReputationLog {
	retobj, err := FindReputationLog(exec, id, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// InsertG a single record. See Insert for whitelist behavior description.
func (o *ReputationLog) InsertG(whitelist ...string) error {
	return o.Insert(boil.GetDB(), whitelist...)
}

// InsertGP a single record, and panics on error. See Insert for whitelist
// behavior description.
func (o *ReputationLog) InsertGP(whitelist ...string) {
	if err := o.Insert(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// InsertP a single record using an executor, and panics on error. See Insert
// for whitelist behavior description.
func (o *ReputationLog) InsertP(exec boil.Executor, whitelist ...string) {
	if err := o.Insert(exec, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Insert a single record using an executor.
// Whitelist behavior: If a whitelist is provided, only those columns supplied are inserted
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns without a default value are included (i.e. name, age)
// - All columns with a default, but non-zero are included (i.e. health = 75)
func (o *ReputationLog) Insert(exec boil.Executor, whitelist ...string) error {
	if o == nil {
		return errors.New("models: no reputation_log provided for insertion")
	}

	var err error
	currTime := time.Now().In(boil.GetLocation())

	if o.CreatedAt.IsZero() {
		o.CreatedAt = currTime
	}

	if err := o.doBeforeInsertHooks(exec); err != nil {
		return err
	}

	nzDefaults := queries.NonZeroDefaultSet(reputationLogColumnsWithDefault, o)

	key := makeCacheKey(whitelist, nzDefaults)
	reputationLogInsertCacheMut.RLock()
	cache, cached := reputationLogInsertCache[key]
	reputationLogInsertCacheMut.RUnlock()

	if !cached {
		wl, returnColumns := strmangle.InsertColumnSet(
			reputationLogColumns,
			reputationLogColumnsWithDefault,
			reputationLogColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)

		cache.valueMapping, err = queries.BindMapping(reputationLogType, reputationLogMapping, wl)
		if err != nil {
			return err
		}
		cache.retMapping, err = queries.BindMapping(reputationLogType, reputationLogMapping, returnColumns)
		if err != nil {
			return err
		}
		cache.query = fmt.Sprintf("INSERT INTO \"reputation_log\" (\"%s\") VALUES (%s)", strings.Join(wl, "\",\""), strmangle.Placeholders(dialect.IndexPlaceholders, len(wl), 1, 1))

		if len(cache.retMapping) != 0 {
			cache.query += fmt.Sprintf(" RETURNING \"%s\"", strings.Join(returnColumns, "\",\""))
		}
	}

	value := reflect.Indirect(reflect.ValueOf(o))
	vals := queries.ValuesFromMapping(value, cache.valueMapping)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, cache.query)
		fmt.Fprintln(boil.DebugWriter, vals)
	}

	if len(cache.retMapping) != 0 {
		err = exec.QueryRow(cache.query, vals...).Scan(queries.PtrsFromMapping(value, cache.retMapping)...)
	} else {
		_, err = exec.Exec(cache.query, vals...)
	}

	if err != nil {
		return errors.Wrap(err, "models: unable to insert into reputation_log")
	}

	if !cached {
		reputationLogInsertCacheMut.Lock()
		reputationLogInsertCache[key] = cache
		reputationLogInsertCacheMut.Unlock()
	}

	return o.doAfterInsertHooks(exec)
}

// UpdateG a single ReputationLog record. See Update for
// whitelist behavior description.
func (o *ReputationLog) UpdateG(whitelist ...string) error {
	return o.Update(boil.GetDB(), whitelist...)
}

// UpdateGP a single ReputationLog record.
// UpdateGP takes a whitelist of column names that should be updated.
// Panics on error. See Update for whitelist behavior description.
func (o *ReputationLog) UpdateGP(whitelist ...string) {
	if err := o.Update(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateP uses an executor to update the ReputationLog, and panics on error.
// See Update for whitelist behavior description.
func (o *ReputationLog) UpdateP(exec boil.Executor, whitelist ...string) {
	err := o.Update(exec, whitelist...)
	if err != nil {
		panic(boil.WrapErr(err))
	}
}

// Update uses an executor to update the ReputationLog.
// Whitelist behavior: If a whitelist is provided, only the columns given are updated.
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns are inferred to start with
// - All primary keys are subtracted from this set
// Update does not automatically update the record in case of default values. Use .Reload()
// to refresh the records.
func (o *ReputationLog) Update(exec boil.Executor, whitelist ...string) error {
	var err error
	if err = o.doBeforeUpdateHooks(exec); err != nil {
		return err
	}
	key := makeCacheKey(whitelist, nil)
	reputationLogUpdateCacheMut.RLock()
	cache, cached := reputationLogUpdateCache[key]
	reputationLogUpdateCacheMut.RUnlock()

	if !cached {
		wl := strmangle.UpdateColumnSet(reputationLogColumns, reputationLogPrimaryKeyColumns, whitelist)
		if len(wl) == 0 {
			return errors.New("models: unable to update reputation_log, could not build whitelist")
		}

		cache.query = fmt.Sprintf("UPDATE \"reputation_log\" SET %s WHERE %s",
			strmangle.SetParamNames("\"", "\"", 1, wl),
			strmangle.WhereClause("\"", "\"", len(wl)+1, reputationLogPrimaryKeyColumns),
		)
		cache.valueMapping, err = queries.BindMapping(reputationLogType, reputationLogMapping, append(wl, reputationLogPrimaryKeyColumns...))
		if err != nil {
			return err
		}
	}

	values := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(o)), cache.valueMapping)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, cache.query)
		fmt.Fprintln(boil.DebugWriter, values)
	}

	_, err = exec.Exec(cache.query, values...)
	if err != nil {
		return errors.Wrap(err, "models: unable to update reputation_log row")
	}

	if !cached {
		reputationLogUpdateCacheMut.Lock()
		reputationLogUpdateCache[key] = cache
		reputationLogUpdateCacheMut.Unlock()
	}

	return o.doAfterUpdateHooks(exec)
}

// UpdateAllP updates all rows with matching column names, and panics on error.
func (q reputationLogQuery) UpdateAllP(cols M) {
	if err := q.UpdateAll(cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values.
func (q reputationLogQuery) UpdateAll(cols M) error {
	queries.SetUpdate(q.Query, cols)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "models: unable to update all for reputation_log")
	}

	return nil
}

// UpdateAllG updates all rows with the specified column values.
func (o ReputationLogSlice) UpdateAllG(cols M) error {
	return o.UpdateAll(boil.GetDB(), cols)
}

// UpdateAllGP updates all rows with the specified column values, and panics on error.
func (o ReputationLogSlice) UpdateAllGP(cols M) {
	if err := o.UpdateAll(boil.GetDB(), cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAllP updates all rows with the specified column values, and panics on error.
func (o ReputationLogSlice) UpdateAllP(exec boil.Executor, cols M) {
	if err := o.UpdateAll(exec, cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values, using an executor.
func (o ReputationLogSlice) UpdateAll(exec boil.Executor, cols M) error {
	ln := int64(len(o))
	if ln == 0 {
		return nil
	}

	if len(cols) == 0 {
		return errors.New("models: update all requires at least one column argument")
	}

	colNames := make([]string, len(cols))
	args := make([]interface{}, len(cols))

	i := 0
	for name, value := range cols {
		colNames[i] = name
		args[i] = value
		i++
	}

	// Append all of the primary key values for each column
	for _, obj := range o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), reputationLogPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"UPDATE \"reputation_log\" SET %s WHERE (\"id\") IN (%s)",
		strmangle.SetParamNames("\"", "\"", 1, colNames),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(reputationLogPrimaryKeyColumns), len(colNames)+1, len(reputationLogPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "models: unable to update all in reputationLog slice")
	}

	return nil
}

// UpsertG attempts an insert, and does an update or ignore on conflict.
func (o *ReputationLog) UpsertG(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	return o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...)
}

// UpsertGP attempts an insert, and does an update or ignore on conflict. Panics on error.
func (o *ReputationLog) UpsertGP(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpsertP attempts an insert using an executor, and does an update or ignore on conflict.
// UpsertP panics on error.
func (o *ReputationLog) UpsertP(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(exec, updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Upsert attempts an insert using an executor, and does an update or ignore on conflict.
func (o *ReputationLog) Upsert(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	if o == nil {
		return errors.New("models: no reputation_log provided for upsert")
	}
	currTime := time.Now().In(boil.GetLocation())

	if o.CreatedAt.IsZero() {
		o.CreatedAt = currTime
	}

	if err := o.doBeforeUpsertHooks(exec); err != nil {
		return err
	}

	nzDefaults := queries.NonZeroDefaultSet(reputationLogColumnsWithDefault, o)

	// Build cache key in-line uglily - mysql vs postgres problems
	buf := strmangle.GetBuffer()
	if updateOnConflict {
		buf.WriteByte('t')
	} else {
		buf.WriteByte('f')
	}
	buf.WriteByte('.')
	for _, c := range conflictColumns {
		buf.WriteString(c)
	}
	buf.WriteByte('.')
	for _, c := range updateColumns {
		buf.WriteString(c)
	}
	buf.WriteByte('.')
	for _, c := range whitelist {
		buf.WriteString(c)
	}
	buf.WriteByte('.')
	for _, c := range nzDefaults {
		buf.WriteString(c)
	}
	key := buf.String()
	strmangle.PutBuffer(buf)

	reputationLogUpsertCacheMut.RLock()
	cache, cached := reputationLogUpsertCache[key]
	reputationLogUpsertCacheMut.RUnlock()

	var err error

	if !cached {
		var ret []string
		whitelist, ret = strmangle.InsertColumnSet(
			reputationLogColumns,
			reputationLogColumnsWithDefault,
			reputationLogColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)
		update := strmangle.UpdateColumnSet(
			reputationLogColumns,
			reputationLogPrimaryKeyColumns,
			updateColumns,
		)
		if len(update) == 0 {
			return errors.New("models: unable to upsert reputation_log, could not build update column list")
		}

		conflict := conflictColumns
		if len(conflict) == 0 {
			conflict = make([]string, len(reputationLogPrimaryKeyColumns))
			copy(conflict, reputationLogPrimaryKeyColumns)
		}
		cache.query = queries.BuildUpsertQueryPostgres(dialect, "\"reputation_log\"", updateOnConflict, ret, update, conflict, whitelist)

		cache.valueMapping, err = queries.BindMapping(reputationLogType, reputationLogMapping, whitelist)
		if err != nil {
			return err
		}
		if len(ret) != 0 {
			cache.retMapping, err = queries.BindMapping(reputationLogType, reputationLogMapping, ret)
			if err != nil {
				return err
			}
		}
	}

	value := reflect.Indirect(reflect.ValueOf(o))
	vals := queries.ValuesFromMapping(value, cache.valueMapping)
	var returns []interface{}
	if len(cache.retMapping) != 0 {
		returns = queries.PtrsFromMapping(value, cache.retMapping)
	}

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, cache.query)
		fmt.Fprintln(boil.DebugWriter, vals)
	}

	if len(cache.retMapping) != 0 {
		err = exec.QueryRow(cache.query, vals...).Scan(returns...)
		if err == sql.ErrNoRows {
			err = nil // Postgres doesn't return anything when there's no update
		}
	} else {
		_, err = exec.Exec(cache.query, vals...)
	}
	if err != nil {
		return errors.Wrap(err, "models: unable to upsert reputation_log")
	}

	if !cached {
		reputationLogUpsertCacheMut.Lock()
		reputationLogUpsertCache[key] = cache
		reputationLogUpsertCacheMut.Unlock()
	}

	return o.doAfterUpsertHooks(exec)
}

// DeleteP deletes a single ReputationLog record with an executor.
// DeleteP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *ReputationLog) DeleteP(exec boil.Executor) {
	if err := o.Delete(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteG deletes a single ReputationLog record.
// DeleteG will match against the primary key column to find the record to delete.
func (o *ReputationLog) DeleteG() error {
	if o == nil {
		return errors.New("models: no ReputationLog provided for deletion")
	}

	return o.Delete(boil.GetDB())
}

// DeleteGP deletes a single ReputationLog record.
// DeleteGP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *ReputationLog) DeleteGP() {
	if err := o.DeleteG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Delete deletes a single ReputationLog record with an executor.
// Delete will match against the primary key column to find the record to delete.
func (o *ReputationLog) Delete(exec boil.Executor) error {
	if o == nil {
		return errors.New("models: no ReputationLog provided for delete")
	}

	if err := o.doBeforeDeleteHooks(exec); err != nil {
		return err
	}

	args := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(o)), reputationLogPrimaryKeyMapping)
	sql := "DELETE FROM \"reputation_log\" WHERE \"id\"=$1"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "models: unable to delete from reputation_log")
	}

	if err := o.doAfterDeleteHooks(exec); err != nil {
		return err
	}

	return nil
}

// DeleteAllP deletes all rows, and panics on error.
func (q reputationLogQuery) DeleteAllP() {
	if err := q.DeleteAll(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all matching rows.
func (q reputationLogQuery) DeleteAll() error {
	if q.Query == nil {
		return errors.New("models: no reputationLogQuery provided for delete all")
	}

	queries.SetDelete(q.Query)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "models: unable to delete all from reputation_log")
	}

	return nil
}

// DeleteAllGP deletes all rows in the slice, and panics on error.
func (o ReputationLogSlice) DeleteAllGP() {
	if err := o.DeleteAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAllG deletes all rows in the slice.
func (o ReputationLogSlice) DeleteAllG() error {
	if o == nil {
		return errors.New("models: no ReputationLog slice provided for delete all")
	}
	return o.DeleteAll(boil.GetDB())
}

// DeleteAllP deletes all rows in the slice, using an executor, and panics on error.
func (o ReputationLogSlice) DeleteAllP(exec boil.Executor) {
	if err := o.DeleteAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all rows in the slice, using an executor.
func (o ReputationLogSlice) DeleteAll(exec boil.Executor) error {
	if o == nil {
		return errors.New("models: no ReputationLog slice provided for delete all")
	}

	if len(o) == 0 {
		return nil
	}

	if len(reputationLogBeforeDeleteHooks) != 0 {
		for _, obj := range o {
			if err := obj.doBeforeDeleteHooks(exec); err != nil {
				return err
			}
		}
	}

	var args []interface{}
	for _, obj := range o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), reputationLogPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"DELETE FROM \"reputation_log\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, reputationLogPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(reputationLogPrimaryKeyColumns), 1, len(reputationLogPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "models: unable to delete all from reputationLog slice")
	}

	if len(reputationLogAfterDeleteHooks) != 0 {
		for _, obj := range o {
			if err := obj.doAfterDeleteHooks(exec); err != nil {
				return err
			}
		}
	}

	return nil
}

// ReloadGP refetches the object from the database and panics on error.
func (o *ReputationLog) ReloadGP() {
	if err := o.ReloadG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadP refetches the object from the database with an executor. Panics on error.
func (o *ReputationLog) ReloadP(exec boil.Executor) {
	if err := o.Reload(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadG refetches the object from the database using the primary keys.
func (o *ReputationLog) ReloadG() error {
	if o == nil {
		return errors.New("models: no ReputationLog provided for reload")
	}

	return o.Reload(boil.GetDB())
}

// Reload refetches the object from the database
// using the primary keys with an executor.
func (o *ReputationLog) Reload(exec boil.Executor) error {
	ret, err := FindReputationLog(exec, o.ID)
	if err != nil {
		return err
	}

	*o = *ret
	return nil
}

// ReloadAllGP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *ReputationLogSlice) ReloadAllGP() {
	if err := o.ReloadAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *ReputationLogSlice) ReloadAllP(exec boil.Executor) {
	if err := o.ReloadAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllG refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *ReputationLogSlice) ReloadAllG() error {
	if o == nil {
		return errors.New("models: empty ReputationLogSlice provided for reload all")
	}

	return o.ReloadAll(boil.GetDB())
}

// ReloadAll refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *ReputationLogSlice) ReloadAll(exec boil.Executor) error {
	if o == nil || len(*o) == 0 {
		return nil
	}

	reputationLogs := ReputationLogSlice{}
	var args []interface{}
	for _, obj := range *o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), reputationLogPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"SELECT \"reputation_log\".* FROM \"reputation_log\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, reputationLogPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(*o)*len(reputationLogPrimaryKeyColumns), 1, len(reputationLogPrimaryKeyColumns)),
	)

	q := queries.Raw(exec, sql, args...)

	err := q.Bind(&reputationLogs)
	if err != nil {
		return errors.Wrap(err, "models: unable to reload all in ReputationLogSlice")
	}

	*o = reputationLogs

	return nil
}

// ReputationLogExists checks if the ReputationLog row exists.
func ReputationLogExists(exec boil.Executor, id int64) (bool, error) {
	var exists bool

	sql := "select exists(select 1 from \"reputation_log\" where \"id\"=$1 limit 1)"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, id)
	}

	row := exec.QueryRow(sql, id)

	err := row.Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, "models: unable to check if reputation_log exists")
	}

	return exists, nil
}

// ReputationLogExistsG checks if the ReputationLog row exists.
func ReputationLogExistsG(id int64) (bool, error) {
	return ReputationLogExists(boil.GetDB(), id)
}

// ReputationLogExistsGP checks if the ReputationLog row exists. Panics on error.
func ReputationLogExistsGP(id int64) bool {
	e, err := ReputationLogExists(boil.GetDB(), id)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// ReputationLogExistsP checks if the ReputationLog row exists. Panics on error.
func ReputationLogExistsP(exec boil.Executor, id int64) bool {
	e, err := ReputationLogExists(exec, id)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}
