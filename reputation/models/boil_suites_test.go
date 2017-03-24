package models

import "testing"

// This test suite runs each operation test in parallel.
// Example, if your database has 3 tables, the suite will run:
// table1, table2 and table3 Delete in parallel
// table1, table2 and table3 Insert in parallel, and so forth.
// It does NOT run each operation group in parallel.
// Separating the tests thusly grants avoidance of Postgres deadlocks.
func TestParent(t *testing.T) {
	t.Run("ReputationConfigs", testReputationConfigs)
	t.Run("ReputationUsers", testReputationUsers)
	t.Run("ReputationLogs", testReputationLogs)
}

func TestDelete(t *testing.T) {
	t.Run("ReputationConfigs", testReputationConfigsDelete)
	t.Run("ReputationUsers", testReputationUsersDelete)
	t.Run("ReputationLogs", testReputationLogsDelete)
}

func TestQueryDeleteAll(t *testing.T) {
	t.Run("ReputationConfigs", testReputationConfigsQueryDeleteAll)
	t.Run("ReputationUsers", testReputationUsersQueryDeleteAll)
	t.Run("ReputationLogs", testReputationLogsQueryDeleteAll)
}

func TestSliceDeleteAll(t *testing.T) {
	t.Run("ReputationConfigs", testReputationConfigsSliceDeleteAll)
	t.Run("ReputationUsers", testReputationUsersSliceDeleteAll)
	t.Run("ReputationLogs", testReputationLogsSliceDeleteAll)
}

func TestExists(t *testing.T) {
	t.Run("ReputationConfigs", testReputationConfigsExists)
	t.Run("ReputationUsers", testReputationUsersExists)
	t.Run("ReputationLogs", testReputationLogsExists)
}

func TestFind(t *testing.T) {
	t.Run("ReputationConfigs", testReputationConfigsFind)
	t.Run("ReputationUsers", testReputationUsersFind)
	t.Run("ReputationLogs", testReputationLogsFind)
}

func TestBind(t *testing.T) {
	t.Run("ReputationConfigs", testReputationConfigsBind)
	t.Run("ReputationUsers", testReputationUsersBind)
	t.Run("ReputationLogs", testReputationLogsBind)
}

func TestOne(t *testing.T) {
	t.Run("ReputationConfigs", testReputationConfigsOne)
	t.Run("ReputationUsers", testReputationUsersOne)
	t.Run("ReputationLogs", testReputationLogsOne)
}

func TestAll(t *testing.T) {
	t.Run("ReputationConfigs", testReputationConfigsAll)
	t.Run("ReputationUsers", testReputationUsersAll)
	t.Run("ReputationLogs", testReputationLogsAll)
}

func TestCount(t *testing.T) {
	t.Run("ReputationConfigs", testReputationConfigsCount)
	t.Run("ReputationUsers", testReputationUsersCount)
	t.Run("ReputationLogs", testReputationLogsCount)
}

func TestHooks(t *testing.T) {
	t.Run("ReputationConfigs", testReputationConfigsHooks)
	t.Run("ReputationUsers", testReputationUsersHooks)
	t.Run("ReputationLogs", testReputationLogsHooks)
}

func TestInsert(t *testing.T) {
	t.Run("ReputationConfigs", testReputationConfigsInsert)
	t.Run("ReputationConfigs", testReputationConfigsInsertWhitelist)
	t.Run("ReputationUsers", testReputationUsersInsert)
	t.Run("ReputationUsers", testReputationUsersInsertWhitelist)
	t.Run("ReputationLogs", testReputationLogsInsert)
	t.Run("ReputationLogs", testReputationLogsInsertWhitelist)
}

// TestToOne tests cannot be run in parallel
// or deadlocks can occur.
func TestToOne(t *testing.T) {}

// TestOneToOne tests cannot be run in parallel
// or deadlocks can occur.
func TestOneToOne(t *testing.T) {}

// TestToMany tests cannot be run in parallel
// or deadlocks can occur.
func TestToMany(t *testing.T) {}

// TestToOneSet tests cannot be run in parallel
// or deadlocks can occur.
func TestToOneSet(t *testing.T) {}

// TestToOneRemove tests cannot be run in parallel
// or deadlocks can occur.
func TestToOneRemove(t *testing.T) {}

// TestOneToOneSet tests cannot be run in parallel
// or deadlocks can occur.
func TestOneToOneSet(t *testing.T) {}

// TestOneToOneRemove tests cannot be run in parallel
// or deadlocks can occur.
func TestOneToOneRemove(t *testing.T) {}

// TestToManyAdd tests cannot be run in parallel
// or deadlocks can occur.
func TestToManyAdd(t *testing.T) {}

// TestToManySet tests cannot be run in parallel
// or deadlocks can occur.
func TestToManySet(t *testing.T) {}

// TestToManyRemove tests cannot be run in parallel
// or deadlocks can occur.
func TestToManyRemove(t *testing.T) {}

func TestReload(t *testing.T) {
	t.Run("ReputationConfigs", testReputationConfigsReload)
	t.Run("ReputationUsers", testReputationUsersReload)
	t.Run("ReputationLogs", testReputationLogsReload)
}

func TestReloadAll(t *testing.T) {
	t.Run("ReputationConfigs", testReputationConfigsReloadAll)
	t.Run("ReputationUsers", testReputationUsersReloadAll)
	t.Run("ReputationLogs", testReputationLogsReloadAll)
}

func TestSelect(t *testing.T) {
	t.Run("ReputationConfigs", testReputationConfigsSelect)
	t.Run("ReputationUsers", testReputationUsersSelect)
	t.Run("ReputationLogs", testReputationLogsSelect)
}

func TestUpdate(t *testing.T) {
	t.Run("ReputationConfigs", testReputationConfigsUpdate)
	t.Run("ReputationUsers", testReputationUsersUpdate)
	t.Run("ReputationLogs", testReputationLogsUpdate)
}

func TestSliceUpdateAll(t *testing.T) {
	t.Run("ReputationConfigs", testReputationConfigsSliceUpdateAll)
	t.Run("ReputationUsers", testReputationUsersSliceUpdateAll)
	t.Run("ReputationLogs", testReputationLogsSliceUpdateAll)
}

func TestUpsert(t *testing.T) {
	t.Run("ReputationConfigs", testReputationConfigsUpsert)
	t.Run("ReputationUsers", testReputationUsersUpsert)
	t.Run("ReputationLogs", testReputationLogsUpsert)
}
