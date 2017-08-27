// IMPORTANT! This is auto generated code by https://github.com/src-d/go-kallax
// Please, do not touch the code below, and if you do, do it under your own
// risk. Take into account that all the code you write here will be completely
// erased from earth the next time you generate the kallax models.
package rolecommands

import (
	"database/sql"
	"fmt"
	"time"

	"gopkg.in/src-d/go-kallax.v1"
	"gopkg.in/src-d/go-kallax.v1/types"
)

var _ types.SQLType
var _ fmt.Formatter

// NewRoleCommand returns a new instance of RoleCommand.
func NewRoleCommand() (record *RoleCommand) {
	return newRoleCommand()
}

// GetID returns the primary key of the model.
func (r *RoleCommand) GetID() kallax.Identifier {
	return (*kallax.NumericID)(&r.ID)
}

// ColumnAddress returns the pointer to the value of the given column.
func (r *RoleCommand) ColumnAddress(col string) (interface{}, error) {
	switch col {
	case "id":
		return (*kallax.NumericID)(&r.ID), nil
	case "created_at":
		return &r.Timestamps.CreatedAt, nil
	case "updated_at":
		return &r.Timestamps.UpdatedAt, nil
	case "guild_id":
		return &r.GuildID, nil
	case "name":
		return &r.Name, nil
	case "role_group_id":
		return types.Nullable(kallax.VirtualColumn("role_group_id", r, new(kallax.NumericID))), nil
	case "role":
		return &r.Role, nil
	case "require_roles":
		return types.Slice(&r.RequireRoles), nil
	case "ignore_roles":
		return types.Slice(&r.IgnoreRoles), nil
	case "position":
		return &r.Position, nil

	default:
		return nil, fmt.Errorf("kallax: invalid column in RoleCommand: %s", col)
	}
}

// Value returns the value of the given column.
func (r *RoleCommand) Value(col string) (interface{}, error) {
	switch col {
	case "id":
		return r.ID, nil
	case "created_at":
		return r.Timestamps.CreatedAt, nil
	case "updated_at":
		return r.Timestamps.UpdatedAt, nil
	case "guild_id":
		return r.GuildID, nil
	case "name":
		return r.Name, nil
	case "role_group_id":
		return r.Model.VirtualColumn(col), nil
	case "role":
		return r.Role, nil
	case "require_roles":
		return types.Slice(r.RequireRoles), nil
	case "ignore_roles":
		return types.Slice(r.IgnoreRoles), nil
	case "position":
		return r.Position, nil

	default:
		return nil, fmt.Errorf("kallax: invalid column in RoleCommand: %s", col)
	}
}

// NewRelationshipRecord returns a new record for the relatiobship in the given
// field.
func (r *RoleCommand) NewRelationshipRecord(field string) (kallax.Record, error) {
	switch field {
	case "Group":
		return new(RoleGroup), nil

	}
	return nil, fmt.Errorf("kallax: model RoleCommand has no relationship %s", field)
}

// SetRelationship sets the given relationship in the given field.
func (r *RoleCommand) SetRelationship(field string, rel interface{}) error {
	switch field {
	case "Group":
		val, ok := rel.(*RoleGroup)
		if !ok {
			return fmt.Errorf("kallax: record of type %t can't be assigned to relationship Group", rel)
		}
		if !val.GetID().IsEmpty() {
			r.Group = val
		}

		return nil

	}
	return fmt.Errorf("kallax: model RoleCommand has no relationship %s", field)
}

// RoleCommandStore is the entity to access the records of the type RoleCommand
// in the database.
type RoleCommandStore struct {
	*kallax.Store
}

// NewRoleCommandStore creates a new instance of RoleCommandStore
// using a SQL database.
func NewRoleCommandStore(db *sql.DB) *RoleCommandStore {
	return &RoleCommandStore{kallax.NewStore(db)}
}

// GenericStore returns the generic store of this store.
func (s *RoleCommandStore) GenericStore() *kallax.Store {
	return s.Store
}

// SetGenericStore changes the generic store of this store.
func (s *RoleCommandStore) SetGenericStore(store *kallax.Store) {
	s.Store = store
}

// Debug returns a new store that will print all SQL statements to stdout using
// the log.Printf function.
func (s *RoleCommandStore) Debug() *RoleCommandStore {
	return &RoleCommandStore{s.Store.Debug()}
}

// DebugWith returns a new store that will print all SQL statements using the
// given logger function.
func (s *RoleCommandStore) DebugWith(logger kallax.LoggerFunc) *RoleCommandStore {
	return &RoleCommandStore{s.Store.DebugWith(logger)}
}

func (s *RoleCommandStore) inverseRecords(record *RoleCommand) []kallax.RecordWithSchema {
	record.ClearVirtualColumns()
	var records []kallax.RecordWithSchema

	if record.Group != nil {
		record.AddVirtualColumn("role_group_id", record.Group.GetID())
		records = append(records, kallax.RecordWithSchema{
			Schema: Schema.RoleGroup.BaseSchema,
			Record: record.Group,
		})
	}

	return records
}

// Insert inserts a RoleCommand in the database. A non-persisted object is
// required for this operation.
func (s *RoleCommandStore) Insert(record *RoleCommand) error {
	record.CreatedAt = record.CreatedAt.Truncate(time.Microsecond)
	record.UpdatedAt = record.UpdatedAt.Truncate(time.Microsecond)

	if err := record.BeforeSave(); err != nil {
		return err
	}

	inverseRecords := s.inverseRecords(record)

	if len(inverseRecords) > 0 {
		return s.Store.Transaction(func(s *kallax.Store) error {
			for _, r := range inverseRecords {
				if err := kallax.ApplyBeforeEvents(r.Record); err != nil {
					return err
				}
				persisted := r.Record.IsPersisted()

				if _, err := s.Save(r.Schema, r.Record); err != nil {
					return err
				}

				if err := kallax.ApplyAfterEvents(r.Record, persisted); err != nil {
					return err
				}
			}

			if err := s.Insert(Schema.RoleCommand.BaseSchema, record); err != nil {
				return err
			}

			return nil
		})
	}

	return s.Store.Insert(Schema.RoleCommand.BaseSchema, record)
}

// Update updates the given record on the database. If the columns are given,
// only these columns will be updated. Otherwise all of them will be.
// Be very careful with this, as you will have a potentially different object
// in memory but not on the database.
// Only writable records can be updated. Writable objects are those that have
// been just inserted or retrieved using a query with no custom select fields.
func (s *RoleCommandStore) Update(record *RoleCommand, cols ...kallax.SchemaField) (updated int64, err error) {
	record.CreatedAt = record.CreatedAt.Truncate(time.Microsecond)
	record.UpdatedAt = record.UpdatedAt.Truncate(time.Microsecond)

	if err := record.BeforeSave(); err != nil {
		return 0, err
	}

	inverseRecords := s.inverseRecords(record)

	if len(inverseRecords) > 0 {
		err = s.Store.Transaction(func(s *kallax.Store) error {
			for _, r := range inverseRecords {
				if err := kallax.ApplyBeforeEvents(r.Record); err != nil {
					return err
				}
				persisted := r.Record.IsPersisted()

				if _, err := s.Save(r.Schema, r.Record); err != nil {
					return err
				}

				if err := kallax.ApplyAfterEvents(r.Record, persisted); err != nil {
					return err
				}
			}

			updated, err = s.Update(Schema.RoleCommand.BaseSchema, record, cols...)
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			return 0, err
		}

		return updated, nil
	}

	return s.Store.Update(Schema.RoleCommand.BaseSchema, record, cols...)
}

// Save inserts the object if the record is not persisted, otherwise it updates
// it. Same rules of Update and Insert apply depending on the case.
func (s *RoleCommandStore) Save(record *RoleCommand) (updated bool, err error) {
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
func (s *RoleCommandStore) Delete(record *RoleCommand) error {
	return s.Store.Delete(Schema.RoleCommand.BaseSchema, record)
}

// Find returns the set of results for the given query.
func (s *RoleCommandStore) Find(q *RoleCommandQuery) (*RoleCommandResultSet, error) {
	rs, err := s.Store.Find(q)
	if err != nil {
		return nil, err
	}

	return NewRoleCommandResultSet(rs), nil
}

// MustFind returns the set of results for the given query, but panics if there
// is any error.
func (s *RoleCommandStore) MustFind(q *RoleCommandQuery) *RoleCommandResultSet {
	return NewRoleCommandResultSet(s.Store.MustFind(q))
}

// Count returns the number of rows that would be retrieved with the given
// query.
func (s *RoleCommandStore) Count(q *RoleCommandQuery) (int64, error) {
	return s.Store.Count(q)
}

// MustCount returns the number of rows that would be retrieved with the given
// query, but panics if there is an error.
func (s *RoleCommandStore) MustCount(q *RoleCommandQuery) int64 {
	return s.Store.MustCount(q)
}

// FindOne returns the first row returned by the given query.
// `ErrNotFound` is returned if there are no results.
func (s *RoleCommandStore) FindOne(q *RoleCommandQuery) (*RoleCommand, error) {
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
func (s *RoleCommandStore) FindAll(q *RoleCommandQuery) ([]*RoleCommand, error) {
	rs, err := s.Find(q)
	if err != nil {
		return nil, err
	}

	return rs.All()
}

// MustFindOne returns the first row retrieved by the given query. It panics
// if there is an error or if there are no rows.
func (s *RoleCommandStore) MustFindOne(q *RoleCommandQuery) *RoleCommand {
	record, err := s.FindOne(q)
	if err != nil {
		panic(err)
	}
	return record
}

// Reload refreshes the RoleCommand with the data in the database and
// makes it writable.
func (s *RoleCommandStore) Reload(record *RoleCommand) error {
	return s.Store.Reload(Schema.RoleCommand.BaseSchema, record)
}

// Transaction executes the given callback in a transaction and rollbacks if
// an error is returned.
// The transaction is only open in the store passed as a parameter to the
// callback.
func (s *RoleCommandStore) Transaction(callback func(*RoleCommandStore) error) error {
	if callback == nil {
		return kallax.ErrInvalidTxCallback
	}

	return s.Store.Transaction(func(store *kallax.Store) error {
		return callback(&RoleCommandStore{store})
	})
}

// RoleCommandQuery is the object used to create queries for the RoleCommand
// entity.
type RoleCommandQuery struct {
	*kallax.BaseQuery
}

// NewRoleCommandQuery returns a new instance of RoleCommandQuery.
func NewRoleCommandQuery() *RoleCommandQuery {
	return &RoleCommandQuery{
		BaseQuery: kallax.NewBaseQuery(Schema.RoleCommand.BaseSchema),
	}
}

// Select adds columns to select in the query.
func (q *RoleCommandQuery) Select(columns ...kallax.SchemaField) *RoleCommandQuery {
	if len(columns) == 0 {
		return q
	}
	q.BaseQuery.Select(columns...)
	return q
}

// SelectNot excludes columns from being selected in the query.
func (q *RoleCommandQuery) SelectNot(columns ...kallax.SchemaField) *RoleCommandQuery {
	q.BaseQuery.SelectNot(columns...)
	return q
}

// Copy returns a new identical copy of the query. Remember queries are mutable
// so make a copy any time you need to reuse them.
func (q *RoleCommandQuery) Copy() *RoleCommandQuery {
	return &RoleCommandQuery{
		BaseQuery: q.BaseQuery.Copy(),
	}
}

// Order adds order clauses to the query for the given columns.
func (q *RoleCommandQuery) Order(cols ...kallax.ColumnOrder) *RoleCommandQuery {
	q.BaseQuery.Order(cols...)
	return q
}

// BatchSize sets the number of items to fetch per batch when there are 1:N
// relationships selected in the query.
func (q *RoleCommandQuery) BatchSize(size uint64) *RoleCommandQuery {
	q.BaseQuery.BatchSize(size)
	return q
}

// Limit sets the max number of items to retrieve.
func (q *RoleCommandQuery) Limit(n uint64) *RoleCommandQuery {
	q.BaseQuery.Limit(n)
	return q
}

// Offset sets the number of items to skip from the result set of items.
func (q *RoleCommandQuery) Offset(n uint64) *RoleCommandQuery {
	q.BaseQuery.Offset(n)
	return q
}

// Where adds a condition to the query. All conditions added are concatenated
// using a logical AND.
func (q *RoleCommandQuery) Where(cond kallax.Condition) *RoleCommandQuery {
	q.BaseQuery.Where(cond)
	return q
}

func (q *RoleCommandQuery) WithGroup() *RoleCommandQuery {
	q.AddRelation(Schema.RoleGroup.BaseSchema, "Group", kallax.OneToOne, nil)
	return q
}

// FindByID adds a new filter to the query that will require that
// the ID property is equal to one of the passed values; if no passed values,
// it will do nothing.
func (q *RoleCommandQuery) FindByID(v ...int64) *RoleCommandQuery {
	if len(v) == 0 {
		return q
	}
	values := make([]interface{}, len(v))
	for i, val := range v {
		values[i] = val
	}
	return q.Where(kallax.In(Schema.RoleCommand.ID, values...))
}

// FindByCreatedAt adds a new filter to the query that will require that
// the CreatedAt property is equal to the passed value.
func (q *RoleCommandQuery) FindByCreatedAt(cond kallax.ScalarCond, v time.Time) *RoleCommandQuery {
	return q.Where(cond(Schema.RoleCommand.CreatedAt, v))
}

// FindByUpdatedAt adds a new filter to the query that will require that
// the UpdatedAt property is equal to the passed value.
func (q *RoleCommandQuery) FindByUpdatedAt(cond kallax.ScalarCond, v time.Time) *RoleCommandQuery {
	return q.Where(cond(Schema.RoleCommand.UpdatedAt, v))
}

// FindByGuildID adds a new filter to the query that will require that
// the GuildID property is equal to the passed value.
func (q *RoleCommandQuery) FindByGuildID(cond kallax.ScalarCond, v int64) *RoleCommandQuery {
	return q.Where(cond(Schema.RoleCommand.GuildID, v))
}

// FindByName adds a new filter to the query that will require that
// the Name property is equal to the passed value.
func (q *RoleCommandQuery) FindByName(v string) *RoleCommandQuery {
	return q.Where(kallax.Eq(Schema.RoleCommand.Name, v))
}

// FindByGroup adds a new filter to the query that will require that
// the foreign key of Group is equal to the passed value.
func (q *RoleCommandQuery) FindByGroup(v int64) *RoleCommandQuery {
	return q.Where(kallax.Eq(Schema.RoleCommand.GroupFK, v))
}

// FindByRole adds a new filter to the query that will require that
// the Role property is equal to the passed value.
func (q *RoleCommandQuery) FindByRole(cond kallax.ScalarCond, v int64) *RoleCommandQuery {
	return q.Where(cond(Schema.RoleCommand.Role, v))
}

// FindByRequireRoles adds a new filter to the query that will require that
// the RequireRoles property contains all the passed values; if no passed values,
// it will do nothing.
func (q *RoleCommandQuery) FindByRequireRoles(v ...int64) *RoleCommandQuery {
	if len(v) == 0 {
		return q
	}
	values := make([]interface{}, len(v))
	for i, val := range v {
		values[i] = val
	}
	return q.Where(kallax.ArrayContains(Schema.RoleCommand.RequireRoles, values...))
}

// FindByIgnoreRoles adds a new filter to the query that will require that
// the IgnoreRoles property contains all the passed values; if no passed values,
// it will do nothing.
func (q *RoleCommandQuery) FindByIgnoreRoles(v ...int64) *RoleCommandQuery {
	if len(v) == 0 {
		return q
	}
	values := make([]interface{}, len(v))
	for i, val := range v {
		values[i] = val
	}
	return q.Where(kallax.ArrayContains(Schema.RoleCommand.IgnoreRoles, values...))
}

// FindByPosition adds a new filter to the query that will require that
// the Position property is equal to the passed value.
func (q *RoleCommandQuery) FindByPosition(cond kallax.ScalarCond, v int) *RoleCommandQuery {
	return q.Where(cond(Schema.RoleCommand.Position, v))
}

// RoleCommandResultSet is the set of results returned by a query to the
// database.
type RoleCommandResultSet struct {
	ResultSet kallax.ResultSet
	last      *RoleCommand
	lastErr   error
}

// NewRoleCommandResultSet creates a new result set for rows of the type
// RoleCommand.
func NewRoleCommandResultSet(rs kallax.ResultSet) *RoleCommandResultSet {
	return &RoleCommandResultSet{ResultSet: rs}
}

// Next fetches the next item in the result set and returns true if there is
// a next item.
// The result set is closed automatically when there are no more items.
func (rs *RoleCommandResultSet) Next() bool {
	if !rs.ResultSet.Next() {
		rs.lastErr = rs.ResultSet.Close()
		rs.last = nil
		return false
	}

	var record kallax.Record
	record, rs.lastErr = rs.ResultSet.Get(Schema.RoleCommand.BaseSchema)
	if rs.lastErr != nil {
		rs.last = nil
	} else {
		var ok bool
		rs.last, ok = record.(*RoleCommand)
		if !ok {
			rs.lastErr = fmt.Errorf("kallax: unable to convert record to *RoleCommand")
			rs.last = nil
		}
	}

	return true
}

// Get retrieves the last fetched item from the result set and the last error.
func (rs *RoleCommandResultSet) Get() (*RoleCommand, error) {
	return rs.last, rs.lastErr
}

// ForEach iterates over the complete result set passing every record found to
// the given callback. It is possible to stop the iteration by returning
// `kallax.ErrStop` in the callback.
// Result set is always closed at the end.
func (rs *RoleCommandResultSet) ForEach(fn func(*RoleCommand) error) error {
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
func (rs *RoleCommandResultSet) All() ([]*RoleCommand, error) {
	var result []*RoleCommand
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
func (rs *RoleCommandResultSet) One() (*RoleCommand, error) {
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
func (rs *RoleCommandResultSet) Err() error {
	return rs.lastErr
}

// Close closes the result set.
func (rs *RoleCommandResultSet) Close() error {
	return rs.ResultSet.Close()
}

// NewRoleGroup returns a new instance of RoleGroup.
func NewRoleGroup() (record *RoleGroup) {
	return new(RoleGroup)
}

// GetID returns the primary key of the model.
func (r *RoleGroup) GetID() kallax.Identifier {
	return (*kallax.NumericID)(&r.ID)
}

// ColumnAddress returns the pointer to the value of the given column.
func (r *RoleGroup) ColumnAddress(col string) (interface{}, error) {
	switch col {
	case "id":
		return (*kallax.NumericID)(&r.ID), nil
	case "guild_id":
		return &r.GuildID, nil
	case "name":
		return &r.Name, nil
	case "require_roles":
		return types.Slice(&r.RequireRoles), nil
	case "ignore_roles":
		return types.Slice(&r.IgnoreRoles), nil
	case "mode":
		return (*int)(&r.Mode), nil
	case "multiple_max":
		return &r.MultipleMax, nil
	case "multiple_min":
		return &r.MultipleMin, nil
	case "single_auto_toggle_off":
		return &r.SingleAutoToggleOff, nil
	case "single_require_one":
		return &r.SingleRequireOne, nil

	default:
		return nil, fmt.Errorf("kallax: invalid column in RoleGroup: %s", col)
	}
}

// Value returns the value of the given column.
func (r *RoleGroup) Value(col string) (interface{}, error) {
	switch col {
	case "id":
		return r.ID, nil
	case "guild_id":
		return r.GuildID, nil
	case "name":
		return r.Name, nil
	case "require_roles":
		return types.Slice(r.RequireRoles), nil
	case "ignore_roles":
		return types.Slice(r.IgnoreRoles), nil
	case "mode":
		return (int)(r.Mode), nil
	case "multiple_max":
		return r.MultipleMax, nil
	case "multiple_min":
		return r.MultipleMin, nil
	case "single_auto_toggle_off":
		return r.SingleAutoToggleOff, nil
	case "single_require_one":
		return r.SingleRequireOne, nil

	default:
		return nil, fmt.Errorf("kallax: invalid column in RoleGroup: %s", col)
	}
}

// NewRelationshipRecord returns a new record for the relatiobship in the given
// field.
func (r *RoleGroup) NewRelationshipRecord(field string) (kallax.Record, error) {
	return nil, fmt.Errorf("kallax: model RoleGroup has no relationships")
}

// SetRelationship sets the given relationship in the given field.
func (r *RoleGroup) SetRelationship(field string, rel interface{}) error {
	return fmt.Errorf("kallax: model RoleGroup has no relationships")
}

// RoleGroupStore is the entity to access the records of the type RoleGroup
// in the database.
type RoleGroupStore struct {
	*kallax.Store
}

// NewRoleGroupStore creates a new instance of RoleGroupStore
// using a SQL database.
func NewRoleGroupStore(db *sql.DB) *RoleGroupStore {
	return &RoleGroupStore{kallax.NewStore(db)}
}

// GenericStore returns the generic store of this store.
func (s *RoleGroupStore) GenericStore() *kallax.Store {
	return s.Store
}

// SetGenericStore changes the generic store of this store.
func (s *RoleGroupStore) SetGenericStore(store *kallax.Store) {
	s.Store = store
}

// Debug returns a new store that will print all SQL statements to stdout using
// the log.Printf function.
func (s *RoleGroupStore) Debug() *RoleGroupStore {
	return &RoleGroupStore{s.Store.Debug()}
}

// DebugWith returns a new store that will print all SQL statements using the
// given logger function.
func (s *RoleGroupStore) DebugWith(logger kallax.LoggerFunc) *RoleGroupStore {
	return &RoleGroupStore{s.Store.DebugWith(logger)}
}

// Insert inserts a RoleGroup in the database. A non-persisted object is
// required for this operation.
func (s *RoleGroupStore) Insert(record *RoleGroup) error {
	return s.Store.Insert(Schema.RoleGroup.BaseSchema, record)
}

// Update updates the given record on the database. If the columns are given,
// only these columns will be updated. Otherwise all of them will be.
// Be very careful with this, as you will have a potentially different object
// in memory but not on the database.
// Only writable records can be updated. Writable objects are those that have
// been just inserted or retrieved using a query with no custom select fields.
func (s *RoleGroupStore) Update(record *RoleGroup, cols ...kallax.SchemaField) (updated int64, err error) {
	return s.Store.Update(Schema.RoleGroup.BaseSchema, record, cols...)
}

// Save inserts the object if the record is not persisted, otherwise it updates
// it. Same rules of Update and Insert apply depending on the case.
func (s *RoleGroupStore) Save(record *RoleGroup) (updated bool, err error) {
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
func (s *RoleGroupStore) Delete(record *RoleGroup) error {
	return s.Store.Delete(Schema.RoleGroup.BaseSchema, record)
}

// Find returns the set of results for the given query.
func (s *RoleGroupStore) Find(q *RoleGroupQuery) (*RoleGroupResultSet, error) {
	rs, err := s.Store.Find(q)
	if err != nil {
		return nil, err
	}

	return NewRoleGroupResultSet(rs), nil
}

// MustFind returns the set of results for the given query, but panics if there
// is any error.
func (s *RoleGroupStore) MustFind(q *RoleGroupQuery) *RoleGroupResultSet {
	return NewRoleGroupResultSet(s.Store.MustFind(q))
}

// Count returns the number of rows that would be retrieved with the given
// query.
func (s *RoleGroupStore) Count(q *RoleGroupQuery) (int64, error) {
	return s.Store.Count(q)
}

// MustCount returns the number of rows that would be retrieved with the given
// query, but panics if there is an error.
func (s *RoleGroupStore) MustCount(q *RoleGroupQuery) int64 {
	return s.Store.MustCount(q)
}

// FindOne returns the first row returned by the given query.
// `ErrNotFound` is returned if there are no results.
func (s *RoleGroupStore) FindOne(q *RoleGroupQuery) (*RoleGroup, error) {
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
func (s *RoleGroupStore) FindAll(q *RoleGroupQuery) ([]*RoleGroup, error) {
	rs, err := s.Find(q)
	if err != nil {
		return nil, err
	}

	return rs.All()
}

// MustFindOne returns the first row retrieved by the given query. It panics
// if there is an error or if there are no rows.
func (s *RoleGroupStore) MustFindOne(q *RoleGroupQuery) *RoleGroup {
	record, err := s.FindOne(q)
	if err != nil {
		panic(err)
	}
	return record
}

// Reload refreshes the RoleGroup with the data in the database and
// makes it writable.
func (s *RoleGroupStore) Reload(record *RoleGroup) error {
	return s.Store.Reload(Schema.RoleGroup.BaseSchema, record)
}

// Transaction executes the given callback in a transaction and rollbacks if
// an error is returned.
// The transaction is only open in the store passed as a parameter to the
// callback.
func (s *RoleGroupStore) Transaction(callback func(*RoleGroupStore) error) error {
	if callback == nil {
		return kallax.ErrInvalidTxCallback
	}

	return s.Store.Transaction(func(store *kallax.Store) error {
		return callback(&RoleGroupStore{store})
	})
}

// RoleGroupQuery is the object used to create queries for the RoleGroup
// entity.
type RoleGroupQuery struct {
	*kallax.BaseQuery
}

// NewRoleGroupQuery returns a new instance of RoleGroupQuery.
func NewRoleGroupQuery() *RoleGroupQuery {
	return &RoleGroupQuery{
		BaseQuery: kallax.NewBaseQuery(Schema.RoleGroup.BaseSchema),
	}
}

// Select adds columns to select in the query.
func (q *RoleGroupQuery) Select(columns ...kallax.SchemaField) *RoleGroupQuery {
	if len(columns) == 0 {
		return q
	}
	q.BaseQuery.Select(columns...)
	return q
}

// SelectNot excludes columns from being selected in the query.
func (q *RoleGroupQuery) SelectNot(columns ...kallax.SchemaField) *RoleGroupQuery {
	q.BaseQuery.SelectNot(columns...)
	return q
}

// Copy returns a new identical copy of the query. Remember queries are mutable
// so make a copy any time you need to reuse them.
func (q *RoleGroupQuery) Copy() *RoleGroupQuery {
	return &RoleGroupQuery{
		BaseQuery: q.BaseQuery.Copy(),
	}
}

// Order adds order clauses to the query for the given columns.
func (q *RoleGroupQuery) Order(cols ...kallax.ColumnOrder) *RoleGroupQuery {
	q.BaseQuery.Order(cols...)
	return q
}

// BatchSize sets the number of items to fetch per batch when there are 1:N
// relationships selected in the query.
func (q *RoleGroupQuery) BatchSize(size uint64) *RoleGroupQuery {
	q.BaseQuery.BatchSize(size)
	return q
}

// Limit sets the max number of items to retrieve.
func (q *RoleGroupQuery) Limit(n uint64) *RoleGroupQuery {
	q.BaseQuery.Limit(n)
	return q
}

// Offset sets the number of items to skip from the result set of items.
func (q *RoleGroupQuery) Offset(n uint64) *RoleGroupQuery {
	q.BaseQuery.Offset(n)
	return q
}

// Where adds a condition to the query. All conditions added are concatenated
// using a logical AND.
func (q *RoleGroupQuery) Where(cond kallax.Condition) *RoleGroupQuery {
	q.BaseQuery.Where(cond)
	return q
}

// FindByID adds a new filter to the query that will require that
// the ID property is equal to one of the passed values; if no passed values,
// it will do nothing.
func (q *RoleGroupQuery) FindByID(v ...int64) *RoleGroupQuery {
	if len(v) == 0 {
		return q
	}
	values := make([]interface{}, len(v))
	for i, val := range v {
		values[i] = val
	}
	return q.Where(kallax.In(Schema.RoleGroup.ID, values...))
}

// FindByGuildID adds a new filter to the query that will require that
// the GuildID property is equal to the passed value.
func (q *RoleGroupQuery) FindByGuildID(cond kallax.ScalarCond, v int64) *RoleGroupQuery {
	return q.Where(cond(Schema.RoleGroup.GuildID, v))
}

// FindByName adds a new filter to the query that will require that
// the Name property is equal to the passed value.
func (q *RoleGroupQuery) FindByName(v string) *RoleGroupQuery {
	return q.Where(kallax.Eq(Schema.RoleGroup.Name, v))
}

// FindByRequireRoles adds a new filter to the query that will require that
// the RequireRoles property contains all the passed values; if no passed values,
// it will do nothing.
func (q *RoleGroupQuery) FindByRequireRoles(v ...int64) *RoleGroupQuery {
	if len(v) == 0 {
		return q
	}
	values := make([]interface{}, len(v))
	for i, val := range v {
		values[i] = val
	}
	return q.Where(kallax.ArrayContains(Schema.RoleGroup.RequireRoles, values...))
}

// FindByIgnoreRoles adds a new filter to the query that will require that
// the IgnoreRoles property contains all the passed values; if no passed values,
// it will do nothing.
func (q *RoleGroupQuery) FindByIgnoreRoles(v ...int64) *RoleGroupQuery {
	if len(v) == 0 {
		return q
	}
	values := make([]interface{}, len(v))
	for i, val := range v {
		values[i] = val
	}
	return q.Where(kallax.ArrayContains(Schema.RoleGroup.IgnoreRoles, values...))
}

// FindByMode adds a new filter to the query that will require that
// the Mode property is equal to the passed value.
func (q *RoleGroupQuery) FindByMode(cond kallax.ScalarCond, v GroupMode) *RoleGroupQuery {
	return q.Where(cond(Schema.RoleGroup.Mode, v))
}

// FindByMultipleMax adds a new filter to the query that will require that
// the MultipleMax property is equal to the passed value.
func (q *RoleGroupQuery) FindByMultipleMax(cond kallax.ScalarCond, v int) *RoleGroupQuery {
	return q.Where(cond(Schema.RoleGroup.MultipleMax, v))
}

// FindByMultipleMin adds a new filter to the query that will require that
// the MultipleMin property is equal to the passed value.
func (q *RoleGroupQuery) FindByMultipleMin(cond kallax.ScalarCond, v int) *RoleGroupQuery {
	return q.Where(cond(Schema.RoleGroup.MultipleMin, v))
}

// FindBySingleAutoToggleOff adds a new filter to the query that will require that
// the SingleAutoToggleOff property is equal to the passed value.
func (q *RoleGroupQuery) FindBySingleAutoToggleOff(v bool) *RoleGroupQuery {
	return q.Where(kallax.Eq(Schema.RoleGroup.SingleAutoToggleOff, v))
}

// FindBySingleRequireOne adds a new filter to the query that will require that
// the SingleRequireOne property is equal to the passed value.
func (q *RoleGroupQuery) FindBySingleRequireOne(v bool) *RoleGroupQuery {
	return q.Where(kallax.Eq(Schema.RoleGroup.SingleRequireOne, v))
}

// RoleGroupResultSet is the set of results returned by a query to the
// database.
type RoleGroupResultSet struct {
	ResultSet kallax.ResultSet
	last      *RoleGroup
	lastErr   error
}

// NewRoleGroupResultSet creates a new result set for rows of the type
// RoleGroup.
func NewRoleGroupResultSet(rs kallax.ResultSet) *RoleGroupResultSet {
	return &RoleGroupResultSet{ResultSet: rs}
}

// Next fetches the next item in the result set and returns true if there is
// a next item.
// The result set is closed automatically when there are no more items.
func (rs *RoleGroupResultSet) Next() bool {
	if !rs.ResultSet.Next() {
		rs.lastErr = rs.ResultSet.Close()
		rs.last = nil
		return false
	}

	var record kallax.Record
	record, rs.lastErr = rs.ResultSet.Get(Schema.RoleGroup.BaseSchema)
	if rs.lastErr != nil {
		rs.last = nil
	} else {
		var ok bool
		rs.last, ok = record.(*RoleGroup)
		if !ok {
			rs.lastErr = fmt.Errorf("kallax: unable to convert record to *RoleGroup")
			rs.last = nil
		}
	}

	return true
}

// Get retrieves the last fetched item from the result set and the last error.
func (rs *RoleGroupResultSet) Get() (*RoleGroup, error) {
	return rs.last, rs.lastErr
}

// ForEach iterates over the complete result set passing every record found to
// the given callback. It is possible to stop the iteration by returning
// `kallax.ErrStop` in the callback.
// Result set is always closed at the end.
func (rs *RoleGroupResultSet) ForEach(fn func(*RoleGroup) error) error {
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
func (rs *RoleGroupResultSet) All() ([]*RoleGroup, error) {
	var result []*RoleGroup
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
func (rs *RoleGroupResultSet) One() (*RoleGroup, error) {
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
func (rs *RoleGroupResultSet) Err() error {
	return rs.lastErr
}

// Close closes the result set.
func (rs *RoleGroupResultSet) Close() error {
	return rs.ResultSet.Close()
}

type schema struct {
	RoleCommand *schemaRoleCommand
	RoleGroup   *schemaRoleGroup
}

type schemaRoleCommand struct {
	*kallax.BaseSchema
	ID           kallax.SchemaField
	CreatedAt    kallax.SchemaField
	UpdatedAt    kallax.SchemaField
	GuildID      kallax.SchemaField
	Name         kallax.SchemaField
	GroupFK      kallax.SchemaField
	Role         kallax.SchemaField
	RequireRoles kallax.SchemaField
	IgnoreRoles  kallax.SchemaField
	Position     kallax.SchemaField
}

type schemaRoleGroup struct {
	*kallax.BaseSchema
	ID                  kallax.SchemaField
	GuildID             kallax.SchemaField
	Name                kallax.SchemaField
	RequireRoles        kallax.SchemaField
	IgnoreRoles         kallax.SchemaField
	Mode                kallax.SchemaField
	MultipleMax         kallax.SchemaField
	MultipleMin         kallax.SchemaField
	SingleAutoToggleOff kallax.SchemaField
	SingleRequireOne    kallax.SchemaField
}

var Schema = &schema{
	RoleCommand: &schemaRoleCommand{
		BaseSchema: kallax.NewBaseSchema(
			"role_commands",
			"__rolecommand",
			kallax.NewSchemaField("id"),
			kallax.ForeignKeys{
				"Group": kallax.NewForeignKey("role_group_id", true),
			},
			func() kallax.Record {
				return new(RoleCommand)
			},
			true,
			kallax.NewSchemaField("id"),
			kallax.NewSchemaField("created_at"),
			kallax.NewSchemaField("updated_at"),
			kallax.NewSchemaField("guild_id"),
			kallax.NewSchemaField("name"),
			kallax.NewSchemaField("role_group_id"),
			kallax.NewSchemaField("role"),
			kallax.NewSchemaField("require_roles"),
			kallax.NewSchemaField("ignore_roles"),
			kallax.NewSchemaField("position"),
		),
		ID:           kallax.NewSchemaField("id"),
		CreatedAt:    kallax.NewSchemaField("created_at"),
		UpdatedAt:    kallax.NewSchemaField("updated_at"),
		GuildID:      kallax.NewSchemaField("guild_id"),
		Name:         kallax.NewSchemaField("name"),
		GroupFK:      kallax.NewSchemaField("role_group_id"),
		Role:         kallax.NewSchemaField("role"),
		RequireRoles: kallax.NewSchemaField("require_roles"),
		IgnoreRoles:  kallax.NewSchemaField("ignore_roles"),
		Position:     kallax.NewSchemaField("position"),
	},
	RoleGroup: &schemaRoleGroup{
		BaseSchema: kallax.NewBaseSchema(
			"role_groups",
			"__rolegroup",
			kallax.NewSchemaField("id"),
			kallax.ForeignKeys{},
			func() kallax.Record {
				return new(RoleGroup)
			},
			true,
			kallax.NewSchemaField("id"),
			kallax.NewSchemaField("guild_id"),
			kallax.NewSchemaField("name"),
			kallax.NewSchemaField("require_roles"),
			kallax.NewSchemaField("ignore_roles"),
			kallax.NewSchemaField("mode"),
			kallax.NewSchemaField("multiple_max"),
			kallax.NewSchemaField("multiple_min"),
			kallax.NewSchemaField("single_auto_toggle_off"),
			kallax.NewSchemaField("single_require_one"),
		),
		ID:                  kallax.NewSchemaField("id"),
		GuildID:             kallax.NewSchemaField("guild_id"),
		Name:                kallax.NewSchemaField("name"),
		RequireRoles:        kallax.NewSchemaField("require_roles"),
		IgnoreRoles:         kallax.NewSchemaField("ignore_roles"),
		Mode:                kallax.NewSchemaField("mode"),
		MultipleMax:         kallax.NewSchemaField("multiple_max"),
		MultipleMin:         kallax.NewSchemaField("multiple_min"),
		SingleAutoToggleOff: kallax.NewSchemaField("single_auto_toggle_off"),
		SingleRequireOne:    kallax.NewSchemaField("single_require_one"),
	},
}
