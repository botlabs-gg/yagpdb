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

// ReputationUser is an object representing the database table.
type ReputationUser struct {
	UserID    int64     `boil:"user_id" json:"user_id" toml:"user_id" yaml:"user_id"`
	GuildID   int64     `boil:"guild_id" json:"guild_id" toml:"guild_id" yaml:"guild_id"`
	CreatedAt time.Time `boil:"created_at" json:"created_at" toml:"created_at" yaml:"created_at"`
	Points    int64     `boil:"points" json:"points" toml:"points" yaml:"points"`

	R *reputationUserR `boil:"-" json:"-" toml:"-" yaml:"-"`
	L reputationUserL  `boil:"-" json:"-" toml:"-" yaml:"-"`
}

// reputationUserR is where relationships are stored.
type reputationUserR struct {
}

// reputationUserL is where Load methods for each relationship are stored.
type reputationUserL struct{}

var (
	reputationUserColumns               = []string{"user_id", "guild_id", "created_at", "points"}
	reputationUserColumnsWithoutDefault = []string{"user_id", "guild_id", "created_at", "points"}
	reputationUserColumnsWithDefault    = []string{}
	reputationUserPrimaryKeyColumns     = []string{"user_id", "guild_id"}
)

type (
	// ReputationUserSlice is an alias for a slice of pointers to ReputationUser.
	// This should generally be used opposed to []ReputationUser.
	ReputationUserSlice []*ReputationUser

	reputationUserQuery struct {
		*queries.Query
	}
)

// Cache for insert, update and upsert
var (
	reputationUserType                 = reflect.TypeOf(&ReputationUser{})
	reputationUserMapping              = queries.MakeStructMapping(reputationUserType)
	reputationUserPrimaryKeyMapping, _ = queries.BindMapping(reputationUserType, reputationUserMapping, reputationUserPrimaryKeyColumns)
	reputationUserInsertCacheMut       sync.RWMutex
	reputationUserInsertCache          = make(map[string]insertCache)
	reputationUserUpdateCacheMut       sync.RWMutex
	reputationUserUpdateCache          = make(map[string]updateCache)
	reputationUserUpsertCacheMut       sync.RWMutex
	reputationUserUpsertCache          = make(map[string]insertCache)
)

var (
	// Force time package dependency for automated UpdatedAt/CreatedAt.
	_ = time.Second
	// Force bytes in case of primary key column that uses []byte (for relationship compares)
	_ = bytes.MinRead
)

// OneP returns a single reputationUser record from the query, and panics on error.
func (q reputationUserQuery) OneP() *ReputationUser {
	o, err := q.One()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// One returns a single reputationUser record from the query.
func (q reputationUserQuery) One() (*ReputationUser, error) {
	o := &ReputationUser{}

	queries.SetLimit(q.Query, 1)

	err := q.Bind(o)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "models: failed to execute a one query for reputation_users")
	}

	return o, nil
}

// AllP returns all ReputationUser records from the query, and panics on error.
func (q reputationUserQuery) AllP() ReputationUserSlice {
	o, err := q.All()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return o
}

// All returns all ReputationUser records from the query.
func (q reputationUserQuery) All() (ReputationUserSlice, error) {
	var o ReputationUserSlice

	err := q.Bind(&o)
	if err != nil {
		return nil, errors.Wrap(err, "models: failed to assign all query results to ReputationUser slice")
	}

	return o, nil
}

// CountP returns the count of all ReputationUser records in the query, and panics on error.
func (q reputationUserQuery) CountP() int64 {
	c, err := q.Count()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return c
}

// Count returns the count of all ReputationUser records in the query.
func (q reputationUserQuery) Count() (int64, error) {
	var count int64

	queries.SetSelect(q.Query, nil)
	queries.SetCount(q.Query)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "models: failed to count reputation_users rows")
	}

	return count, nil
}

// Exists checks if the row exists in the table, and panics on error.
func (q reputationUserQuery) ExistsP() bool {
	e, err := q.Exists()
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// Exists checks if the row exists in the table.
func (q reputationUserQuery) Exists() (bool, error) {
	var count int64

	queries.SetCount(q.Query)
	queries.SetLimit(q.Query, 1)

	err := q.Query.QueryRow().Scan(&count)
	if err != nil {
		return false, errors.Wrap(err, "models: failed to check if reputation_users exists")
	}

	return count > 0, nil
}

// ReputationUsersG retrieves all records.
func ReputationUsersG(mods ...qm.QueryMod) reputationUserQuery {
	return ReputationUsers(boil.GetDB(), mods...)
}

// ReputationUsers retrieves all the records using an executor.
func ReputationUsers(exec boil.Executor, mods ...qm.QueryMod) reputationUserQuery {
	mods = append(mods, qm.From("\"reputation_users\""))
	return reputationUserQuery{NewQuery(exec, mods...)}
}

// FindReputationUserG retrieves a single record by ID.
func FindReputationUserG(userID int64, guildID int64, selectCols ...string) (*ReputationUser, error) {
	return FindReputationUser(boil.GetDB(), userID, guildID, selectCols...)
}

// FindReputationUserGP retrieves a single record by ID, and panics on error.
func FindReputationUserGP(userID int64, guildID int64, selectCols ...string) *ReputationUser {
	retobj, err := FindReputationUser(boil.GetDB(), userID, guildID, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// FindReputationUser retrieves a single record by ID with an executor.
// If selectCols is empty Find will return all columns.
func FindReputationUser(exec boil.Executor, userID int64, guildID int64, selectCols ...string) (*ReputationUser, error) {
	reputationUserObj := &ReputationUser{}

	sel := "*"
	if len(selectCols) > 0 {
		sel = strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, selectCols), ",")
	}
	query := fmt.Sprintf(
		"select %s from \"reputation_users\" where \"user_id\"=$1 AND \"guild_id\"=$2", sel,
	)

	q := queries.Raw(exec, query, userID, guildID)

	err := q.Bind(reputationUserObj)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "models: unable to select from reputation_users")
	}

	return reputationUserObj, nil
}

// FindReputationUserP retrieves a single record by ID with an executor, and panics on error.
func FindReputationUserP(exec boil.Executor, userID int64, guildID int64, selectCols ...string) *ReputationUser {
	retobj, err := FindReputationUser(exec, userID, guildID, selectCols...)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return retobj
}

// InsertG a single record. See Insert for whitelist behavior description.
func (o *ReputationUser) InsertG(whitelist ...string) error {
	return o.Insert(boil.GetDB(), whitelist...)
}

// InsertGP a single record, and panics on error. See Insert for whitelist
// behavior description.
func (o *ReputationUser) InsertGP(whitelist ...string) {
	if err := o.Insert(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// InsertP a single record using an executor, and panics on error. See Insert
// for whitelist behavior description.
func (o *ReputationUser) InsertP(exec boil.Executor, whitelist ...string) {
	if err := o.Insert(exec, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Insert a single record using an executor.
// Whitelist behavior: If a whitelist is provided, only those columns supplied are inserted
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns without a default value are included (i.e. name, age)
// - All columns with a default, but non-zero are included (i.e. health = 75)
func (o *ReputationUser) Insert(exec boil.Executor, whitelist ...string) error {
	if o == nil {
		return errors.New("models: no reputation_users provided for insertion")
	}

	var err error
	currTime := time.Now().In(boil.GetLocation())

	if o.CreatedAt.IsZero() {
		o.CreatedAt = currTime
	}

	nzDefaults := queries.NonZeroDefaultSet(reputationUserColumnsWithDefault, o)

	key := makeCacheKey(whitelist, nzDefaults)
	reputationUserInsertCacheMut.RLock()
	cache, cached := reputationUserInsertCache[key]
	reputationUserInsertCacheMut.RUnlock()

	if !cached {
		wl, returnColumns := strmangle.InsertColumnSet(
			reputationUserColumns,
			reputationUserColumnsWithDefault,
			reputationUserColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)

		cache.valueMapping, err = queries.BindMapping(reputationUserType, reputationUserMapping, wl)
		if err != nil {
			return err
		}
		cache.retMapping, err = queries.BindMapping(reputationUserType, reputationUserMapping, returnColumns)
		if err != nil {
			return err
		}
		cache.query = fmt.Sprintf("INSERT INTO \"reputation_users\" (\"%s\") VALUES (%s)", strings.Join(wl, "\",\""), strmangle.Placeholders(dialect.IndexPlaceholders, len(wl), 1, 1))

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
		return errors.Wrap(err, "models: unable to insert into reputation_users")
	}

	if !cached {
		reputationUserInsertCacheMut.Lock()
		reputationUserInsertCache[key] = cache
		reputationUserInsertCacheMut.Unlock()
	}

	return nil
}

// UpdateG a single ReputationUser record. See Update for
// whitelist behavior description.
func (o *ReputationUser) UpdateG(whitelist ...string) error {
	return o.Update(boil.GetDB(), whitelist...)
}

// UpdateGP a single ReputationUser record.
// UpdateGP takes a whitelist of column names that should be updated.
// Panics on error. See Update for whitelist behavior description.
func (o *ReputationUser) UpdateGP(whitelist ...string) {
	if err := o.Update(boil.GetDB(), whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateP uses an executor to update the ReputationUser, and panics on error.
// See Update for whitelist behavior description.
func (o *ReputationUser) UpdateP(exec boil.Executor, whitelist ...string) {
	err := o.Update(exec, whitelist...)
	if err != nil {
		panic(boil.WrapErr(err))
	}
}

// Update uses an executor to update the ReputationUser.
// Whitelist behavior: If a whitelist is provided, only the columns given are updated.
// No whitelist behavior: Without a whitelist, columns are inferred by the following rules:
// - All columns are inferred to start with
// - All primary keys are subtracted from this set
// Update does not automatically update the record in case of default values. Use .Reload()
// to refresh the records.
func (o *ReputationUser) Update(exec boil.Executor, whitelist ...string) error {
	var err error
	key := makeCacheKey(whitelist, nil)
	reputationUserUpdateCacheMut.RLock()
	cache, cached := reputationUserUpdateCache[key]
	reputationUserUpdateCacheMut.RUnlock()

	if !cached {
		wl := strmangle.UpdateColumnSet(reputationUserColumns, reputationUserPrimaryKeyColumns, whitelist)
		if len(wl) == 0 {
			return errors.New("models: unable to update reputation_users, could not build whitelist")
		}

		cache.query = fmt.Sprintf("UPDATE \"reputation_users\" SET %s WHERE %s",
			strmangle.SetParamNames("\"", "\"", 1, wl),
			strmangle.WhereClause("\"", "\"", len(wl)+1, reputationUserPrimaryKeyColumns),
		)
		cache.valueMapping, err = queries.BindMapping(reputationUserType, reputationUserMapping, append(wl, reputationUserPrimaryKeyColumns...))
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
		return errors.Wrap(err, "models: unable to update reputation_users row")
	}

	if !cached {
		reputationUserUpdateCacheMut.Lock()
		reputationUserUpdateCache[key] = cache
		reputationUserUpdateCacheMut.Unlock()
	}

	return nil
}

// UpdateAllP updates all rows with matching column names, and panics on error.
func (q reputationUserQuery) UpdateAllP(cols M) {
	if err := q.UpdateAll(cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values.
func (q reputationUserQuery) UpdateAll(cols M) error {
	queries.SetUpdate(q.Query, cols)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "models: unable to update all for reputation_users")
	}

	return nil
}

// UpdateAllG updates all rows with the specified column values.
func (o ReputationUserSlice) UpdateAllG(cols M) error {
	return o.UpdateAll(boil.GetDB(), cols)
}

// UpdateAllGP updates all rows with the specified column values, and panics on error.
func (o ReputationUserSlice) UpdateAllGP(cols M) {
	if err := o.UpdateAll(boil.GetDB(), cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAllP updates all rows with the specified column values, and panics on error.
func (o ReputationUserSlice) UpdateAllP(exec boil.Executor, cols M) {
	if err := o.UpdateAll(exec, cols); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpdateAll updates all rows with the specified column values, using an executor.
func (o ReputationUserSlice) UpdateAll(exec boil.Executor, cols M) error {
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
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), reputationUserPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"UPDATE \"reputation_users\" SET %s WHERE (\"user_id\",\"guild_id\") IN (%s)",
		strmangle.SetParamNames("\"", "\"", 1, colNames),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(reputationUserPrimaryKeyColumns), len(colNames)+1, len(reputationUserPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "models: unable to update all in reputationUser slice")
	}

	return nil
}

// UpsertG attempts an insert, and does an update or ignore on conflict.
func (o *ReputationUser) UpsertG(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	return o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...)
}

// UpsertGP attempts an insert, and does an update or ignore on conflict. Panics on error.
func (o *ReputationUser) UpsertGP(updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// UpsertP attempts an insert using an executor, and does an update or ignore on conflict.
// UpsertP panics on error.
func (o *ReputationUser) UpsertP(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) {
	if err := o.Upsert(exec, updateOnConflict, conflictColumns, updateColumns, whitelist...); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Upsert attempts an insert using an executor, and does an update or ignore on conflict.
func (o *ReputationUser) Upsert(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns []string, whitelist ...string) error {
	if o == nil {
		return errors.New("models: no reputation_users provided for upsert")
	}
	currTime := time.Now().In(boil.GetLocation())

	if o.CreatedAt.IsZero() {
		o.CreatedAt = currTime
	}

	nzDefaults := queries.NonZeroDefaultSet(reputationUserColumnsWithDefault, o)

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

	reputationUserUpsertCacheMut.RLock()
	cache, cached := reputationUserUpsertCache[key]
	reputationUserUpsertCacheMut.RUnlock()

	var err error

	if !cached {
		var ret []string
		whitelist, ret = strmangle.InsertColumnSet(
			reputationUserColumns,
			reputationUserColumnsWithDefault,
			reputationUserColumnsWithoutDefault,
			nzDefaults,
			whitelist,
		)
		update := strmangle.UpdateColumnSet(
			reputationUserColumns,
			reputationUserPrimaryKeyColumns,
			updateColumns,
		)
		if len(update) == 0 {
			return errors.New("models: unable to upsert reputation_users, could not build update column list")
		}

		conflict := conflictColumns
		if len(conflict) == 0 {
			conflict = make([]string, len(reputationUserPrimaryKeyColumns))
			copy(conflict, reputationUserPrimaryKeyColumns)
		}
		cache.query = queries.BuildUpsertQueryPostgres(dialect, "\"reputation_users\"", updateOnConflict, ret, update, conflict, whitelist)

		cache.valueMapping, err = queries.BindMapping(reputationUserType, reputationUserMapping, whitelist)
		if err != nil {
			return err
		}
		if len(ret) != 0 {
			cache.retMapping, err = queries.BindMapping(reputationUserType, reputationUserMapping, ret)
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
		return errors.Wrap(err, "models: unable to upsert reputation_users")
	}

	if !cached {
		reputationUserUpsertCacheMut.Lock()
		reputationUserUpsertCache[key] = cache
		reputationUserUpsertCacheMut.Unlock()
	}

	return nil
}

// DeleteP deletes a single ReputationUser record with an executor.
// DeleteP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *ReputationUser) DeleteP(exec boil.Executor) {
	if err := o.Delete(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteG deletes a single ReputationUser record.
// DeleteG will match against the primary key column to find the record to delete.
func (o *ReputationUser) DeleteG() error {
	if o == nil {
		return errors.New("models: no ReputationUser provided for deletion")
	}

	return o.Delete(boil.GetDB())
}

// DeleteGP deletes a single ReputationUser record.
// DeleteGP will match against the primary key column to find the record to delete.
// Panics on error.
func (o *ReputationUser) DeleteGP() {
	if err := o.DeleteG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// Delete deletes a single ReputationUser record with an executor.
// Delete will match against the primary key column to find the record to delete.
func (o *ReputationUser) Delete(exec boil.Executor) error {
	if o == nil {
		return errors.New("models: no ReputationUser provided for delete")
	}

	args := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(o)), reputationUserPrimaryKeyMapping)
	sql := "DELETE FROM \"reputation_users\" WHERE \"user_id\"=$1 AND \"guild_id\"=$2"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "models: unable to delete from reputation_users")
	}

	return nil
}

// DeleteAllP deletes all rows, and panics on error.
func (q reputationUserQuery) DeleteAllP() {
	if err := q.DeleteAll(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all matching rows.
func (q reputationUserQuery) DeleteAll() error {
	if q.Query == nil {
		return errors.New("models: no reputationUserQuery provided for delete all")
	}

	queries.SetDelete(q.Query)

	_, err := q.Query.Exec()
	if err != nil {
		return errors.Wrap(err, "models: unable to delete all from reputation_users")
	}

	return nil
}

// DeleteAllGP deletes all rows in the slice, and panics on error.
func (o ReputationUserSlice) DeleteAllGP() {
	if err := o.DeleteAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAllG deletes all rows in the slice.
func (o ReputationUserSlice) DeleteAllG() error {
	if o == nil {
		return errors.New("models: no ReputationUser slice provided for delete all")
	}
	return o.DeleteAll(boil.GetDB())
}

// DeleteAllP deletes all rows in the slice, using an executor, and panics on error.
func (o ReputationUserSlice) DeleteAllP(exec boil.Executor) {
	if err := o.DeleteAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// DeleteAll deletes all rows in the slice, using an executor.
func (o ReputationUserSlice) DeleteAll(exec boil.Executor) error {
	if o == nil {
		return errors.New("models: no ReputationUser slice provided for delete all")
	}

	if len(o) == 0 {
		return nil
	}

	var args []interface{}
	for _, obj := range o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), reputationUserPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"DELETE FROM \"reputation_users\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, reputationUserPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(o)*len(reputationUserPrimaryKeyColumns), 1, len(reputationUserPrimaryKeyColumns)),
	)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args)
	}

	_, err := exec.Exec(sql, args...)
	if err != nil {
		return errors.Wrap(err, "models: unable to delete all from reputationUser slice")
	}

	return nil
}

// ReloadGP refetches the object from the database and panics on error.
func (o *ReputationUser) ReloadGP() {
	if err := o.ReloadG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadP refetches the object from the database with an executor. Panics on error.
func (o *ReputationUser) ReloadP(exec boil.Executor) {
	if err := o.Reload(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadG refetches the object from the database using the primary keys.
func (o *ReputationUser) ReloadG() error {
	if o == nil {
		return errors.New("models: no ReputationUser provided for reload")
	}

	return o.Reload(boil.GetDB())
}

// Reload refetches the object from the database
// using the primary keys with an executor.
func (o *ReputationUser) Reload(exec boil.Executor) error {
	ret, err := FindReputationUser(exec, o.UserID, o.GuildID)
	if err != nil {
		return err
	}

	*o = *ret
	return nil
}

// ReloadAllGP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *ReputationUserSlice) ReloadAllGP() {
	if err := o.ReloadAllG(); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllP refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
// Panics on error.
func (o *ReputationUserSlice) ReloadAllP(exec boil.Executor) {
	if err := o.ReloadAll(exec); err != nil {
		panic(boil.WrapErr(err))
	}
}

// ReloadAllG refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *ReputationUserSlice) ReloadAllG() error {
	if o == nil {
		return errors.New("models: empty ReputationUserSlice provided for reload all")
	}

	return o.ReloadAll(boil.GetDB())
}

// ReloadAll refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *ReputationUserSlice) ReloadAll(exec boil.Executor) error {
	if o == nil || len(*o) == 0 {
		return nil
	}

	reputationUsers := ReputationUserSlice{}
	var args []interface{}
	for _, obj := range *o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), reputationUserPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf(
		"SELECT \"reputation_users\".* FROM \"reputation_users\" WHERE (%s) IN (%s)",
		strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, reputationUserPrimaryKeyColumns), ","),
		strmangle.Placeholders(dialect.IndexPlaceholders, len(*o)*len(reputationUserPrimaryKeyColumns), 1, len(reputationUserPrimaryKeyColumns)),
	)

	q := queries.Raw(exec, sql, args...)

	err := q.Bind(&reputationUsers)
	if err != nil {
		return errors.Wrap(err, "models: unable to reload all in ReputationUserSlice")
	}

	*o = reputationUsers

	return nil
}

// ReputationUserExists checks if the ReputationUser row exists.
func ReputationUserExists(exec boil.Executor, userID int64, guildID int64) (bool, error) {
	var exists bool

	sql := "select exists(select 1 from \"reputation_users\" where \"user_id\"=$1 AND \"guild_id\"=$2 limit 1)"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, userID, guildID)
	}

	row := exec.QueryRow(sql, userID, guildID)

	err := row.Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, "models: unable to check if reputation_users exists")
	}

	return exists, nil
}

// ReputationUserExistsG checks if the ReputationUser row exists.
func ReputationUserExistsG(userID int64, guildID int64) (bool, error) {
	return ReputationUserExists(boil.GetDB(), userID, guildID)
}

// ReputationUserExistsGP checks if the ReputationUser row exists. Panics on error.
func ReputationUserExistsGP(userID int64, guildID int64) bool {
	e, err := ReputationUserExists(boil.GetDB(), userID, guildID)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}

// ReputationUserExistsP checks if the ReputationUser row exists. Panics on error.
func ReputationUserExistsP(exec boil.Executor, userID int64, guildID int64) bool {
	e, err := ReputationUserExists(exec, userID, guildID)
	if err != nil {
		panic(boil.WrapErr(err))
	}

	return e
}
