package common

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

var (
	createTableRegex         = regexp.MustCompile(`(?i)create table if not exists ([0-9a-z_]*) *\(`)
	alterTableAddColumnRegex = regexp.MustCompile(`(?i)alter table ([0-9a-z_]*) add column if not exists ([0-9a-z_]*)`)
	addIndexRegex            = regexp.MustCompile(`(?i)create (unique )?index if not exists ([0-9a-z_]*) on ([0-9a-z_]*)`)
)

type DBSchema struct {
	Name    string
	Schemas []string
}

var schemasToInit = make([]*DBSchema, 0)

func RegisterDBSchemas(name string, schemas ...string) {
	schemasToInit = append(schemasToInit, &DBSchema{Name: name, Schemas: schemas})
}

func initQueuedSchemas() {
	for _, v := range schemasToInit {
		InitSchemas(v.Name, v.Schemas...)
	}
}

func initSchema(schema string, name string) {
	if confNoSchemaInit.GetBool() {
		return
	}

	skip, err := checkSkipSchemaInit(schema, name)
	if err != nil {
		logger.WithError(err).Error("Failed checking if we should skip schema: ", schema)
	}

	if skip {
		return
	}

	logger.Info("Schema initialization: ", name, ": not skipped")
	// if strings.HasPrefix("create table if not exists", trimmedLower) {

	// }else if strings.HasPrefix("alter table", prefix)

	_, err = PQ.Exec(schema)
	if err != nil {
		UnlockRedisKey("schema_init")
		logger.WithError(err).Fatal("failed initializing postgres db schema for ", name)
	}

	return
}

func checkSkipSchemaInit(schema string, name string) (exists bool, err error) {
	trimmed := strings.TrimSpace(schema)

	if matches := createTableRegex.FindAllStringSubmatch(trimmed, -1); len(matches) > 0 {
		return TableExists(matches[0][1])
	}

	if matches := addIndexRegex.FindAllStringSubmatch(trimmed, -1); len(matches) > 0 {
		return checkIndexExists(matches[0][3], matches[0][2])
	}

	if matches := alterTableAddColumnRegex.FindAllStringSubmatch(trimmed, -1); len(matches) > 0 {
		return checkColumnExists(matches[0][1], matches[0][2])
	}

	return false, nil
}

func TableExists(table string) (b bool, err error) {
	const query = `	
SELECT EXISTS 
(
	SELECT 1
	FROM information_schema.tables 
	WHERE table_schema = 'public'
	AND table_name = $1
);`

	err = PQ.QueryRow(query, table).Scan(&b)
	return b, err
}

func checkIndexExists(table, index string) (b bool, err error) {
	const query = `	
SELECT EXISTS 
(
	SELECT 1
FROM
    pg_class t,
    pg_class i,
    pg_index ix,
    pg_attribute a
WHERE
    t.oid = ix.indrelid
    AND i.oid = ix.indexrelid
    AND a.attrelid = t.oid
    AND a.attnum = ANY(ix.indkey)
    AND t.relkind = 'r'
    AND t.relname = $1
    AND i.relname = $2
);`

	err = PQ.QueryRow(query, table, index).Scan(&b)
	return b, err
}

func checkColumnExists(table, column string) (b bool, err error) {
	const query = `	
SELECT EXISTS 
(
SELECT 1 
FROM information_schema.columns 
WHERE table_name=$1 and column_name=$2
);`

	err = PQ.QueryRow(query, table, column).Scan(&b)
	return b, err
}

func InitSchemas(name string, schemas ...string) {
	if err := BlockingLockRedisKey("schema_init", time.Minute*10, 60*60); err != nil {
		panic(err)
	}

	defer UnlockRedisKey("schema_init")

	for i, v := range schemas {
		actualName := fmt.Sprintf("%s[%d]", name, i)
		initSchema(v, actualName)
	}

	return
}
