// IMPORTANT! This is auto generated code by https://github.com/src-d/go-kallax
// Please, do not touch the code below, and if you do, do it under your own
// risk. Take into account that all the code you write here will be completely
// erased from earth the next time you generate the kallax models.
package mqueue

import (
	"database/sql"
	"fmt"

	"gopkg.in/src-d/go-kallax.v1"
	"gopkg.in/src-d/go-kallax.v1/types"
)

var _ types.SQLType
var _ fmt.Formatter

// NewQueuedElement returns a new instance of QueuedElement.
func NewQueuedElement() (record *QueuedElement) {
	return new(QueuedElement)
}

// GetID returns the primary key of the model.
func (r *QueuedElement) GetID() kallax.Identifier {
	return (*kallax.NumericID)(&r.ID)
}

// ColumnAddress returns the pointer to the value of the given column.
func (r *QueuedElement) ColumnAddress(col string) (interface{}, error) {
	switch col {
	case "id":
		return (*kallax.NumericID)(&r.ID), nil
	case "source":
		return &r.Source, nil
	case "source_id":
		return &r.SourceID, nil
	case "message_str":
		return &r.MessageStr, nil
	case "message_embed":
		return &r.MessageEmbed, nil
	case "channel":
		return &r.Channel, nil
	case "processed":
		return &r.Processed, nil

	default:
		return nil, fmt.Errorf("kallax: invalid column in QueuedElement: %s", col)
	}
}

// Value returns the value of the given column.
func (r *QueuedElement) Value(col string) (interface{}, error) {
	switch col {
	case "id":
		return r.ID, nil
	case "source":
		return r.Source, nil
	case "source_id":
		return r.SourceID, nil
	case "message_str":
		return r.MessageStr, nil
	case "message_embed":
		return r.MessageEmbed, nil
	case "channel":
		return r.Channel, nil
	case "processed":
		return r.Processed, nil

	default:
		return nil, fmt.Errorf("kallax: invalid column in QueuedElement: %s", col)
	}
}

// NewRelationshipRecord returns a new record for the relatiobship in the given
// field.
func (r *QueuedElement) NewRelationshipRecord(field string) (kallax.Record, error) {
	return nil, fmt.Errorf("kallax: model QueuedElement has no relationships")
}

// SetRelationship sets the given relationship in the given field.
func (r *QueuedElement) SetRelationship(field string, rel interface{}) error {
	return fmt.Errorf("kallax: model QueuedElement has no relationships")
}

// QueuedElementStore is the entity to access the records of the type QueuedElement
// in the database.
type QueuedElementStore struct {
	*kallax.Store
}

// NewQueuedElementStore creates a new instance of QueuedElementStore
// using a SQL database.
func NewQueuedElementStore(db *sql.DB) *QueuedElementStore {
	return &QueuedElementStore{kallax.NewStore(db)}
}

// GenericStore returns the generic store of this store.
func (s *QueuedElementStore) GenericStore() *kallax.Store {
	return s.Store
}

// SetGenericStore changes the generic store of this store.
func (s *QueuedElementStore) SetGenericStore(store *kallax.Store) {
	s.Store = store
}

// Debug returns a new store that will print all SQL statements to stdout using
// the log.Printf function.
func (s *QueuedElementStore) Debug() *QueuedElementStore {
	return &QueuedElementStore{s.Store.Debug()}
}

// DebugWith returns a new store that will print all SQL statements using the
// given logger function.
func (s *QueuedElementStore) DebugWith(logger kallax.LoggerFunc) *QueuedElementStore {
	return &QueuedElementStore{s.Store.DebugWith(logger)}
}

// Insert inserts a QueuedElement in the database. A non-persisted object is
// required for this operation.
func (s *QueuedElementStore) Insert(record *QueuedElement) error {
	return s.Store.Insert(Schema.QueuedElement.BaseSchema, record)
}

// Update updates the given record on the database. If the columns are given,
// only these columns will be updated. Otherwise all of them will be.
// Be very careful with this, as you will have a potentially different object
// in memory but not on the database.
// Only writable records can be updated. Writable objects are those that have
// been just inserted or retrieved using a query with no custom select fields.
func (s *QueuedElementStore) Update(record *QueuedElement, cols ...kallax.SchemaField) (updated int64, err error) {
	return s.Store.Update(Schema.QueuedElement.BaseSchema, record, cols...)
}

// Save inserts the object if the record is not persisted, otherwise it updates
// it. Same rules of Update and Insert apply depending on the case.
func (s *QueuedElementStore) Save(record *QueuedElement) (updated bool, err error) {
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
func (s *QueuedElementStore) Delete(record *QueuedElement) error {
	return s.Store.Delete(Schema.QueuedElement.BaseSchema, record)
}

// Find returns the set of results for the given query.
func (s *QueuedElementStore) Find(q *QueuedElementQuery) (*QueuedElementResultSet, error) {
	rs, err := s.Store.Find(q)
	if err != nil {
		return nil, err
	}

	return NewQueuedElementResultSet(rs), nil
}

// MustFind returns the set of results for the given query, but panics if there
// is any error.
func (s *QueuedElementStore) MustFind(q *QueuedElementQuery) *QueuedElementResultSet {
	return NewQueuedElementResultSet(s.Store.MustFind(q))
}

// Count returns the number of rows that would be retrieved with the given
// query.
func (s *QueuedElementStore) Count(q *QueuedElementQuery) (int64, error) {
	return s.Store.Count(q)
}

// MustCount returns the number of rows that would be retrieved with the given
// query, but panics if there is an error.
func (s *QueuedElementStore) MustCount(q *QueuedElementQuery) int64 {
	return s.Store.MustCount(q)
}

// FindOne returns the first row returned by the given query.
// `ErrNotFound` is returned if there are no results.
func (s *QueuedElementStore) FindOne(q *QueuedElementQuery) (*QueuedElement, error) {
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
func (s *QueuedElementStore) FindAll(q *QueuedElementQuery) ([]*QueuedElement, error) {
	rs, err := s.Find(q)
	if err != nil {
		return nil, err
	}

	return rs.All()
}

// MustFindOne returns the first row retrieved by the given query. It panics
// if there is an error or if there are no rows.
func (s *QueuedElementStore) MustFindOne(q *QueuedElementQuery) *QueuedElement {
	record, err := s.FindOne(q)
	if err != nil {
		panic(err)
	}
	return record
}

// Reload refreshes the QueuedElement with the data in the database and
// makes it writable.
func (s *QueuedElementStore) Reload(record *QueuedElement) error {
	return s.Store.Reload(Schema.QueuedElement.BaseSchema, record)
}

// Transaction executes the given callback in a transaction and rollbacks if
// an error is returned.
// The transaction is only open in the store passed as a parameter to the
// callback.
func (s *QueuedElementStore) Transaction(callback func(*QueuedElementStore) error) error {
	if callback == nil {
		return kallax.ErrInvalidTxCallback
	}

	return s.Store.Transaction(func(store *kallax.Store) error {
		return callback(&QueuedElementStore{store})
	})
}

// QueuedElementQuery is the object used to create queries for the QueuedElement
// entity.
type QueuedElementQuery struct {
	*kallax.BaseQuery
}

// NewQueuedElementQuery returns a new instance of QueuedElementQuery.
func NewQueuedElementQuery() *QueuedElementQuery {
	return &QueuedElementQuery{
		BaseQuery: kallax.NewBaseQuery(Schema.QueuedElement.BaseSchema),
	}
}

// Select adds columns to select in the query.
func (q *QueuedElementQuery) Select(columns ...kallax.SchemaField) *QueuedElementQuery {
	if len(columns) == 0 {
		return q
	}
	q.BaseQuery.Select(columns...)
	return q
}

// SelectNot excludes columns from being selected in the query.
func (q *QueuedElementQuery) SelectNot(columns ...kallax.SchemaField) *QueuedElementQuery {
	q.BaseQuery.SelectNot(columns...)
	return q
}

// Copy returns a new identical copy of the query. Remember queries are mutable
// so make a copy any time you need to reuse them.
func (q *QueuedElementQuery) Copy() *QueuedElementQuery {
	return &QueuedElementQuery{
		BaseQuery: q.BaseQuery.Copy(),
	}
}

// Order adds order clauses to the query for the given columns.
func (q *QueuedElementQuery) Order(cols ...kallax.ColumnOrder) *QueuedElementQuery {
	q.BaseQuery.Order(cols...)
	return q
}

// BatchSize sets the number of items to fetch per batch when there are 1:N
// relationships selected in the query.
func (q *QueuedElementQuery) BatchSize(size uint64) *QueuedElementQuery {
	q.BaseQuery.BatchSize(size)
	return q
}

// Limit sets the max number of items to retrieve.
func (q *QueuedElementQuery) Limit(n uint64) *QueuedElementQuery {
	q.BaseQuery.Limit(n)
	return q
}

// Offset sets the number of items to skip from the result set of items.
func (q *QueuedElementQuery) Offset(n uint64) *QueuedElementQuery {
	q.BaseQuery.Offset(n)
	return q
}

// Where adds a condition to the query. All conditions added are concatenated
// using a logical AND.
func (q *QueuedElementQuery) Where(cond kallax.Condition) *QueuedElementQuery {
	q.BaseQuery.Where(cond)
	return q
}

// FindByID adds a new filter to the query that will require that
// the ID property is equal to one of the passed values; if no passed values,
// it will do nothing.
func (q *QueuedElementQuery) FindByID(v ...int64) *QueuedElementQuery {
	if len(v) == 0 {
		return q
	}
	values := make([]interface{}, len(v))
	for i, val := range v {
		values[i] = val
	}
	return q.Where(kallax.In(Schema.QueuedElement.ID, values...))
}

// FindBySource adds a new filter to the query that will require that
// the Source property is equal to the passed value.
func (q *QueuedElementQuery) FindBySource(v string) *QueuedElementQuery {
	return q.Where(kallax.Eq(Schema.QueuedElement.Source, v))
}

// FindBySourceID adds a new filter to the query that will require that
// the SourceID property is equal to the passed value.
func (q *QueuedElementQuery) FindBySourceID(v string) *QueuedElementQuery {
	return q.Where(kallax.Eq(Schema.QueuedElement.SourceID, v))
}

// FindByMessageStr adds a new filter to the query that will require that
// the MessageStr property is equal to the passed value.
func (q *QueuedElementQuery) FindByMessageStr(v string) *QueuedElementQuery {
	return q.Where(kallax.Eq(Schema.QueuedElement.MessageStr, v))
}

// FindByMessageEmbed adds a new filter to the query that will require that
// the MessageEmbed property is equal to the passed value.
func (q *QueuedElementQuery) FindByMessageEmbed(v string) *QueuedElementQuery {
	return q.Where(kallax.Eq(Schema.QueuedElement.MessageEmbed, v))
}

// FindByChannel adds a new filter to the query that will require that
// the Channel property is equal to the passed value.
func (q *QueuedElementQuery) FindByChannel(v string) *QueuedElementQuery {
	return q.Where(kallax.Eq(Schema.QueuedElement.Channel, v))
}

// FindByProcessed adds a new filter to the query that will require that
// the Processed property is equal to the passed value.
func (q *QueuedElementQuery) FindByProcessed(v bool) *QueuedElementQuery {
	return q.Where(kallax.Eq(Schema.QueuedElement.Processed, v))
}

// QueuedElementResultSet is the set of results returned by a query to the
// database.
type QueuedElementResultSet struct {
	ResultSet kallax.ResultSet
	last      *QueuedElement
	lastErr   error
}

// NewQueuedElementResultSet creates a new result set for rows of the type
// QueuedElement.
func NewQueuedElementResultSet(rs kallax.ResultSet) *QueuedElementResultSet {
	return &QueuedElementResultSet{ResultSet: rs}
}

// Next fetches the next item in the result set and returns true if there is
// a next item.
// The result set is closed automatically when there are no more items.
func (rs *QueuedElementResultSet) Next() bool {
	if !rs.ResultSet.Next() {
		rs.lastErr = rs.ResultSet.Close()
		rs.last = nil
		return false
	}

	var record kallax.Record
	record, rs.lastErr = rs.ResultSet.Get(Schema.QueuedElement.BaseSchema)
	if rs.lastErr != nil {
		rs.last = nil
	} else {
		var ok bool
		rs.last, ok = record.(*QueuedElement)
		if !ok {
			rs.lastErr = fmt.Errorf("kallax: unable to convert record to *QueuedElement")
			rs.last = nil
		}
	}

	return true
}

// Get retrieves the last fetched item from the result set and the last error.
func (rs *QueuedElementResultSet) Get() (*QueuedElement, error) {
	return rs.last, rs.lastErr
}

// ForEach iterates over the complete result set passing every record found to
// the given callback. It is possible to stop the iteration by returning
// `kallax.ErrStop` in the callback.
// Result set is always closed at the end.
func (rs *QueuedElementResultSet) ForEach(fn func(*QueuedElement) error) error {
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
func (rs *QueuedElementResultSet) All() ([]*QueuedElement, error) {
	var result []*QueuedElement
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
func (rs *QueuedElementResultSet) One() (*QueuedElement, error) {
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
func (rs *QueuedElementResultSet) Err() error {
	return rs.lastErr
}

// Close closes the result set.
func (rs *QueuedElementResultSet) Close() error {
	return rs.ResultSet.Close()
}

type schema struct {
	QueuedElement *schemaQueuedElement
}

type schemaQueuedElement struct {
	*kallax.BaseSchema
	ID           kallax.SchemaField
	Source       kallax.SchemaField
	SourceID     kallax.SchemaField
	MessageStr   kallax.SchemaField
	MessageEmbed kallax.SchemaField
	Channel      kallax.SchemaField
	Processed    kallax.SchemaField
}

var Schema = &schema{
	QueuedElement: &schemaQueuedElement{
		BaseSchema: kallax.NewBaseSchema(
			"mqueue",
			"__queuedelement",
			kallax.NewSchemaField("id"),
			kallax.ForeignKeys{},
			func() kallax.Record {
				return new(QueuedElement)
			},
			true,
			kallax.NewSchemaField("id"),
			kallax.NewSchemaField("source"),
			kallax.NewSchemaField("source_id"),
			kallax.NewSchemaField("message_str"),
			kallax.NewSchemaField("message_embed"),
			kallax.NewSchemaField("channel"),
			kallax.NewSchemaField("processed"),
		),
		ID:           kallax.NewSchemaField("id"),
		Source:       kallax.NewSchemaField("source"),
		SourceID:     kallax.NewSchemaField("source_id"),
		MessageStr:   kallax.NewSchemaField("message_str"),
		MessageEmbed: kallax.NewSchemaField("message_embed"),
		Channel:      kallax.NewSchemaField("channel"),
		Processed:    kallax.NewSchemaField("processed"),
	},
}
