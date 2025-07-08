# Luau Execution
These endpoints let you execute Luau code in one of your experience(s).

::: danger
Scripts can run engine methods regardless of the scopes granted. For example, it can run datastore related methods. Proceed with caution when granting any luau-execution scopes.
:::

## Variables
### `LuauExecutionTaskPathRegex`
This regex is used to parse the URL of a path to grab the `UniverseID`, `PlaceID`, `VersionID`, `SessionID`, and `TaskID`. It is used by the [`LuauExecutionTask.TaskInfo()`](#taskinfo) method.
```go
regexp.MustCompile(`universes/(?<UniverseID>\d+)\/places\/(?<PlaceID>\d+)\/(versions\/(?<VersionID>\d+)\/)?(luau-execution-sessions\/(?<SessionID>.+)?\/tasks\/(?<TaskID>.+)|(luau-execution-session-tasks\/(?<TaskID>.+)))`)
```

## Methods
### `CreateLuauExecutionSessionTask` <Badge type="info" text="universe.place.luau-execution-session:write" />
```go
func CreateLuauExecutionSessionTask(ctx context.Context, universeId, placeId string, versionId *string, data LuauExecutionTaskCreate) (*LuauExecutionTask, *Response, error)
```
##### Parameters
| Parameter  | Value           | Description                        | Required |
|------------|-----------------|------------------------------------|----------|
| ctx        | context.Context | The background context.            | true     |
| universeId | string          | The Universe ID of the experience. | true     |
| placeId    | string          | The Place ID for the universe.     | true     |
| versionId  | string          | The Version ID of the place.       | false    |
| data | [LuauExecutionTaskCreate](#luauexecutiontaskcreate) | The data to send to the API. | true |
---
### `GetLuauExecutionSessionTask` <Badge type="info" text="universe.place.luau-execution-session:read" />
```go
func GetLuauExecutionSessionTask(ctx context.Context, universeId, placeId string, versionId, sessionId *string, taskId string) (*LuauExecutionTask, *Response, error)
```
##### Parameters
| Parameter  | Value           | Description                        | Required |
|------------|-----------------|------------------------------------|----------|
| ctx        | context.Context | The background context.            | true     |
| universeId | string          | The Universe ID of the experience. | true     |
| placeId    | string          | The Place ID for the universe.     | true     |
| versionId  | string          | The Version ID of the place.       | false    |
| sessionId  | string          | The Session ID of the task.        | false    |
| taskId     | string          | The Task ID of the task.           | true     |
---
### `ListLuauExecutionSessionTaskLogs` <Badge type="info" text="universe.place.luau-execution-session:read" />
```go
func ListLuauExecutionSessionTaskLogs(ctx context.Context, universeId, placeId string, versionId, sessionId *string, taskId string, opts *Options) (*LuauExecutionTaskLogs, *Response, error)
```
##### Parameters
| Parameter  | Value           | Description                        | Required |
|------------|-----------------|------------------------------------|----------|
| ctx        | context.Context | The background context.            | true     |
| universeId | string          | The Universe ID of the experience. | true     |
| placeId    | string          | The Place ID for the universe.     | true     |
| versionId  | string          | The Version ID of the place.       | false    |
| sessionId  | string          | The Session ID of the task.        | false    |
| taskId     | string          | The Task ID of the task.           | true     |
| opts | [Options](/documentation/opencloud/common.html#options) | The options to send to the API. | false |

## Constants
### `LuauExecutionState` <Badge type="tip" text="string" />
```go
const (
	LuauExecutionStateUnspecified LuauExecutionState = "STATE_UNSPECIFIED"
	LuauExecutionStateQueued      LuauExecutionState = "QUEUED"
	LuauExecutionStateProcessing  LuauExecutionState = "PROCESSING"
	LuauExecutionStateCancelled   LuauExecutionState = "CANCELLED"
	LuauExecutionStateComplete    LuauExecutionState = "COMPLETE"
	LuauExecutionStateFailed      LuauExecutionState = "FAILED"
)
```
---
### `LuauExecutionErrorCode` <Badge type="tip" text="string" />
```go
const (
	LuauExecutionErrorCodeUnspecified             LuauExecutionErrorCode = "ERROR_CODE_UNSPECIFIED"
	LuauExecutionErrorCodeScriptError             LuauExecutionErrorCode = "SCRIPT_ERROR"
	LuauExecutionErrorCodeDeadlineExceeded        LuauExecutionErrorCode = "DEADLINE_EXCEEDED"
	LuauExecutionErrorCodeOutputSizeLimitExceeded LuauExecutionErrorCode = "OUTPUT_SIZE_LIMIT_EXCEEDED"
	LuauExecutionErrorCodeInternalError           LuauExecutionErrorCode = "INTERNAL_ERROR"
)
```
---
### `StructuredMessageType` <Badge type="tip" text="string" />
```go
const (
	StructuredMessageTypeUnspecified StructuredMessageType = "MESSAGE_TYPE_UNSPECIFIED"
	StructuredMessageTypeOutput      StructuredMessageType = "OUTPUT"
	StructuredMessageTypeInfo        StructuredMessageType = "INFO"
	StructuredMessageTypeWarning     StructuredMessageType = "WARNING"
	StructuredMessageTypeError       StructuredMessageType = "ERROR"
)
```

## Structures
### `LuauExecutionTaskError`
```go
type LuauExecutionTaskError struct {
	Code    LuauExecutionErrorCode `json:"code"`
	Message string                 `json:"message"`
}
```
---
### `LuauExecutionTaskOutput`
```go
type LuauExecutionTaskOutput struct {
	Results []any `json:"results"`
}
```
---
### `LuauExecutionTask`
```go
type LuauExecutionTask struct {
	Path                string                   `json:"path"`
	CreateTime          string                   `json:"createTime"`
	UpdateTime          string                   `json:"updateTime"`
	User                string                   `json:"user"`
	State               LuauExecutionState       `json:"state"`
	Script              string                   `json:"script"`
	Timeout             string                   `json:"timeout"`
	Error               *LuauExecutionTaskError  `json:"error"`
	Output              *LuauExecutionTaskOutput `json:"output"`
	BinaryInput         string                   `json:"binaryInput"`
	EnabledBinaryOutput bool                     `json:"enabledBinaryOutput"`
	BinaryOutputURI     string                   `json:"binaryOutputUri"`
}
```
#### `TaskInfo`
This method will return information from the URL path for the task.
This is useful for [`GetLuauExecutionSessionTask`](#getluauexecutionsessiontask) when polling the method.
```go
func (t *LuauExecutionTask) TaskInfo() (universeId string, placeId string, versionId *string, sessionId *string, taskId string)
```
---
### `LuauExecutionTaskCreate`
```go
type LuauExecutionTaskCreate struct {
	Script              *string `json:"script,omitempty"`
	Timeout             *string `json:"timeout,omitempty"`
	BinaryInput         *string `json:"binaryInput,omitempty"`
	EnabledBinaryOutput *bool   `json:"enabledBinaryOutput,omitempty"`
}
```
---
### `LuauExecutionTaskLogStructuredMessage`
```go
type LuauExecutionTaskLogStructuredMessage struct {
	Message     string                `json:"message"`
	CreateTime  string                `json:"createTime"`
	MessageType StructuredMessageType `json:"messageType"`
}
```
---
### `LuauExecutionTaskLog`
```go
type LuauExecutionTaskLog struct {
	Path    string   `json:"path"`
	Mesages []string `json:"messages"`
}
```
---
### `LuauExecutionTaskLogs`
```go
type LuauExecutionTaskLogs struct {
	LuauExecutionSessionTaskLogs []LuauExecutionTaskLog `json:"luauExecutionSessionTaskLogs"`
	NextPageToken                string                 `json:"nextPageToken"`
}
```