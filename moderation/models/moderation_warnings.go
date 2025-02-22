// Code generated by SQLBoiler 4.16.2 (https://github.com/volatiletech/sqlboiler). DO NOT EDIT.
// This file is meant to be re-generated in place and/or deleted at any time.

package models

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/friendsofgo/errors"
	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
	"github.com/volatiletech/sqlboiler/v4/queries/qmhelper"
	"github.com/volatiletech/strmangle"
)

// ModerationWarning is an object representing the database table.
type ModerationWarning struct {
	ID                    int         `boil:"id" json:"id" toml:"id" yaml:"id"`
	CreatedAt             time.Time   `boil:"created_at" json:"created_at" toml:"created_at" yaml:"created_at"`
	UpdatedAt             time.Time   `boil:"updated_at" json:"updated_at" toml:"updated_at" yaml:"updated_at"`
	GuildID               int64       `boil:"guild_id" json:"guild_id" toml:"guild_id" yaml:"guild_id"`
	UserID                string      `boil:"user_id" json:"user_id" toml:"user_id" yaml:"user_id"`
	AuthorID              string      `boil:"author_id" json:"author_id" toml:"author_id" yaml:"author_id"`
	AuthorUsernameDiscrim string      `boil:"author_username_discrim" json:"author_username_discrim" toml:"author_username_discrim" yaml:"author_username_discrim"`
	Message               string      `boil:"message" json:"message" toml:"message" yaml:"message"`
	LogsLink              null.String `boil:"logs_link" json:"logs_link,omitempty" toml:"logs_link" yaml:"logs_link,omitempty"`

	R *moderationWarningR `boil:"-" json:"-" toml:"-" yaml:"-"`
	L moderationWarningL  `boil:"-" json:"-" toml:"-" yaml:"-"`
}

var ModerationWarningColumns = struct {
	ID                    string
	CreatedAt             string
	UpdatedAt             string
	GuildID               string
	UserID                string
	AuthorID              string
	AuthorUsernameDiscrim string
	Message               string
	LogsLink              string
}{
	ID:                    "id",
	CreatedAt:             "created_at",
	UpdatedAt:             "updated_at",
	GuildID:               "guild_id",
	UserID:                "user_id",
	AuthorID:              "author_id",
	AuthorUsernameDiscrim: "author_username_discrim",
	Message:               "message",
	LogsLink:              "logs_link",
}

var ModerationWarningTableColumns = struct {
	ID                    string
	CreatedAt             string
	UpdatedAt             string
	GuildID               string
	UserID                string
	AuthorID              string
	AuthorUsernameDiscrim string
	Message               string
	LogsLink              string
}{
	ID:                    "moderation_warnings.id",
	CreatedAt:             "moderation_warnings.created_at",
	UpdatedAt:             "moderation_warnings.updated_at",
	GuildID:               "moderation_warnings.guild_id",
	UserID:                "moderation_warnings.user_id",
	AuthorID:              "moderation_warnings.author_id",
	AuthorUsernameDiscrim: "moderation_warnings.author_username_discrim",
	Message:               "moderation_warnings.message",
	LogsLink:              "moderation_warnings.logs_link",
}

// Generated where

type whereHelperint struct{ field string }

func (w whereHelperint) EQ(x int) qm.QueryMod  { return qmhelper.Where(w.field, qmhelper.EQ, x) }
func (w whereHelperint) NEQ(x int) qm.QueryMod { return qmhelper.Where(w.field, qmhelper.NEQ, x) }
func (w whereHelperint) LT(x int) qm.QueryMod  { return qmhelper.Where(w.field, qmhelper.LT, x) }
func (w whereHelperint) LTE(x int) qm.QueryMod { return qmhelper.Where(w.field, qmhelper.LTE, x) }
func (w whereHelperint) GT(x int) qm.QueryMod  { return qmhelper.Where(w.field, qmhelper.GT, x) }
func (w whereHelperint) GTE(x int) qm.QueryMod { return qmhelper.Where(w.field, qmhelper.GTE, x) }
func (w whereHelperint) IN(slice []int) qm.QueryMod {
	values := make([]interface{}, 0, len(slice))
	for _, value := range slice {
		values = append(values, value)
	}
	return qm.WhereIn(fmt.Sprintf("%s IN ?", w.field), values...)
}
func (w whereHelperint) NIN(slice []int) qm.QueryMod {
	values := make([]interface{}, 0, len(slice))
	for _, value := range slice {
		values = append(values, value)
	}
	return qm.WhereNotIn(fmt.Sprintf("%s NOT IN ?", w.field), values...)
}

type whereHelperstring struct{ field string }

func (w whereHelperstring) EQ(x string) qm.QueryMod    { return qmhelper.Where(w.field, qmhelper.EQ, x) }
func (w whereHelperstring) NEQ(x string) qm.QueryMod   { return qmhelper.Where(w.field, qmhelper.NEQ, x) }
func (w whereHelperstring) LT(x string) qm.QueryMod    { return qmhelper.Where(w.field, qmhelper.LT, x) }
func (w whereHelperstring) LTE(x string) qm.QueryMod   { return qmhelper.Where(w.field, qmhelper.LTE, x) }
func (w whereHelperstring) GT(x string) qm.QueryMod    { return qmhelper.Where(w.field, qmhelper.GT, x) }
func (w whereHelperstring) GTE(x string) qm.QueryMod   { return qmhelper.Where(w.field, qmhelper.GTE, x) }
func (w whereHelperstring) LIKE(x string) qm.QueryMod  { return qm.Where(w.field+" LIKE ?", x) }
func (w whereHelperstring) NLIKE(x string) qm.QueryMod { return qm.Where(w.field+" NOT LIKE ?", x) }
func (w whereHelperstring) IN(slice []string) qm.QueryMod {
	values := make([]interface{}, 0, len(slice))
	for _, value := range slice {
		values = append(values, value)
	}
	return qm.WhereIn(fmt.Sprintf("%s IN ?", w.field), values...)
}
func (w whereHelperstring) NIN(slice []string) qm.QueryMod {
	values := make([]interface{}, 0, len(slice))
	for _, value := range slice {
		values = append(values, value)
	}
	return qm.WhereNotIn(fmt.Sprintf("%s NOT IN ?", w.field), values...)
}

var ModerationWarningWhere = struct {
	ID                    whereHelperint
	CreatedAt             whereHelpertime_Time
	UpdatedAt             whereHelpertime_Time
	GuildID               whereHelperint64
	UserID                whereHelperstring
	AuthorID              whereHelperstring
	AuthorUsernameDiscrim whereHelperstring
	Message               whereHelperstring
	LogsLink              whereHelpernull_String
}{
	ID:                    whereHelperint{field: "\"moderation_warnings\".\"id\""},
	CreatedAt:             whereHelpertime_Time{field: "\"moderation_warnings\".\"created_at\""},
	UpdatedAt:             whereHelpertime_Time{field: "\"moderation_warnings\".\"updated_at\""},
	GuildID:               whereHelperint64{field: "\"moderation_warnings\".\"guild_id\""},
	UserID:                whereHelperstring{field: "\"moderation_warnings\".\"user_id\""},
	AuthorID:              whereHelperstring{field: "\"moderation_warnings\".\"author_id\""},
	AuthorUsernameDiscrim: whereHelperstring{field: "\"moderation_warnings\".\"author_username_discrim\""},
	Message:               whereHelperstring{field: "\"moderation_warnings\".\"message\""},
	LogsLink:              whereHelpernull_String{field: "\"moderation_warnings\".\"logs_link\""},
}

// ModerationWarningRels is where relationship names are stored.
var ModerationWarningRels = struct {
}{}

// moderationWarningR is where relationships are stored.
type moderationWarningR struct {
}

// NewStruct creates a new relationship struct
func (*moderationWarningR) NewStruct() *moderationWarningR {
	return &moderationWarningR{}
}

// moderationWarningL is where Load methods for each relationship are stored.
type moderationWarningL struct{}

var (
	moderationWarningAllColumns            = []string{"id", "created_at", "updated_at", "guild_id", "user_id", "author_id", "author_username_discrim", "message", "logs_link"}
	moderationWarningColumnsWithoutDefault = []string{"created_at", "updated_at", "guild_id", "user_id", "author_id", "author_username_discrim", "message"}
	moderationWarningColumnsWithDefault    = []string{"id", "logs_link"}
	moderationWarningPrimaryKeyColumns     = []string{"id"}
	moderationWarningGeneratedColumns      = []string{}
)

type (
	// ModerationWarningSlice is an alias for a slice of pointers to ModerationWarning.
	// This should almost always be used instead of []ModerationWarning.
	ModerationWarningSlice []*ModerationWarning

	moderationWarningQuery struct {
		*queries.Query
	}
)

// Cache for insert, update and upsert
var (
	moderationWarningType                 = reflect.TypeOf(&ModerationWarning{})
	moderationWarningMapping              = queries.MakeStructMapping(moderationWarningType)
	moderationWarningPrimaryKeyMapping, _ = queries.BindMapping(moderationWarningType, moderationWarningMapping, moderationWarningPrimaryKeyColumns)
	moderationWarningInsertCacheMut       sync.RWMutex
	moderationWarningInsertCache          = make(map[string]insertCache)
	moderationWarningUpdateCacheMut       sync.RWMutex
	moderationWarningUpdateCache          = make(map[string]updateCache)
	moderationWarningUpsertCacheMut       sync.RWMutex
	moderationWarningUpsertCache          = make(map[string]insertCache)
)

var (
	// Force time package dependency for automated UpdatedAt/CreatedAt.
	_ = time.Second
	// Force qmhelper dependency for where clause generation (which doesn't
	// always happen)
	_ = qmhelper.Where
)

// OneG returns a single moderationWarning record from the query using the global executor.
func (q moderationWarningQuery) OneG(ctx context.Context) (*ModerationWarning, error) {
	return q.One(ctx, boil.GetContextDB())
}

// One returns a single moderationWarning record from the query.
func (q moderationWarningQuery) One(ctx context.Context, exec boil.ContextExecutor) (*ModerationWarning, error) {
	o := &ModerationWarning{}

	queries.SetLimit(q.Query, 1)

	err := q.Bind(ctx, exec, o)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "models: failed to execute a one query for moderation_warnings")
	}

	return o, nil
}

// AllG returns all ModerationWarning records from the query using the global executor.
func (q moderationWarningQuery) AllG(ctx context.Context) (ModerationWarningSlice, error) {
	return q.All(ctx, boil.GetContextDB())
}

// All returns all ModerationWarning records from the query.
func (q moderationWarningQuery) All(ctx context.Context, exec boil.ContextExecutor) (ModerationWarningSlice, error) {
	var o []*ModerationWarning

	err := q.Bind(ctx, exec, &o)
	if err != nil {
		return nil, errors.Wrap(err, "models: failed to assign all query results to ModerationWarning slice")
	}

	return o, nil
}

// CountG returns the count of all ModerationWarning records in the query using the global executor
func (q moderationWarningQuery) CountG(ctx context.Context) (int64, error) {
	return q.Count(ctx, boil.GetContextDB())
}

// Count returns the count of all ModerationWarning records in the query.
func (q moderationWarningQuery) Count(ctx context.Context, exec boil.ContextExecutor) (int64, error) {
	var count int64

	queries.SetSelect(q.Query, nil)
	queries.SetCount(q.Query)

	err := q.Query.QueryRowContext(ctx, exec).Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "models: failed to count moderation_warnings rows")
	}

	return count, nil
}

// ExistsG checks if the row exists in the table using the global executor.
func (q moderationWarningQuery) ExistsG(ctx context.Context) (bool, error) {
	return q.Exists(ctx, boil.GetContextDB())
}

// Exists checks if the row exists in the table.
func (q moderationWarningQuery) Exists(ctx context.Context, exec boil.ContextExecutor) (bool, error) {
	var count int64

	queries.SetSelect(q.Query, nil)
	queries.SetCount(q.Query)
	queries.SetLimit(q.Query, 1)

	err := q.Query.QueryRowContext(ctx, exec).Scan(&count)
	if err != nil {
		return false, errors.Wrap(err, "models: failed to check if moderation_warnings exists")
	}

	return count > 0, nil
}

// ModerationWarnings retrieves all the records using an executor.
func ModerationWarnings(mods ...qm.QueryMod) moderationWarningQuery {
	mods = append(mods, qm.From("\"moderation_warnings\""))
	q := NewQuery(mods...)
	if len(queries.GetSelect(q)) == 0 {
		queries.SetSelect(q, []string{"\"moderation_warnings\".*"})
	}

	return moderationWarningQuery{q}
}

// FindModerationWarningG retrieves a single record by ID.
func FindModerationWarningG(ctx context.Context, iD int, selectCols ...string) (*ModerationWarning, error) {
	return FindModerationWarning(ctx, boil.GetContextDB(), iD, selectCols...)
}

// FindModerationWarning retrieves a single record by ID with an executor.
// If selectCols is empty Find will return all columns.
func FindModerationWarning(ctx context.Context, exec boil.ContextExecutor, iD int, selectCols ...string) (*ModerationWarning, error) {
	moderationWarningObj := &ModerationWarning{}

	sel := "*"
	if len(selectCols) > 0 {
		sel = strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, selectCols), ",")
	}
	query := fmt.Sprintf(
		"select %s from \"moderation_warnings\" where \"id\"=$1", sel,
	)

	q := queries.Raw(query, iD)

	err := q.Bind(ctx, exec, moderationWarningObj)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "models: unable to select from moderation_warnings")
	}

	return moderationWarningObj, nil
}

// InsertG a single record. See Insert for whitelist behavior description.
func (o *ModerationWarning) InsertG(ctx context.Context, columns boil.Columns) error {
	return o.Insert(ctx, boil.GetContextDB(), columns)
}

// Insert a single record using an executor.
// See boil.Columns.InsertColumnSet documentation to understand column list inference for inserts.
func (o *ModerationWarning) Insert(ctx context.Context, exec boil.ContextExecutor, columns boil.Columns) error {
	if o == nil {
		return errors.New("models: no moderation_warnings provided for insertion")
	}

	var err error
	if !boil.TimestampsAreSkipped(ctx) {
		currTime := time.Now().In(boil.GetLocation())

		if o.CreatedAt.IsZero() {
			o.CreatedAt = currTime
		}
		if o.UpdatedAt.IsZero() {
			o.UpdatedAt = currTime
		}
	}

	nzDefaults := queries.NonZeroDefaultSet(moderationWarningColumnsWithDefault, o)

	key := makeCacheKey(columns, nzDefaults)
	moderationWarningInsertCacheMut.RLock()
	cache, cached := moderationWarningInsertCache[key]
	moderationWarningInsertCacheMut.RUnlock()

	if !cached {
		wl, returnColumns := columns.InsertColumnSet(
			moderationWarningAllColumns,
			moderationWarningColumnsWithDefault,
			moderationWarningColumnsWithoutDefault,
			nzDefaults,
		)

		cache.valueMapping, err = queries.BindMapping(moderationWarningType, moderationWarningMapping, wl)
		if err != nil {
			return err
		}
		cache.retMapping, err = queries.BindMapping(moderationWarningType, moderationWarningMapping, returnColumns)
		if err != nil {
			return err
		}
		if len(wl) != 0 {
			cache.query = fmt.Sprintf("INSERT INTO \"moderation_warnings\" (\"%s\") %%sVALUES (%s)%%s", strings.Join(wl, "\",\""), strmangle.Placeholders(dialect.UseIndexPlaceholders, len(wl), 1, 1))
		} else {
			cache.query = "INSERT INTO \"moderation_warnings\" %sDEFAULT VALUES%s"
		}

		var queryOutput, queryReturning string

		if len(cache.retMapping) != 0 {
			queryReturning = fmt.Sprintf(" RETURNING \"%s\"", strings.Join(returnColumns, "\",\""))
		}

		cache.query = fmt.Sprintf(cache.query, queryOutput, queryReturning)
	}

	value := reflect.Indirect(reflect.ValueOf(o))
	vals := queries.ValuesFromMapping(value, cache.valueMapping)

	if boil.IsDebug(ctx) {
		writer := boil.DebugWriterFrom(ctx)
		fmt.Fprintln(writer, cache.query)
		fmt.Fprintln(writer, vals)
	}

	if len(cache.retMapping) != 0 {
		err = exec.QueryRowContext(ctx, cache.query, vals...).Scan(queries.PtrsFromMapping(value, cache.retMapping)...)
	} else {
		_, err = exec.ExecContext(ctx, cache.query, vals...)
	}

	if err != nil {
		return errors.Wrap(err, "models: unable to insert into moderation_warnings")
	}

	if !cached {
		moderationWarningInsertCacheMut.Lock()
		moderationWarningInsertCache[key] = cache
		moderationWarningInsertCacheMut.Unlock()
	}

	return nil
}

// UpdateG a single ModerationWarning record using the global executor.
// See Update for more documentation.
func (o *ModerationWarning) UpdateG(ctx context.Context, columns boil.Columns) (int64, error) {
	return o.Update(ctx, boil.GetContextDB(), columns)
}

// Update uses an executor to update the ModerationWarning.
// See boil.Columns.UpdateColumnSet documentation to understand column list inference for updates.
// Update does not automatically update the record in case of default values. Use .Reload() to refresh the records.
func (o *ModerationWarning) Update(ctx context.Context, exec boil.ContextExecutor, columns boil.Columns) (int64, error) {
	if !boil.TimestampsAreSkipped(ctx) {
		currTime := time.Now().In(boil.GetLocation())

		o.UpdatedAt = currTime
	}

	var err error
	key := makeCacheKey(columns, nil)
	moderationWarningUpdateCacheMut.RLock()
	cache, cached := moderationWarningUpdateCache[key]
	moderationWarningUpdateCacheMut.RUnlock()

	if !cached {
		wl := columns.UpdateColumnSet(
			moderationWarningAllColumns,
			moderationWarningPrimaryKeyColumns,
		)

		if !columns.IsWhitelist() {
			wl = strmangle.SetComplement(wl, []string{"created_at"})
		}
		if len(wl) == 0 {
			return 0, errors.New("models: unable to update moderation_warnings, could not build whitelist")
		}

		cache.query = fmt.Sprintf("UPDATE \"moderation_warnings\" SET %s WHERE %s",
			strmangle.SetParamNames("\"", "\"", 1, wl),
			strmangle.WhereClause("\"", "\"", len(wl)+1, moderationWarningPrimaryKeyColumns),
		)
		cache.valueMapping, err = queries.BindMapping(moderationWarningType, moderationWarningMapping, append(wl, moderationWarningPrimaryKeyColumns...))
		if err != nil {
			return 0, err
		}
	}

	values := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(o)), cache.valueMapping)

	if boil.IsDebug(ctx) {
		writer := boil.DebugWriterFrom(ctx)
		fmt.Fprintln(writer, cache.query)
		fmt.Fprintln(writer, values)
	}
	var result sql.Result
	result, err = exec.ExecContext(ctx, cache.query, values...)
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to update moderation_warnings row")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "models: failed to get rows affected by update for moderation_warnings")
	}

	if !cached {
		moderationWarningUpdateCacheMut.Lock()
		moderationWarningUpdateCache[key] = cache
		moderationWarningUpdateCacheMut.Unlock()
	}

	return rowsAff, nil
}

// UpdateAllG updates all rows with the specified column values.
func (q moderationWarningQuery) UpdateAllG(ctx context.Context, cols M) (int64, error) {
	return q.UpdateAll(ctx, boil.GetContextDB(), cols)
}

// UpdateAll updates all rows with the specified column values.
func (q moderationWarningQuery) UpdateAll(ctx context.Context, exec boil.ContextExecutor, cols M) (int64, error) {
	queries.SetUpdate(q.Query, cols)

	result, err := q.Query.ExecContext(ctx, exec)
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to update all for moderation_warnings")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to retrieve rows affected for moderation_warnings")
	}

	return rowsAff, nil
}

// UpdateAllG updates all rows with the specified column values.
func (o ModerationWarningSlice) UpdateAllG(ctx context.Context, cols M) (int64, error) {
	return o.UpdateAll(ctx, boil.GetContextDB(), cols)
}

// UpdateAll updates all rows with the specified column values, using an executor.
func (o ModerationWarningSlice) UpdateAll(ctx context.Context, exec boil.ContextExecutor, cols M) (int64, error) {
	ln := int64(len(o))
	if ln == 0 {
		return 0, nil
	}

	if len(cols) == 0 {
		return 0, errors.New("models: update all requires at least one column argument")
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
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), moderationWarningPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf("UPDATE \"moderation_warnings\" SET %s WHERE %s",
		strmangle.SetParamNames("\"", "\"", 1, colNames),
		strmangle.WhereClauseRepeated(string(dialect.LQ), string(dialect.RQ), len(colNames)+1, moderationWarningPrimaryKeyColumns, len(o)))

	if boil.IsDebug(ctx) {
		writer := boil.DebugWriterFrom(ctx)
		fmt.Fprintln(writer, sql)
		fmt.Fprintln(writer, args...)
	}
	result, err := exec.ExecContext(ctx, sql, args...)
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to update all in moderationWarning slice")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to retrieve rows affected all in update all moderationWarning")
	}
	return rowsAff, nil
}

// UpsertG attempts an insert, and does an update or ignore on conflict.
func (o *ModerationWarning) UpsertG(ctx context.Context, updateOnConflict bool, conflictColumns []string, updateColumns, insertColumns boil.Columns) error {
	return o.Upsert(ctx, boil.GetContextDB(), updateOnConflict, conflictColumns, updateColumns, insertColumns)
}

// Upsert attempts an insert using an executor, and does an update or ignore on conflict.
// See boil.Columns documentation for how to properly use updateColumns and insertColumns.
func (o *ModerationWarning) Upsert(ctx context.Context, exec boil.ContextExecutor, updateOnConflict bool, conflictColumns []string, updateColumns, insertColumns boil.Columns) error {
	if o == nil {
		return errors.New("models: no moderation_warnings provided for upsert")
	}
	if !boil.TimestampsAreSkipped(ctx) {
		currTime := time.Now().In(boil.GetLocation())

		if o.CreatedAt.IsZero() {
			o.CreatedAt = currTime
		}
		o.UpdatedAt = currTime
	}

	nzDefaults := queries.NonZeroDefaultSet(moderationWarningColumnsWithDefault, o)

	// Build cache key in-line uglily - mysql vs psql problems
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
	buf.WriteString(strconv.Itoa(updateColumns.Kind))
	for _, c := range updateColumns.Cols {
		buf.WriteString(c)
	}
	buf.WriteByte('.')
	buf.WriteString(strconv.Itoa(insertColumns.Kind))
	for _, c := range insertColumns.Cols {
		buf.WriteString(c)
	}
	buf.WriteByte('.')
	for _, c := range nzDefaults {
		buf.WriteString(c)
	}
	key := buf.String()
	strmangle.PutBuffer(buf)

	moderationWarningUpsertCacheMut.RLock()
	cache, cached := moderationWarningUpsertCache[key]
	moderationWarningUpsertCacheMut.RUnlock()

	var err error

	if !cached {
		insert, ret := insertColumns.InsertColumnSet(
			moderationWarningAllColumns,
			moderationWarningColumnsWithDefault,
			moderationWarningColumnsWithoutDefault,
			nzDefaults,
		)

		update := updateColumns.UpdateColumnSet(
			moderationWarningAllColumns,
			moderationWarningPrimaryKeyColumns,
		)

		if updateOnConflict && len(update) == 0 {
			return errors.New("models: unable to upsert moderation_warnings, could not build update column list")
		}

		conflict := conflictColumns
		if len(conflict) == 0 {
			conflict = make([]string, len(moderationWarningPrimaryKeyColumns))
			copy(conflict, moderationWarningPrimaryKeyColumns)
		}
		cache.query = buildUpsertQueryPostgres(dialect, "\"moderation_warnings\"", updateOnConflict, ret, update, conflict, insert)

		cache.valueMapping, err = queries.BindMapping(moderationWarningType, moderationWarningMapping, insert)
		if err != nil {
			return err
		}
		if len(ret) != 0 {
			cache.retMapping, err = queries.BindMapping(moderationWarningType, moderationWarningMapping, ret)
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

	if boil.IsDebug(ctx) {
		writer := boil.DebugWriterFrom(ctx)
		fmt.Fprintln(writer, cache.query)
		fmt.Fprintln(writer, vals)
	}
	if len(cache.retMapping) != 0 {
		err = exec.QueryRowContext(ctx, cache.query, vals...).Scan(returns...)
		if errors.Is(err, sql.ErrNoRows) {
			err = nil // Postgres doesn't return anything when there's no update
		}
	} else {
		_, err = exec.ExecContext(ctx, cache.query, vals...)
	}
	if err != nil {
		return errors.Wrap(err, "models: unable to upsert moderation_warnings")
	}

	if !cached {
		moderationWarningUpsertCacheMut.Lock()
		moderationWarningUpsertCache[key] = cache
		moderationWarningUpsertCacheMut.Unlock()
	}

	return nil
}

// DeleteG deletes a single ModerationWarning record.
// DeleteG will match against the primary key column to find the record to delete.
func (o *ModerationWarning) DeleteG(ctx context.Context) (int64, error) {
	return o.Delete(ctx, boil.GetContextDB())
}

// Delete deletes a single ModerationWarning record with an executor.
// Delete will match against the primary key column to find the record to delete.
func (o *ModerationWarning) Delete(ctx context.Context, exec boil.ContextExecutor) (int64, error) {
	if o == nil {
		return 0, errors.New("models: no ModerationWarning provided for delete")
	}

	args := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(o)), moderationWarningPrimaryKeyMapping)
	sql := "DELETE FROM \"moderation_warnings\" WHERE \"id\"=$1"

	if boil.IsDebug(ctx) {
		writer := boil.DebugWriterFrom(ctx)
		fmt.Fprintln(writer, sql)
		fmt.Fprintln(writer, args...)
	}
	result, err := exec.ExecContext(ctx, sql, args...)
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to delete from moderation_warnings")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "models: failed to get rows affected by delete for moderation_warnings")
	}

	return rowsAff, nil
}

func (q moderationWarningQuery) DeleteAllG(ctx context.Context) (int64, error) {
	return q.DeleteAll(ctx, boil.GetContextDB())
}

// DeleteAll deletes all matching rows.
func (q moderationWarningQuery) DeleteAll(ctx context.Context, exec boil.ContextExecutor) (int64, error) {
	if q.Query == nil {
		return 0, errors.New("models: no moderationWarningQuery provided for delete all")
	}

	queries.SetDelete(q.Query)

	result, err := q.Query.ExecContext(ctx, exec)
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to delete all from moderation_warnings")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "models: failed to get rows affected by deleteall for moderation_warnings")
	}

	return rowsAff, nil
}

// DeleteAllG deletes all rows in the slice.
func (o ModerationWarningSlice) DeleteAllG(ctx context.Context) (int64, error) {
	return o.DeleteAll(ctx, boil.GetContextDB())
}

// DeleteAll deletes all rows in the slice, using an executor.
func (o ModerationWarningSlice) DeleteAll(ctx context.Context, exec boil.ContextExecutor) (int64, error) {
	if len(o) == 0 {
		return 0, nil
	}

	var args []interface{}
	for _, obj := range o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), moderationWarningPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := "DELETE FROM \"moderation_warnings\" WHERE " +
		strmangle.WhereClauseRepeated(string(dialect.LQ), string(dialect.RQ), 1, moderationWarningPrimaryKeyColumns, len(o))

	if boil.IsDebug(ctx) {
		writer := boil.DebugWriterFrom(ctx)
		fmt.Fprintln(writer, sql)
		fmt.Fprintln(writer, args)
	}
	result, err := exec.ExecContext(ctx, sql, args...)
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to delete all from moderationWarning slice")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "models: failed to get rows affected by deleteall for moderation_warnings")
	}

	return rowsAff, nil
}

// ReloadG refetches the object from the database using the primary keys.
func (o *ModerationWarning) ReloadG(ctx context.Context) error {
	if o == nil {
		return errors.New("models: no ModerationWarning provided for reload")
	}

	return o.Reload(ctx, boil.GetContextDB())
}

// Reload refetches the object from the database
// using the primary keys with an executor.
func (o *ModerationWarning) Reload(ctx context.Context, exec boil.ContextExecutor) error {
	ret, err := FindModerationWarning(ctx, exec, o.ID)
	if err != nil {
		return err
	}

	*o = *ret
	return nil
}

// ReloadAllG refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *ModerationWarningSlice) ReloadAllG(ctx context.Context) error {
	if o == nil {
		return errors.New("models: empty ModerationWarningSlice provided for reload all")
	}

	return o.ReloadAll(ctx, boil.GetContextDB())
}

// ReloadAll refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *ModerationWarningSlice) ReloadAll(ctx context.Context, exec boil.ContextExecutor) error {
	if o == nil || len(*o) == 0 {
		return nil
	}

	slice := ModerationWarningSlice{}
	var args []interface{}
	for _, obj := range *o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), moderationWarningPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := "SELECT \"moderation_warnings\".* FROM \"moderation_warnings\" WHERE " +
		strmangle.WhereClauseRepeated(string(dialect.LQ), string(dialect.RQ), 1, moderationWarningPrimaryKeyColumns, len(*o))

	q := queries.Raw(sql, args...)

	err := q.Bind(ctx, exec, &slice)
	if err != nil {
		return errors.Wrap(err, "models: unable to reload all in ModerationWarningSlice")
	}

	*o = slice

	return nil
}

// ModerationWarningExistsG checks if the ModerationWarning row exists.
func ModerationWarningExistsG(ctx context.Context, iD int) (bool, error) {
	return ModerationWarningExists(ctx, boil.GetContextDB(), iD)
}

// ModerationWarningExists checks if the ModerationWarning row exists.
func ModerationWarningExists(ctx context.Context, exec boil.ContextExecutor, iD int) (bool, error) {
	var exists bool
	sql := "select exists(select 1 from \"moderation_warnings\" where \"id\"=$1 limit 1)"

	if boil.IsDebug(ctx) {
		writer := boil.DebugWriterFrom(ctx)
		fmt.Fprintln(writer, sql)
		fmt.Fprintln(writer, iD)
	}
	row := exec.QueryRowContext(ctx, sql, iD)

	err := row.Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, "models: unable to check if moderation_warnings exists")
	}

	return exists, nil
}

// Exists checks if the ModerationWarning row exists.
func (o *ModerationWarning) Exists(ctx context.Context, exec boil.ContextExecutor) (bool, error) {
	return ModerationWarningExists(ctx, exec, o.ID)
}
