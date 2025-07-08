package opencloud

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
)

var (
	// https://regex101.com/r/8SofBV/1
	LuauExecutionTaskPathRegex = regexp.MustCompile(`universes/(?<UniverseID>\d+)\/places\/(?<PlaceID>\d+)\/(versions\/(?<VersionID>\d+)\/)?(luau-execution-sessions\/(?<SessionID>.+)?\/tasks\/(?<TaskID>.+)|(luau-execution-session-tasks\/(?<TaskID>.+)))`)
)

// LuauExecutionService will handle communciation with the actions related to the API.
//
// Roblox Open Cloud API Docs: https://create.roblox.com/docs/en-us/cloud
type LuauExecutionService struct {
	client *Client
}

type LuauExecutionState string

const (
	LuauExecutionStateUnspecified LuauExecutionState = "STATE_UNSPECIFIED"
	LuauExecutionStateQueued      LuauExecutionState = "QUEUED"
	LuauExecutionStateProcessing  LuauExecutionState = "PROCESSING"
	LuauExecutionStateCancelled   LuauExecutionState = "CANCELLED"
	LuauExecutionStateComplete    LuauExecutionState = "COMPLETE"
	LuauExecutionStateFailed      LuauExecutionState = "FAILED"
)

type LuauExecutionErrorCode string

const (
	LuauExecutionErrorCodeUnspecified             LuauExecutionErrorCode = "ERROR_CODE_UNSPECIFIED"
	LuauExecutionErrorCodeScriptError             LuauExecutionErrorCode = "SCRIPT_ERROR"
	LuauExecutionErrorCodeDeadlineExceeded        LuauExecutionErrorCode = "DEADLINE_EXCEEDED"
	LuauExecutionErrorCodeOutputSizeLimitExceeded LuauExecutionErrorCode = "OUTPUT_SIZE_LIMIT_EXCEEDED"
	LuauExecutionErrorCodeInternalError           LuauExecutionErrorCode = "INTERNAL_ERROR"
)

type LuauExecutionTaskError struct {
	Code    LuauExecutionErrorCode `json:"code"`
	Message string                 `json:"message"`
}

type LuauExecutionTaskOutput struct {
	Results []any `json:"results"`
}

type LuauExecutionTask struct {
	Path                string                   `json:"path"`
	CreateTime          string                   `json:"createTime"`
	UpdateTime          string                   `json:"updateTime"`
	User                string                   `json:"user"`
	State               LuauExecutionState       `json:"state"`
	Script              string                   `json:"script"`
	Timeout             string                   `json:"timeout"`
	Error               *LuauExecutionTaskError  `json:"error,omitempty"`
	Output              *LuauExecutionTaskOutput `json:"output,omitempty"`
	BinaryInput         string                   `json:"binaryInput"`
	EnabledBinaryOutput bool                     `json:"enabledBinaryOutput"`
	BinaryOutputURI     string                   `json:"binaryOutputUri"`
}

func getInfo(path string) map[string]string {
	match := LuauExecutionTaskPathRegex.FindStringSubmatch(path)
	results := make(map[string]string)

	if match == nil {
		return results
	}

	for i, name := range LuauExecutionTaskPathRegex.SubexpNames() {
		value := match[i]
		if name == "" || value == "" {
			continue
		}

		results[name] = value
	}

	return results
}

// TaskInfo will return information from the URL path for the task.
// This is useful for GetLuauExecutionSessionTask when polling the method.
//
// universeId, placeId, and taskId will always be present.
// versionId and sessionId may or may not be nil.
func (t *LuauExecutionTask) TaskInfo() (universeId, placeId string, versionId, sessionId *string, taskId string) {
	info := getInfo(t.Path)

	for key, value := range info {
		switch key {
		case "UniverseID":
			universeId = value
		case "PlaceID":
			placeId = value
		case "VersionID":
			versionId = &value
		case "SessionID":
			sessionId = &value
		case "TaskID":
			taskId = value
		}
	}

	return universeId, placeId, versionId, sessionId, taskId
}

type LuauExecutionTaskCreate struct {
	Script              *string `json:"script,omitempty"`
	Timeout             *string `json:"timeout,omitempty"`
	BinaryInput         *string `json:"binaryInput,omitempty"`
	EnabledBinaryOutput *bool   `json:"enabledBinaryOutput,omitempty"`
}

// CreateLuauExecutionSessionTask will execute a Luau script on a specific place.
// If a versionId is provided, the code will be executed on that version of the place.
//
// Required scopes: universe.place.luau-execution-session:write
//
// Roblox Opencloud API Docs: https://create.roblox.com/docs/en-us/cloud/reference/LuauExecutionSessionTask#Cloud_CreateLuauExecutionSessionTask
//
// [POST] /cloud/v2/universes/{universe_id}/places/{place_id}/luau-execution-session-tasks
//
// [POST] /cloud/v2/universes/{universe_id}/places/{place_id}/luau-execution-session-tasks/{version_id}
func (s *LuauExecutionService) CreateLuauExecutionSessionTask(ctx context.Context, universeId, placeId string, versionId *string, data LuauExecutionTaskCreate) (*LuauExecutionTask, *Response, error) {
	u := fmt.Sprintf("/cloud/v2/universes/%s/places/%s/luau-execution-session-tasks", universeId, placeId)
	if versionId != nil {
		u = fmt.Sprintf("/cloud/v2/universes/%s/places/%s/luau-execution-session-tasks/%s", universeId, placeId, *versionId)
	}

	req, err := s.client.NewRequest(http.MethodPost, u, data)
	if err != nil {
		return nil, nil, err
	}

	luauExecutionSessionTask := new(LuauExecutionTask)
	resp, err := s.client.Do(ctx, req, luauExecutionSessionTask)
	if err != nil {
		return nil, resp, err
	}

	return luauExecutionSessionTask, resp, nil
}

// GetLuauExecutionSessionTask will fetch the executed Luau task.
//
// Required scopes:
//
// - universe.place.luau-execution-session:read
//
// - universe.place.luau-execution-session:write
//
// Roblox Opencloud API Docs: https://create.roblox.com/docs/en-us/cloud/reference/LuauExecutionSessionTask#Cloud_GetLuauExecutionSessionTask
//
// [GET] /cloud/v2/universes/{universe_id}/places/{place_id}/luau-execution-tasks/{task_id}
//
// [GET] /cloud/v2/universes/{universe_id}/places/{place_id}/versions/{version_id}/luau-execution-tasks/{task_id}
//
// [GET] /cloud/v2/universes/{universe_id}/places/{place_id}/luau-execution-sessions/{session_id}/tasks/{task_id}
//
// [GET] /cloud/v2/universes/{universe_id}/places/{place_id}/versions/{version_id}/luau-execution-sessions/{session_id}/tasks/{task_id}
func (s *LuauExecutionService) GetLuauExecutionSessionTask(ctx context.Context, universeId, placeId string, versionId, sessionId *string, taskId string) (*LuauExecutionTask, *Response, error) {
	u := fmt.Sprintf("/cloud/v2/universes/%s/places/%s", universeId, placeId)

	if versionId != nil {
		u += fmt.Sprintf("/versions/%s", *versionId)
	}

	if sessionId != nil {
		u += fmt.Sprintf("/luau-execution-sessions/%s/tasks/%s", *sessionId, taskId)
	} else {
		u += fmt.Sprintf("/luau-execution-tasks/%s", taskId)
	}

	req, err := s.client.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, nil, err
	}

	luauExecutionSessionTask := new(LuauExecutionTask)
	resp, err := s.client.Do(ctx, req, luauExecutionSessionTask)
	if err != nil {
		return nil, resp, err
	}

	return luauExecutionSessionTask, resp, nil
}

type StructuredMessageType string

const (
	StructuredMessageTypeUnspecified StructuredMessageType = "MESSAGE_TYPE_UNSPECIFIED"
	StructuredMessageTypeOutput      StructuredMessageType = "OUTPUT"
	StructuredMessageTypeInfo        StructuredMessageType = "INFO"
	StructuredMessageTypeWarning     StructuredMessageType = "WARNING"
	StructuredMessageTypeError       StructuredMessageType = "ERROR"
)

type LuauExecutionTaskLogStructuredMessage struct {
	Message     string                `json:"message"`
	CreateTime  string                `json:"createTime"`
	MessageType StructuredMessageType `json:"messageType"`
}

type LuauExecutionTaskLog struct {
	Path    string   `json:"path"`
	Mesages []string `json:"messages"`
}

type LuauExecutionTaskLogs struct {
	LuauExecutionSessionTaskLogs []LuauExecutionTaskLog `json:"luauExecutionSessionTaskLogs"`
	NextPageToken                string                 `json:"nextPageToken"`
}

// ListLuauExecutionSessionTaskLogs will list log chunks generated from a specific Luau task.
//
// Required scopes:
//
// - universe.place.luau-execution-session:read
//
// - universe.place.luau-execution-session:write
//
// Roblox Opencloud API Docs: https://create.roblox.com/docs/en-us/cloud/reference/LuauExecutionSessionTaskLog#Cloud_ListLuauExecutionSessionTaskLogs
//
// [GET] /cloud/v2/universes/{universe_id}/places/{place_id}/luau-execution-tasks/{task_id}/logs
//
// [GET] /cloud/v2/universes/{universe_id}/places/{place_id}/versions/{version_id}/luau-execution-tasks/{task_id}/logs
//
// [GET] /cloud/v2/universes/{universe_id}/places/{place_id}/luau-execution-sessions/{session_id}/tasks/{task_id}/logs
//
// [GET] /cloud/v2/universes/{universe_id}/places/{place_id}/versions/{version_id}/luau-execution-sessions/{session_id}/tasks/{task_id}/logs
func (s *LuauExecutionService) ListLuauExecutionSessionTaskLogs(ctx context.Context, universeId, placeId string, versionId, sessionId *string, taskId string, opts *Options) (*LuauExecutionTaskLogs, *Response, error) {
	u := fmt.Sprintf("/cloud/v2/universes/%s/places/%s", universeId, placeId)

	if versionId != nil {
		u += fmt.Sprintf("/versions/%s", *versionId)
	}

	if sessionId != nil {
		u += fmt.Sprintf("/luau-execution-sessions/%s/tasks/%s", *sessionId, taskId)
	} else {
		u += fmt.Sprintf("/luau-execution-tasks/%s", taskId)
	}

	u, err := addOpts(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, nil, err
	}

	luauExecutionSessionTaskLogs := new(LuauExecutionTaskLogs)
	resp, err := s.client.Do(ctx, req, luauExecutionSessionTaskLogs)
	if err != nil {
		return nil, resp, err
	}

	return luauExecutionSessionTaskLogs, resp, nil
}
