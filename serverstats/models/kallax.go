// IMPORTANT! This is auto generated code by https://github.com/src-d/go-kallax
// Please, do not touch the code below, and if you do, do it under your own
// risk. Take into account that all the code you write here will be completely
// erased from earth the next time you generate the kallax models.
package models

import (
	"database/sql"
	"fmt"
	"time"

	"gopkg.in/src-d/go-kallax.v1"
	"gopkg.in/src-d/go-kallax.v1/types"
)

var _ types.SQLType
var _ fmt.Formatter

// NewStatsPeriod returns a new instance of StatsPeriod.
func NewStatsPeriod() (record *StatsPeriod) {
	return new(StatsPeriod)
}

// GetID returns the primary key of the model.
func (r *StatsPeriod) GetID() kallax.Identifier {
	return (*kallax.NumericID)(&r.ID)
}

// ColumnAddress returns the pointer to the value of the given column.
func (r *StatsPeriod) ColumnAddress(col string) (interface{}, error) {
	switch col {
	case "id":
		return (*kallax.NumericID)(&r.ID), nil
	case "started":
		return &r.Started, nil
	case "duration":
		return (*int64)(&r.Duration), nil
	case "guild_id":
		return &r.GuildID, nil
	case "user_id":
		return &r.UserID, nil
	case "channel_id":
		return &r.ChannelID, nil
	case "count":
		return &r.Count, nil

	default:
		return nil, fmt.Errorf("kallax: invalid column in StatsPeriod: %s", col)
	}
}

// Value returns the value of the given column.
func (r *StatsPeriod) Value(col string) (interface{}, error) {
	switch col {
	case "id":
		return r.ID, nil
	case "started":
		return r.Started, nil
	case "duration":
		return (int64)(r.Duration), nil
	case "guild_id":
		return r.GuildID, nil
	case "user_id":
		return r.UserID, nil
	case "channel_id":
		return r.ChannelID, nil
	case "count":
		return r.Count, nil

	default:
		return nil, fmt.Errorf("kallax: invalid column in StatsPeriod: %s", col)
	}
}

// NewRelationshipRecord returns a new record for the relatiobship in the given
// field.
func (r *StatsPeriod) NewRelationshipRecord(field string) (kallax.Record, error) {
	return nil, fmt.Errorf("kallax: model StatsPeriod has no relationships")
}

// SetRelationship sets the given relationship in the given field.
func (r *StatsPeriod) SetRelationship(field string, rel interface{}) error {
	return fmt.Errorf("kallax: model StatsPeriod has no relationships")
}

// StatsPeriodStore is the entity to access the records of the type StatsPeriod
// in the database.
type StatsPeriodStore struct {
	*kallax.Store
}

// NewStatsPeriodStore creates a new instance of StatsPeriodStore
// using a SQL database.
func NewStatsPeriodStore(db *sql.DB) *StatsPeriodStore {
	return &StatsPeriodStore{kallax.NewStore(db)}
}

// GenericStore returns the generic store of this store.
func (s *StatsPeriodStore) GenericStore() *kallax.Store {
	return s.Store
}

// SetGenericStore changes the generic store of this store.
func (s *StatsPeriodStore) SetGenericStore(store *kallax.Store) {
	s.Store = store
}

// Debug returns a new store that will print all SQL statements to stdout using
// the log.Printf function.
func (s *StatsPeriodStore) Debug() *StatsPeriodStore {
	return &StatsPeriodStore{s.Store.Debug()}
}

// DebugWith returns a new store that will print all SQL statements using the
// given logger function.
func (s *StatsPeriodStore) DebugWith(logger kallax.LoggerFunc) *StatsPeriodStore {
	return &StatsPeriodStore{s.Store.DebugWith(logger)}
}

// Insert inserts a StatsPeriod in the database. A non-persisted object is
// required for this operation.
func (s *StatsPeriodStore) Insert(record *StatsPeriod) error {
	record.Started = record.Started.Truncate(time.Microsecond)

	return s.Store.Insert(Schema.StatsPeriod.BaseSchema, record)
}

// Update updates the given record on the database. If the columns are given,
// only these columns will be updated. Otherwise all of them will be.
// Be very careful with this, as you will have a potentially different object
// in memory but not on the database.
// Only writable records can be updated. Writable objects are those that have
// been just inserted or retrieved using a query with no custom select fields.
func (s *StatsPeriodStore) Update(record *StatsPeriod, cols ...kallax.SchemaField) (updated int64, err error) {
	record.Started = record.Started.Truncate(time.Microsecond)

	return s.Store.Update(Schema.StatsPeriod.BaseSchema, record, cols...)
}

// Save inserts the object if the record is not persisted, otherwise it updates
// it. Same rules of Update and Insert apply depending on the case.
func (s *StatsPeriodStore) Save(record *StatsPeriod) (updated bool, err error) {
	if !record.IsPersisted() {
		return false, s.Insert(record)
	}

	rowsUpdated, err := s.Update(record)
	if err != nil {
		return false, err
	}

	return rowsUpdated > 0, nil
}

// Delete removes the given record from the database.
func (s *StatsPeriodStore) Delete(record *StatsPeriod) error {
	return s.Store.Delete(Schema.StatsPeriod.BaseSchema, record)
}

// Find returns the set of results for the given query.
func (s *StatsPeriodStore) Find(q *StatsPeriodQuery) (*StatsPeriodResultSet, error) {
	rs, err := s.Store.Find(q)
	if err != nil {
		return nil, err
	}

	return NewStatsPeriodResultSet(rs), nil
}

// MustFind returns the set of results for the given query, but panics if there
// is any error.
func (s *StatsPeriodStore) MustFind(q *StatsPeriodQuery) *StatsPeriodResultSet {
	return NewStatsPeriodResultSet(s.Store.MustFind(q))
}

// Count returns the number of rows that would be retrieved with the given
// query.
func (s *StatsPeriodStore) Count(q *StatsPeriodQuery) (int64, error) {
	return s.Store.Count(q)
}

// MustCount returns the number of rows that would be retrieved with the given
// query, but panics if there is an error.
func (s *StatsPeriodStore) MustCount(q *StatsPeriodQuery) int64 {
	return s.Store.MustCount(q)
}

// FindOne returns the first row returned by the given query.
// `ErrNotFound` is returned if there are no results.
func (s *StatsPeriodStore) FindOne(q *StatsPeriodQuery) (*StatsPeriod, error) {
	q.Limit(1)
	q.Offset(0)
	rs, err := s.Find(q)
	if err != nil {
		return nil, err
	}

	if !rs.Next() {
		return nil, kallax.ErrNotFound
	}

	record, err := rs.Get()
	if err != nil {
		return nil, err
	}

	if err := rs.Close(); err != nil {
		return nil, err
	}

	return record, nil
}

// FindAll returns a list of all the rows returned by the given query.
func (s *StatsPeriodStore) FindAll(q *StatsPeriodQuery) ([]*StatsPeriod, error) {
	rs, err := s.Find(q)
	if err != nil {
		return nil, err
	}

	return rs.All()
}

// MustFindOne returns the first row retrieved by the given query. It panics
// if there is an error or if there are no rows.
func (s *StatsPeriodStore) MustFindOne(q *StatsPeriodQuery) *StatsPeriod {
	record, err := s.FindOne(q)
	if err != nil {
		panic(err)
	}
	return record
}

// Reload refreshes the StatsPeriod with the data in the database and
// makes it writable.
func (s *StatsPeriodStore) Reload(record *StatsPeriod) error {
	return s.Store.Reload(Schema.StatsPeriod.BaseSchema, record)
}

// Transaction executes the given callback in a transaction and rollbacks if
// an error is returned.
// The transaction is only open in the store passed as a parameter to the
// callback.
func (s *StatsPeriodStore) Transaction(callback func(*StatsPeriodStore) error) error {
	if callback == nil {
		return kallax.ErrInvalidTxCallback
	}

	return s.Store.Transaction(func(store *kallax.Store) error {
		return callback(&StatsPeriodStore{store})
	})
}

// StatsPeriodQuery is the object used to create queries for the StatsPeriod
// entity.
type StatsPeriodQuery struct {
	*kallax.BaseQuery
}

// NewStatsPeriodQuery returns a new instance of StatsPeriodQuery.
func NewStatsPeriodQuery() *StatsPeriodQuery {
	return &StatsPeriodQuery{
		BaseQuery: kallax.NewBaseQuery(Schema.StatsPeriod.BaseSchema),
	}
}

// Select adds columns to select in the query.
func (q *StatsPeriodQuery) Select(columns ...kallax.SchemaField) *StatsPeriodQuery {
	if len(columns) == 0 {
		return q
	}
	q.BaseQuery.Select(columns...)
	return q
}

// SelectNot excludes columns from being selected in the query.
func (q *StatsPeriodQuery) SelectNot(columns ...kallax.SchemaField) *StatsPeriodQuery {
	q.BaseQuery.SelectNot(columns...)
	return q
}

// Copy returns a new identical copy of the query. Remember queries are mutable
// so make a copy any time you need to reuse them.
func (q *StatsPeriodQuery) Copy() *StatsPeriodQuery {
	return &StatsPeriodQuery{
		BaseQuery: q.BaseQuery.Copy(),
	}
}

// Order adds order clauses to the query for the given columns.
func (q *StatsPeriodQuery) Order(cols ...kallax.ColumnOrder) *StatsPeriodQuery {
	q.BaseQuery.Order(cols...)
	return q
}

// BatchSize sets the number of items to fetch per batch when there are 1:N
// relationships selected in the query.
func (q *StatsPeriodQuery) BatchSize(size uint64) *StatsPeriodQuery {
	q.BaseQuery.BatchSize(size)
	return q
}

// Limit sets the max number of items to retrieve.
func (q *StatsPeriodQuery) Limit(n uint64) *StatsPeriodQuery {
	q.BaseQuery.Limit(n)
	return q
}

// Offset sets the number of items to skip from the result set of items.
func (q *StatsPeriodQuery) Offset(n uint64) *StatsPeriodQuery {
	q.BaseQuery.Offset(n)
	return q
}

// Where adds a condition to the query. All conditions added are concatenated
// using a logical AND.
func (q *StatsPeriodQuery) Where(cond kallax.Condition) *StatsPeriodQuery {
	q.BaseQuery.Where(cond)
	return q
}

// FindByID adds a new filter to the query that will require that
// the ID property is equal to one of the passed values; if no passed values,
// it will do nothing.
func (q *StatsPeriodQuery) FindByID(v ...int64) *StatsPeriodQuery {
	if len(v) == 0 {
		return q
	}
	values := make([]interface{}, len(v))
	for i, val := range v {
		values[i] = val
	}
	return q.Where(kallax.In(Schema.StatsPeriod.ID, values...))
}

// FindByStarted adds a new filter to the query that will require that
// the Started property is equal to the passed value.
func (q *StatsPeriodQuery) FindByStarted(cond kallax.ScalarCond, v time.Time) *StatsPeriodQuery {
	return q.Where(cond(Schema.StatsPeriod.Started, v))
}

// FindByDuration adds a new filter to the query that will require that
// the Duration property is equal to the passed value.
func (q *StatsPeriodQuery) FindByDuration(cond kallax.ScalarCond, v time.Duration) *StatsPeriodQuery {
	return q.Where(cond(Schema.StatsPeriod.Duration, v))
}

// FindByGuildID adds a new filter to the query that will require that
// the GuildID property is equal to the passed value.
func (q *StatsPeriodQuery) FindByGuildID(cond kallax.ScalarCond, v int64) *StatsPeriodQuery {
	return q.Where(cond(Schema.StatsPeriod.GuildID, v))
}

// FindByUserID adds a new filter to the query that will require that
// the UserID property is equal to the passed value.
func (q *StatsPeriodQuery) FindByUserID(cond kallax.ScalarCond, v int64) *StatsPeriodQuery {
	return q.Where(cond(Schema.StatsPeriod.UserID, v))
}

// FindByChannelID adds a new filter to the query that will require that
// the ChannelID property is equal to the passed value.
func (q *StatsPeriodQuery) FindByChannelID(cond kallax.ScalarCond, v int64) *StatsPeriodQuery {
	return q.Where(cond(Schema.StatsPeriod.ChannelID, v))
}

// FindByCount adds a new filter to the query that will require that
// the Count property is equal to the passed value.
func (q *StatsPeriodQuery) FindByCount(cond kallax.ScalarCond, v int64) *StatsPeriodQuery {
	return q.Where(cond(Schema.StatsPeriod.Count, v))
}

// StatsPeriodResultSet is the set of results returned by a query to the
// database.
type StatsPeriodResultSet struct {
	ResultSet kallax.ResultSet
	last      *StatsPeriod
	lastErr   error
}

// NewStatsPeriodResultSet creates a new result set for rows of the type
// StatsPeriod.
func NewStatsPeriodResultSet(rs kallax.ResultSet) *StatsPeriodResultSet {
	return &StatsPeriodResultSet{ResultSet: rs}
}

// Next fetches the next item in the result set and returns true if there is
// a next item.
// The result set is closed automatically when there are no more items.
func (rs *StatsPeriodResultSet) Next() bool {
	if !rs.ResultSet.Next() {
		rs.lastErr = rs.ResultSet.Close()
		rs.last = nil
		return false
	}

	var record kallax.Record
	record, rs.lastErr = rs.ResultSet.Get(Schema.StatsPeriod.BaseSchema)
	if rs.lastErr != nil {
		rs.last = nil
	} else {
		var ok bool
		rs.last, ok = record.(*StatsPeriod)
		if !ok {
			rs.lastErr = fmt.Errorf("kallax: unable to convert record to *StatsPeriod")
			rs.last = nil
		}
	}

	return true
}

// Get retrieves the last fetched item from the result set and the last error.
func (rs *StatsPeriodResultSet) Get() (*StatsPeriod, error) {
	return rs.last, rs.lastErr
}

// ForEach iterates over the complete result set passing every record found to
// the given callback. It is possible to stop the iteration by returning
// `kallax.ErrStop` in the callback.
// Result set is always closed at the end.
func (rs *StatsPeriodResultSet) ForEach(fn func(*StatsPeriod) error) error {
	for rs.Next() {
		record, err := rs.Get()
		if err != nil {
			return err
		}

		if err := fn(record); err != nil {
			if err == kallax.ErrStop {
				return rs.Close()
			}

			return err
		}
	}
	return nil
}

// All returns all records on the result set and closes the result set.
func (rs *StatsPeriodResultSet) All() ([]*StatsPeriod, error) {
	var result []*StatsPeriod
	for rs.Next() {
		record, err := rs.Get()
		if err != nil {
			return nil, err
		}
		result = append(result, record)
	}
	return result, nil
}

// One returns the first record on the result set and closes the result set.
func (rs *StatsPeriodResultSet) One() (*StatsPeriod, error) {
	if !rs.Next() {
		return nil, kallax.ErrNotFound
	}

	record, err := rs.Get()
	if err != nil {
		return nil, err
	}

	if err := rs.Close(); err != nil {
		return nil, err
	}

	return record, nil
}

// Err returns the last error occurred.
func (rs *StatsPeriodResultSet) Err() error {
	return rs.lastErr
}

// Close closes the result set.
func (rs *StatsPeriodResultSet) Close() error {
	return rs.ResultSet.Close()
}

type schema struct {
	StatsPeriod *schemaStatsPeriod
}

type schemaStatsPeriod struct {
	*kallax.BaseSchema
	ID        kallax.SchemaField
	Started   kallax.SchemaField
	Duration  kallax.SchemaField
	GuildID   kallax.SchemaField
	UserID    kallax.SchemaField
	ChannelID kallax.SchemaField
	Count     kallax.SchemaField
}

var Schema = &schema{
	StatsPeriod: &schemaStatsPeriod{
		BaseSchema: kallax.NewBaseSchema(
			"serverstats_periods",
			"__statsperiod",
			kallax.NewSchemaField("id"),
			kallax.ForeignKeys{},
			func() kallax.Record {
				return new(StatsPeriod)
			},
			true,
			kallax.NewSchemaField("id"),
			kallax.NewSchemaField("started"),
			kallax.NewSchemaField("duration"),
			kallax.NewSchemaField("guild_id"),
			kallax.NewSchemaField("user_id"),
			kallax.NewSchemaField("channel_id"),
			kallax.NewSchemaField("count"),
		),
		ID:        kallax.NewSchemaField("id"),
		Started:   kallax.NewSchemaField("started"),
		Duration:  kallax.NewSchemaField("duration"),
		GuildID:   kallax.NewSchemaField("guild_id"),
		UserID:    kallax.NewSchemaField("user_id"),
		ChannelID: kallax.NewSchemaField("channel_id"),
		Count:     kallax.NewSchemaField("count"),
	},
}
