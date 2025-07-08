package opencloud

import (
	"context"
	"fmt"
	"net/http"
)

// DataAndMemoryStoreService will handle communciation with the actions related to the API.
//
// Roblox Open Cloud API Docs: https://create.roblox.com/docs/en-us/cloud
type DataAndMemoryStoreService service

type DataStore struct {
	Path       string `json:"path"`
	CreateTime string `json:"createTime"`
	ID         string `json:"id"`
}

type DataStoreList struct {
	DataStores    []DataStore `json:"dataStores"`
	NextPageToken string      `json:"nextPageToken"`
}

// ListDataStores will fetch a list of data stores for a specific universe.
//
// Required scopes: universe-datastores.control:list
//
// Roblox Opencloud API Docs: https://create.roblox.com/docs/en-us/cloud/reference/DataStore#Cloud_ListDataStores
//
// [GET] /cloud/v2/universes/{universe_id}/data-stores
func (s *DataAndMemoryStoreService) ListDataStores(ctx context.Context, universeId string, opts *OptionsWithFilter) (*DataStoreList, *Response, error) {
	u := fmt.Sprintf("/cloud/v2/universes/%s/data-stores", universeId)

	u, err := addOpts(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, nil, err
	}

	dataStoreList := new(DataStoreList)
	resp, err := s.client.Do(ctx, req, dataStoreList)
	if err != nil {
		return nil, resp, err
	}

	return dataStoreList, resp, nil
}

type DataStoreSnapshot struct {
	NewSnapshotTaken   bool   `json:"newSnapshotTaken"`
	LatestSnapshotTime string `json:"latestSnapshotTime"`
}

// SnapshotDataStores will take a snapshot of all data stores for a specific universe.
// After the snapshot is taken, the next write to every key in the experience creaets a versioned backup of data.
//
// Required scopes:
//
// Roblox Opencloud API Docs: https://create.roblox.com/docs/en-us/cloud/reference/DataStore#Cloud_SnapshotDataStores
//
// [POST] /cloud/v2/universes/{universe_id}/data-stores:snapshot
func (s *DataAndMemoryStoreService) SnapshotDataStores(ctx context.Context, universeId string) (*DataStoreSnapshot, *Response, error) {
	u := fmt.Sprintf("/cloud/v2/universes/%s/data-stores:snapshot", universeId)

	req, err := s.client.NewRequest(http.MethodPost, u, nil)
	if err != nil {
		return nil, nil, err
	}

	dataStoreSnapshot := new(DataStoreSnapshot)
	resp, err := s.client.Do(ctx, req, dataStoreSnapshot)
	if err != nil {
		return nil, resp, err
	}

	return dataStoreSnapshot, resp, nil
}

type DataStoreEntryState string

const (
	DataStoreEntryStateUnspecified DataStoreEntryState = "STATE_UNSPECIFIED"
	DataStoreEntryStateActive      DataStoreEntryState = "ACTIVE"
	DataStoreEntryStateDeleted     DataStoreEntryState = "DELETED"
)

type DataStoreEntry struct {
	Path                 string              `json:"path"`
	CreateTime           string              `json:"createTime"`
	RevisionID           string              `json:"revisionId"`
	RevisionCreationTime string              `json:"revisionCreationTime"`
	State                DataStoreEntryState `json:"state"`
	Etag                 string              `json:"etag"`
	Value                any                 `json:"value"`
	ID                   string              `json:"id"`
	User                 string              `json:"user"`
}

type DataStoreEntriesList struct {
	DataStoreEntries []DataStoreEntry `json:"dataStoreEntries"`
	NextPageToken    string           `json:"nextPageToken"`
}

type ListDataStoreEntriesOptions struct {
	OptionsWithFilter
	ShowDeleted *bool `url:"showDeleted,omitempty"`
}

// ListDataStoreEntries will fetch a list of entries for a specific data store under a specific universe.
//
// Required scopes: universe-datastores.objects:list
//
// Roblox Opencloud API Docs: https://create.roblox.com/docs/en-us/cloud/reference/DataStoreEntry#Cloud_ListDataStoreEntries
//
// [GET] /cloud/v2/universes/{universe_id}/data-stores/{data_store_id}/entries
//
// [GET] /cloud/v2/universes/{universe_id}/data-stores/{data_store_id}/scopes/{scope_id}/entries
func (s *DataAndMemoryStoreService) ListDataStoreEntries(ctx context.Context, universeId, dataStoreId string, scope *string, opts *ListDataStoreEntriesOptions) (*DataStoreEntriesList, *Response, error) {
	u := fmt.Sprintf("/cloud/v2/universes/%s/data-stores/%s", universeId, dataStoreId)
	if scope != nil {
		u += fmt.Sprintf("/scopes/%s", *scope)
	}
	u += "/entries"

	u, err := addOpts(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, nil, err
	}

	dataStoreEntriesList := new(DataStoreEntriesList)
	resp, err := s.client.Do(ctx, req, dataStoreEntriesList)
	if err != nil {
		return nil, resp, err
	}

	return dataStoreEntriesList, resp, nil
}

type DataStoreEntryCreate struct {
	Etag       *string         `json:"etag,omitempty"`
	Value      *any            `json:"value,omitempty"`
	Users      *[]string       `json:"users,omitempty"`
	Attributes *map[string]any `json:"attributes,omitempty"`
}

type DataStoreEntryCreateOptions struct {
	ID *string `url:"id,omitempty"`
}

// CreateDataStoreEntry will create a new entry for the specified datastore under a specific universe.
//
// Required scopes: universe-datastores.objects:create
//
// Roblox Opencloud API Docs: https://create.roblox.com/docs/en-us/cloud/reference/DataStoreEntry#Cloud_CreateDataStoreEntry
//
// [POST] /cloud/v2/universes/{universe_id}/data-stores/{data_store_id}/entries
//
// [POST] /cloud/v2/universes/{universe_id}/data-stores/{data_store_id}/scopes/{scope_id}/entries
func (s *DataAndMemoryStoreService) CreateDataStoreEntry(ctx context.Context, universeId, dataStoreId string, scope *string, data DataStoreEntryCreate, opts *DataStoreEntryCreateOptions) (*DataStoreEntry, *Response, error) {
	u := fmt.Sprintf("/cloud/v2/universes/%s/data-stores/%s", universeId, dataStoreId)
	if scope != nil {
		u += fmt.Sprintf("/scopes/%s", *scope)
	}
	u += "/entries"

	u, err := addOpts(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(http.MethodPost, u, data)
	if err != nil {
		return nil, nil, err
	}

	dataStoreEntry := new(DataStoreEntry)
	resp, err := s.client.Do(ctx, req, dataStoreEntry)
	if err != nil {
		return nil, resp, err
	}

	return dataStoreEntry, resp, nil
}

// GetDataStoreEntry will fetch a specified entry for the specified datastore under a specific universe.
//
// Required scopes: universe-datastores.objects:read
//
// Roblox Opencloud API Docs: https://create.roblox.com/docs/en-us/cloud/reference/DataStoreEntry#Cloud_GetDataStoreEntry
//
// [GET] /cloud/v2/universes/{universe_id}/data-stores/{data_store_id}/entries/{entry_id}
//
// [GET] /cloud/v2/universes/{universe_id}/data-stores/{data_store_id}/scopes/{scope_id}/entries/{entry_id}
func (s *DataAndMemoryStoreService) GetDataStoreEntry(ctx context.Context, universeId, datastoreId string, scope *string, entryId string) (*DataStoreEntry, *Response, error) {
	u := fmt.Sprintf("/cloud/v2/universes/%s/data-stores/%s", universeId, datastoreId)
	if scope != nil {
		u += fmt.Sprintf("/scopes/%s", *scope)
	}
	u += fmt.Sprintf("/entries/%s", entryId)

	req, err := s.client.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, nil, err
	}

	dataStoreEntry := new(DataStoreEntry)
	resp, err := s.client.Do(ctx, req, dataStoreEntry)
	if err != nil {
		return nil, resp, err
	}

	return dataStoreEntry, resp, nil
}

// DeleteDataStoreEntry will delete a specified entry for the specified datastore under a specific universe.
//
// Required scopes: universe-datastores.objects:delete
//
// Roblox Opencloud API Docs: https://create.roblox.com/docs/en-us/cloud/reference/DataStoreEntry#Cloud_DeleteDataStoreEntry
//
// [DELETE] /cloud/v2/universes/{universe_id}/data-stores/{data_store_id}/entries/{entry_id}
//
// [DELETE] /cloud/v2/universes/{universe_id}/data-stores/{data_store_id}/scopes/{scope_id}/entries/{entry_id}
func (s *DataAndMemoryStoreService) DeleteDataStoreEntry(ctx context.Context, universeId, datastoreId string, scope *string, entryId string) (*Response, error) {
	u := fmt.Sprintf("/cloud/v2/universes/%s/data-stores/%s", universeId, datastoreId)
	if scope != nil {
		u += fmt.Sprintf("/scopes/%s", *scope)
	}
	u += fmt.Sprintf("/entries/%s", entryId)

	req, err := s.client.NewRequest(http.MethodDelete, u, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(ctx, req, nil)
	if err != nil {
		return resp, err
	}

	return resp, nil
}

type DataStoreEntryUpdate struct {
	Etag       *string         `json:"etag,omitempty"`
	Value      *any            `json:"value,omitempty"`
	Users      *[]string       `json:"users,omitempty"`
	Attributes *map[string]any `json:"attributes,omitempty"`
}

type DataStoreEntryUpdateOpts struct {
	AllowMissing *bool `url:"allowMissing,omitempty"`
}

// UpdateDataStoreEntry will update a specified entry for the specified datastore under a specific universe.
//
// Required scopes: universe-datastores.objects:update
//
// Roblox Opencloud API Docs: https://create.roblox.com/docs/en-us/cloud/reference/DataStoreEntry#Cloud_UpdateDataStoreEntry
//
// [PATCH] /cloud/v2/universes/{universe_id}/data-stores/{data_store_id}/entries/{entry_id}
//
// [PATCH] /cloud/v2/universes/{universe_id}/data-stores/{data_store_id}/scopes/{scope_id}/entries/{entry_id}
func (s *DataAndMemoryStoreService) UpdateDataStoreEntry(ctx context.Context, universeId, datastoreId string, scope *string, entryId string, data DataStoreEntryUpdate, opts *DataStoreEntryUpdateOpts) (*DataStoreEntry, *Response, error) {
	u := fmt.Sprintf("/cloud/v2/universes/%s/data-stores/%s", universeId, datastoreId)
	if scope != nil {
		u += fmt.Sprintf("/scopes/%s", *scope)
	}
	u += fmt.Sprintf("/entries/%s", entryId)

	u, err := addOpts(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(http.MethodPatch, u, data)
	if err != nil {
		return nil, nil, err
	}

	dataStoreEntry := new(DataStoreEntry)
	resp, err := s.client.Do(ctx, req, dataStoreEntry)
	if err != nil {
		return nil, resp, err
	}

	return dataStoreEntry, resp, nil
}

type DataStoreEntryIncrement struct {
	Amount     *int            `json:"amount,omitempty"`
	Users      *[]string       `json:"users,omitempty"`
	Attributes *map[string]any `json:"attributes,omitempty"`
}

// IncrementDataStoreEntry will increment the value for a specified entry for the specified datastore under a specific universe.
//
// Required scopes:
//
// Roblox Opencloud API Docs: https://create.roblox.com/docs/en-us/cloud/reference/DataStoreEntry#Cloud_IncrementDataStoreEntry
//
// [POST] /cloud/v2/universes/{universe_id}/data-stores/{data_store_id}/entries/{entry_id}:increment
//
// [POST] /cloud/v2/universes/{universe_id}/data-stores/{data_store_id}/scopes/{scope_id}/entries/{entry_id}:increment
func (s *DataAndMemoryStoreService) IncrementDataStoreEntry(ctx context.Context, universeId, datastoreId string, scope *string, entryId string, data DataStoreEntryIncrement) (*DataStoreEntry, *Response, error) {
	u := fmt.Sprintf("/cloud/v2/universes/%s/data-stores/%s", universeId, datastoreId)
	if scope != nil {
		u += fmt.Sprintf("/scopes/%s", *scope)
	}
	u += fmt.Sprintf("/entries/%s:increment", entryId)

	req, err := s.client.NewRequest(http.MethodPost, u, data)
	if err != nil {
		return nil, nil, err
	}

	dataStoreEntry := new(DataStoreEntry)
	resp, err := s.client.Do(ctx, req, dataStoreEntry)
	if err != nil {
		return nil, resp, err
	}

	return dataStoreEntry, resp, nil
}

type DataStoryEntryRevisionsList struct {
	DataStoreEntries []DataStoreEntry `json:"dataStoreEntries"`
	NextPageToken    string           `json:"nextPageToken"`
}

// ListDataStoreEntryRevisions will fetch a list of revisions for a specific entry for the specified datastore under a specific universe.
//
// Required scopes: universe-datastores.versions:list
//
// Roblox Opencloud API Docs: https://create.roblox.com/docs/en-us/cloud/reference/DataStoreEntry#Cloud_ListDataStoreEntryRevisions
//
// [GET] /cloud/v2/universes/{universe_id}/data-stores/{data_store_id}/entries/{entry_id}:listRevisions
//
// [GET] /cloud/v2/universes/{universe_id}/data-stores/{data_store_id}/scopes/{scope_id}/entries/{entry_id}:listRevisions
func (s *DataAndMemoryStoreService) ListDataStoreEntryRevisions(ctx context.Context, universeId, datastoreId string, scope *string, entryId string, opts *Options) (*DataStoryEntryRevisionsList, *Response, error) {
	u := fmt.Sprintf("/cloud/v2/universes/%s/data-stores/%s", universeId, datastoreId)
	if scope != nil {
		u += fmt.Sprintf("/scopes/%s", *scope)
	}
	u += fmt.Sprintf("/entries/%s:listRevisions", entryId)

	u, err := addOpts(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, nil, err
	}

	dataStoreEntryRevisionsList := new(DataStoryEntryRevisionsList)
	resp, err := s.client.Do(ctx, req, dataStoreEntryRevisionsList)
	if err != nil {
		return nil, resp, err
	}

	return dataStoreEntryRevisionsList, resp, nil
}

// FlushMemoryStore will asynchronously flush the memory store for a specific universe.
//
// Required scopes: universe.memory-store:flush
//
// Roblox Opencloud API Docs: https://create.roblox.com/docs/en-us/cloud/reference/MemoryStore#Cloud_FlushMemoryStore
//
// [POST] /cloud/v2/universes/{universe_id}/memory-store:flush
func (s *DataAndMemoryStoreService) FlushMemoryStore(ctx context.Context, universeId string) (*Operation, *Response, error) {
	u := fmt.Sprintf("/cloud/v2/universes/%s/memory-store:flush", universeId)

	req, err := s.client.NewRequest(http.MethodPost, u, nil)
	if err != nil {
		return nil, nil, err
	}

	operation := new(Operation)
	resp, err := s.client.Do(ctx, req, operation)
	if err != nil {
		return nil, resp, err
	}

	return operation, resp, nil
}

type MemoryStoreQueueItem struct {
	Path       string `json:"path"`
	Data       any    `json:"data"`
	Priority   int    `json:"priority"`
	ExpireTime string `json:"expireTime"`
	ID         string `json:"id"`
}

type MemoryStoreQueueItemCreate struct {
	Data     *any    `json:"data,omitempty"`
	Priority *int    `json:"priority,omitempty"`
	TTL      *string `json:"ttl,omitempty"`
}

// CreateMemoryStoreQueueItem will create a new item in the memory store queue for a specific universe.
//
// Required scopes: universe.memory-store.queue:write
//
// Roblox Opencloud API Docs: https://create.roblox.com/docs/en-us/cloud/reference/MemoryStoreQueueItem#Cloud_CreateMemoryStoreQueueItem
//
// [POST] /cloud/v2/universes/{universe_id}/memory-store/queues/{queue_id}/items
func (s *DataAndMemoryStoreService) CreateMemoryStoreQueueItem(ctx context.Context, universeId, queueId string, data MemoryStoreQueueItemCreate) (*MemoryStoreQueueItem, *Response, error) {
	u := fmt.Sprintf("/cloud/v2/universes/%s/memory-store/queues/%s/items", universeId, queueId)

	req, err := s.client.NewRequest(http.MethodPost, u, data)
	if err != nil {
		return nil, nil, err
	}

	memoryStoreQueueItem := new(MemoryStoreQueueItem)
	resp, err := s.client.Do(ctx, req, memoryStoreQueueItem)
	if err != nil {
		return nil, resp, err
	}

	return memoryStoreQueueItem, resp, nil
}

type MemoryStoreQueueItemsDiscard struct {
	ReadID string `json:"readId"`
}

// DiscardMemoryStoreQueueItems will discard a specific item(s) in the memory store queue for a specific universe.
//
// Required scopes: universe.memory-store.queue:discard
//
// Roblox Opencloud API Docs: https://create.roblox.com/docs/en-us/cloud/reference/MemoryStoreQueueItem#Cloud_DiscardMemoryStoreQueueItems
//
// [POST] /cloud/v2/universes/{universe_id}/memory-store/queues/{queue_id}/items:discard
func (s *DataAndMemoryStoreService) DiscardMemoryStoreQueueItems(ctx context.Context, universeId, queueId string, data MemoryStoreQueueItemsDiscard) (*Response, error) {
	u := fmt.Sprintf("/cloud/v2/universes/%s/memory-store/queues/%s/items:discard", universeId, queueId)

	req, err := s.client.NewRequest(http.MethodPost, u, data)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(ctx, req, nil)
	if err != nil {
		return resp, err
	}

	return resp, nil
}

type MemoryStoreQueueItems struct {
	ReadID string                 `json:"readId"`
	Items  []MemoryStoreQueueItem `json:"items"`
}

type MemoryStoreQueueItemsOptions struct {
	Count              *int    `url:"count,omitempty"`
	AllOrNothing       *bool   `url:"allOrNothing,omitempty"`
	InvisibilityWindow *string `url:"invisibilityWindow,omitempty"`
}

// ReadMemoryStoreQueueItems will fetch specific item(s) in the memory store queue for a specific universe.
//
// Required scopes: universe.memory-store.queue:dequeue
//
// Roblox Opencloud API Docs: https://create.roblox.com/docs/en-us/cloud/reference/MemoryStoreQueueItem#Cloud_ReadMemoryStoreQueueItems
//
// [GET] /cloud/v2/universes/{universe_id}/memory-store/queues/{queue_id}/items:read
func (s *DataAndMemoryStoreService) ReadMemoryStoreQueueItems(ctx context.Context, universeId, queueId string, opts *MemoryStoreQueueItemsOptions) (*MemoryStoreQueueItems, *Response, error) {
	u := fmt.Sprintf("/cloud/v2/universes/%s/memory-store/queues/%s/items:read", universeId, queueId)

	u, err := addOpts(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, nil, err
	}

	memoryStoreQueueItems := new(MemoryStoreQueueItems)
	resp, err := s.client.Do(ctx, req, memoryStoreQueueItems)
	if err != nil {
		return nil, resp, err
	}

	return memoryStoreQueueItems, resp, nil
}

type MemoryStoreSortedMapItem struct {
	Path           string  `json:"path"`
	Value          any     `json:"value"`
	Etag           string  `json:"etag"`
	ID             string  `json:"id"`
	StringSortKey  *string `json:"stringSortKey,omitempty"`
	NumericSortKey *int    `json:"numericSortKey,omitempty"`
}

type MemoryStoreSortedMapList struct {
	MemoryStoreSortedMapItems []MemoryStoreSortedMapItem `json:"memoryStoreSortedMapItems"`
	NextPageToken             string                     `json:"nextPageToken"`
}

type MemoryStoreSortedMapItemListOptions struct {
	OptionsWithFilter
	OrderBy *string `url:"orderBy,omitempty"`
}

// ListMemoryStoreSortedMapItems will fetch a list of memory store map items for a specific sorted map under a specific universe.
//
// Required scopes: universe.memory-store.sorted-map:read
//
// Roblox Opencloud API Docs: https://create.roblox.com/docs/en-us/cloud/reference/MemoryStoreSortedMapItem#Cloud_ListMemoryStoreSortedMapItems
//
// [GET] /cloud/v2/universes/{universe_id}/memory-store/sorted-maps/{sorted_map_id}/items
func (s *DataAndMemoryStoreService) ListMemoryStoreSortedMapItems(ctx context.Context, universeId, sortedMapId string, opts *MemoryStoreSortedMapItemListOptions) (*MemoryStoreSortedMapList, *Response, error) {
	u := fmt.Sprintf("/cloud/v2/universes/%s/memory-store/sorted-maps/%s/items", universeId, sortedMapId)

	u, err := addOpts(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, nil, err
	}

	memoryStoreSortedMapList := new(MemoryStoreSortedMapList)
	resp, err := s.client.Do(ctx, req, memoryStoreSortedMapList)
	if err != nil {
		return nil, resp, err
	}

	return memoryStoreSortedMapList, resp, nil
}

type MemoryStoreSortedMapItemCreate struct {
	Value         *any    `json:"value,omitempty"`
	TTL           *string `json:"ttl,omitempty"`
	ID            *string `json:"id,omitempty"`
	StringSortKey *string `json:"stringSortKey,omitempty"`
}

type MemoryStoreSortedMapItemCreateOptions struct {
	ID string `url:"id,omitempty"`
}

// CreateMemoryStoreSortedMapItem will create a new item in the memory store sorted map for a specific universe.
//
// Required scopes: universe.memory-store.sorted-map:write
//
// Roblox Opencloud API Docs: https://create.roblox.com/docs/en-us/cloud/reference/MemoryStoreSortedMapItem#Cloud_CreateMemoryStoreSortedMapItem
//
// [POST] /cloud/v2/universes/{universe_id}/memory-store/sorted-maps/{sorted_map_id}/items
func (s *DataAndMemoryStoreService) CreateMemoryStoreSortedMapItem(ctx context.Context, universeId, sortedMapId string, data MemoryStoreSortedMapItemCreate, opts *MemoryStoreSortedMapItemCreateOptions) (*MemoryStoreSortedMapItem, *Response, error) {
	u := fmt.Sprintf("/cloud/v2/universes/%s/memory-store/sorted-maps/%s/items", universeId, sortedMapId)

	u, err := addOpts(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(http.MethodPost, u, data)
	if err != nil {
		return nil, nil, err
	}

	memoryStoreSortedMapItem := new(MemoryStoreSortedMapItem)
	resp, err := s.client.Do(ctx, req, memoryStoreSortedMapItem)
	if err != nil {
		return nil, resp, err
	}

	return memoryStoreSortedMapItem, resp, nil
}

// GetMemoryStoreSortedMapItem will fetch a specific item in the memory store sorted map for a specific universe.
//
// Required scopes: universe.memory-store.sorted-map:read
//
// Roblox Opencloud API Docs: https://create.roblox.com/docs/en-us/cloud/reference/MemoryStoreSortedMapItem#Cloud_GetMemoryStoreSortedMapItem
//
// [GET] /cloud/v2/universes/{universe_id}/memory-store/sorted-maps/{sorted_map_id}/items/{item_id}
func (s *DataAndMemoryStoreService) GetMemoryStoreSortedMapItem(ctx context.Context, universeId, sortedMapId, itemId string) (*MemoryStoreSortedMapItem, *Response, error) {
	u := fmt.Sprintf("/cloud/v2/universes/%s/memory-store/sorted-maps/%s/items/%s", universeId, sortedMapId, itemId)

	req, err := s.client.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, nil, err
	}

	memoryStoreSortedMapItem := new(MemoryStoreSortedMapItem)
	resp, err := s.client.Do(ctx, req, memoryStoreSortedMapItem)
	if err != nil {
		return nil, resp, err
	}

	return memoryStoreSortedMapItem, resp, nil
}

// DeleteMemoryStoreSortedMapItem will delete a specific item in the memory store sorted map for a specific universe.
//
// Required scopes: universe.memory-store.sorted-map:write
//
// Roblox Opencloud API Docs: https://create.roblox.com/docs/en-us/cloud/reference/MemoryStoreSortedMapItem#Cloud_DeleteMemoryStoreSortedMapItem
//
// [DELETE] /cloud/v2/universes/{universe_id}/memory-store/sorted-maps/{sorted_map_id}/items/{item_id}
func (s *DataAndMemoryStoreService) DeleteMemoryStoreSortedMapItem(ctx context.Context, universeId, sortedMapId, itemId string) (*Response, error) {
	u := fmt.Sprintf("/cloud/v2/universes/%s/memory-store/sorted-maps/%s/items/%s", universeId, sortedMapId, itemId)

	req, err := s.client.NewRequest(http.MethodDelete, u, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(ctx, req, nil)
	if err != nil {
		return resp, err
	}

	return resp, nil
}

type MemoryStoreSortedMapItemUpdate struct {
	Value         *any    `json:"value,omitempty"`
	TTL           *string `json:"ttl,omitempty"`
	ID            *string `json:"id,omitempty"`
	StringSortKey *string `json:"stringSortKey,omitempty"`
}

type MemoryStoreSortedMapItemUpdateOpts struct {
	ID string `url:"id,omitempty"`
}

// UpdateMemoryStoreSortedMapItem will update a specific item in the memory store sorted map for a specific universe.
//
// Required scopes: universe.memory-store.sorted-map:write
//
// Roblox Opencloud API Docs: https://create.roblox.com/docs/en-us/cloud/reference/MemoryStoreSortedMapItem#Cloud_UpdateMemoryStoreSortedMapItem
//
// [PATCH] /cloud/v2/universes/{universe_id}/memory-store/sorted-maps/{sorted_map_id}/items/{item_id}
func (s *DataAndMemoryStoreService) UpdateMemoryStoreSortedMapItem(ctx context.Context, universeId, sortedMapId, itemId string, data MemoryStoreSortedMapItemUpdate, opts *MemoryStoreSortedMapItemUpdateOpts) (*MemoryStoreSortedMapItem, *Response, error) {
	u := fmt.Sprintf("/cloud/v2/universes/%s/memory-store/sorted-maps/%s/items/%s", universeId, sortedMapId, itemId)

	u, err := addOpts(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(http.MethodPatch, u, data)
	if err != nil {
		return nil, nil, err
	}

	memoryStoreSortedMapItem := new(MemoryStoreSortedMapItem)
	resp, err := s.client.Do(ctx, req, memoryStoreSortedMapItem)
	if err != nil {
		return nil, resp, err
	}

	return memoryStoreSortedMapItem, resp, nil
}

type OrderedDataStoreEntry struct {
	Path  string `json:"path"`
	Value int    `json:"value"`
	ID    string `json:"id"`
}

type OrderedDataStoreEntryList struct {
	OrderedDataStoreEntries []OrderedDataStoreEntry `json:"orderedDataStoreEntries"`
	NextPageToken           string                  `json:"nextPageToken"`
}

// -
//
// Required scopes: universe.ordered-data-store.scope.entry:read
//
// Roblox Opencloud API Docs: https://create.roblox.com/docs/en-us/cloud/reference/OrderedDataStoreEntry#Cloud_ListOrderedDataStoreEntries
//
// [GET] /cloud/v2/universes/{universe_id}/ordered-data-stores/{ordered_data_store_id}/scopes/{scope_id}/entries
func (s *DataAndMemoryStoreService) ListOrderedDataStoreEntries(ctx context.Context, universeId, orderedDataStoreId, scopeId string, opts *Options) (*OrderedDataStoreEntryList, *Response, error) {
	u := fmt.Sprintf("/cloud/v2/universes/%s/ordered-data-stores/%s/scopes/%s/entries", universeId, orderedDataStoreId, scopeId)

	u, err := addOpts(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, nil, err
	}

	orderedDataStoreEntryList := new(OrderedDataStoreEntryList)
	resp, err := s.client.Do(ctx, req, orderedDataStoreEntryList)
	if err != nil {
		return nil, resp, err
	}

	return orderedDataStoreEntryList, resp, nil
}

type OrderedDataStoreEntryCreate struct {
	Value *int `json:"value,omitempty"`
}

type OrderedDataStoreEntryCreateOptions struct {
	ID *string `url:"id,omitempty"`
}

// -
//
// Required scopes: universe.ordered-data-store.scope.entry:write
//
// Roblox Opencloud API Docs:https://create.roblox.com/docs/en-us/cloud/reference/OrderedDataStoreEntry#Cloud_CreateOrderedDataStoreEntry
//
// [POST] /cloud/v2/universes/{universe_id}/ordered-data-stores/{ordered_data_store_id}/scopes/{scope_id}/entries
func (s *DataAndMemoryStoreService) CreateOrderedDataStoreEntry(ctx context.Context, universeId, orderedDataStoreId, scopeId string, data OrderedDataStoreEntryCreate, opts *OrderedDataStoreEntryCreateOptions) (*OrderedDataStoreEntry, *Response, error) {
	u := fmt.Sprintf("/cloud/v2/universes/%s/ordered-data-stores/%s/scopes/%s/entries", universeId, orderedDataStoreId, scopeId)

	u, err := addOpts(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(http.MethodPost, u, data)
	if err != nil {
		return nil, nil, err
	}

	orderedDataStoreEntry := new(OrderedDataStoreEntry)
	resp, err := s.client.Do(ctx, req, orderedDataStoreEntry)
	if err != nil {
		return nil, resp, err
	}

	return orderedDataStoreEntry, resp, nil
}

// -
//
// Required scopes: universe.ordered-data-store.scope.entry:read
//
// Roblox Opencloud API Docs: https://create.roblox.com/docs/en-us/cloud/reference/OrderedDataStoreEntry#Cloud_GetOrderedDataStoreEntry
//
// [GET] /cloud/v2/universes/{universe_id}/ordered-data-stores/{ordered_data_store_id}/scopes/{scope_id}/entries/{entry_id}
func (s *DataAndMemoryStoreService) GetOrderedDataStoreEntry(ctx context.Context, universeId, orderedDataStoreId, scopeId, entryId string) (*OrderedDataStoreEntry, *Response, error) {
	u := fmt.Sprintf("/cloud/v2/universes/%s/ordered-data-stores/%s/scopes/%s/entries/%s", universeId, orderedDataStoreId, scopeId, entryId)

	req, err := s.client.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, nil, err
	}

	orderedDataStoreEntry := new(OrderedDataStoreEntry)
	resp, err := s.client.Do(ctx, req, orderedDataStoreEntry)
	if err != nil {
		return nil, resp, err
	}

	return orderedDataStoreEntry, resp, nil
}

// -
//
// Required scopes: universe.ordered-data-store.scope.entry:write
//
// Roblox Opencloud API Docs: https://create.roblox.com/docs/en-us/cloud/reference/OrderedDataStoreEntry#Cloud_DeleteOrderedDataStoreEntry
//
// [DELETE] /cloud/v2/universes/{universe_id}/ordered-data-stores/{ordered_data_store_id}/scopes/{scope_id}/entries/{entry_id}
func (s *DataAndMemoryStoreService) DeleteOrderedDataStoreEntry(ctx context.Context, universeId, orderedDataStoreId, scopeId, entryId string) (*Response, error) {
	u := fmt.Sprintf("/cloud/v2/universes/%s/ordered-data-stores/%s/scopes/%s/entries/%s", universeId, orderedDataStoreId, scopeId, entryId)

	req, err := s.client.NewRequest(http.MethodDelete, u, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(ctx, req, nil)
	if err != nil {
		return resp, err
	}

	return resp, nil
}

type OrderedDataStoreEntryUpdate struct {
	Value *int `json:"value,omitempty"`
}

type OrderedDataStoreEntryUpdateOpts struct {
	AllowMissing *bool `url:"allowMissing,omitempty"`
}

// -
//
// Required scopes: universe.ordered-data-store.scope.entry:write
//
// Roblox Opencloud API Docs: https://create.roblox.com/docs/en-us/cloud/reference/OrderedDataStoreEntry#Cloud_UpdateOrderedDataStoreEntry
//
// [PATCH] /cloud/v2/universes/{universe_id}/ordered-data-stores/{ordered_data_store_id}/scopes/{scope_id}/entries/{entry_id}
func (s *DataAndMemoryStoreService) UpdateOrderedDataStoreEntry(ctx context.Context, universeId, orderedDataStoreId, scopeId, entryId string, data OrderedDataStoreEntryUpdate, opts *OrderedDataStoreEntryUpdateOpts) (*OrderedDataStoreEntry, *Response, error) {
	u := fmt.Sprintf("/cloud/v2/universes/%s/ordered-data-stores/%s/scopes/%s/entries/%s", universeId, orderedDataStoreId, scopeId, entryId)

	u, err := addOpts(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(http.MethodPatch, u, data)
	if err != nil {
		return nil, nil, err
	}

	orderedDataStoreEntry := new(OrderedDataStoreEntry)
	resp, err := s.client.Do(ctx, req, orderedDataStoreEntry)
	if err != nil {
		return nil, resp, err
	}

	return orderedDataStoreEntry, resp, nil
}

type OrderedDataStoreEntryIncrement struct {
	Amount *int `json:"amount,omitempty"`
}

// -
//
// Required scopes: universe.ordered-data-store.scope.entry:write
//
// Roblox Opencloud API Docs: https://create.roblox.com/docs/en-us/cloud/reference/OrderedDataStoreEntry#Cloud_IncrementOrderedDataStoreEntry
//
// [POST] /cloud/v2/universes/{universe_id}/ordered-data-stores/{ordered_data_store_id}/scopes/{scope_id}/entries/{entry_id}:increment
func (s *DataAndMemoryStoreService) IncrementOrderedDataStoreEntry(ctx context.Context, universeId, orderedDataStoreId, scopeId, entryId string, data OrderedDataStoreEntryIncrement) (*OrderedDataStoreEntry, *Response, error) {
	u := fmt.Sprintf("/cloud/v2/universes/%s/ordered-data-stores/%s/scopes/%s/entries/%s:increment", universeId, orderedDataStoreId, scopeId, entryId)

	req, err := s.client.NewRequest(http.MethodPost, u, data)
	if err != nil {
		return nil, nil, err
	}

	orderedDataStoreEntry := new(OrderedDataStoreEntry)
	resp, err := s.client.Do(ctx, req, orderedDataStoreEntry)
	if err != nil {
		return nil, resp, err
	}

	return orderedDataStoreEntry, resp, nil
}
