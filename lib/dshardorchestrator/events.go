package dshardorchestrator

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/pkg/errors"
	"github.com/vmihailenco/msgpack"
	"reflect"
)

// EventType represents a dshardorchestrator protocol event
// The internal event IDs are hardcoded to preserve compatibility between versions
type EventType uint32

const (

	// <10: misc control codes

	// EvtIdentify identify new node connection
	// orchestrator <- node: Identify the new connection, orchestrator responds with a EvtIdentified if successfulll
	EvtIdentify EventType = 1

	// EvtIdentified is a response to EvtIdentify
	// orchestrator -> node: The connection was sucessfully established, now ready for whatever else
	EvtIdentified EventType = 2

	// EvtShutdown is sent to shut down a node completely, exiting
	// orchestrator -> node: shut down the node completely
	EvtShutdown EventType = 3

	// 1x: Shard control codes

	// EvtStartShards assigns the following shards to the node, going through the full identify flow
	// orchestrator -> node: assign the shards to this node, respond with the same event when processed
	// orchestrator <- node: sent as a response when the node has been registered, does not need to have fully connected the shard yet, just registered.
	EvtStartShards EventType = 10

	// EvtStopShard is sent to stop the following shard
	// orhcestrator -> node: stop the shard, respond with the same event when done
	// orhcestrator <- node: sent when shard has been stopped
	EvtStopShard EventType = 11

	// 2x: migration codes

	// EvtPrepareShardmigration is sent from the orchestrator when we should prepare for a shard migration, and also used as a response
	// orchestrator -> origin node: close the gateway connection and respond with a EvtPrepareShardmigration with session ID and sequence number
	// orchestrator <- origin node: send when the origin node has closed the gateway connection, includes sessionID and sequence number for resuming on destination node, the event is forwarded to the destination node
	// orchestrator -> destination node: save the session id and sequence number and prepare for a incoming shard transfe, respond with EvtPrepareShardmigration when ready
	// orchestrator <- destination node: sent as a response when ready for the shard data transfer, followed by EvtStartShardMigration
	EvtPrepareShardmigration EventType = 20

	// EvtStartShardMigration is used when we should start transferring shard data, the flow goes like this:
	// orchestrator -> oirign node: start sending all user data events, should respond with a EvtAllUserdataSent with the total number of user data events sent
	EvtStartShardMigration EventType = 21

	// EvtAllUserdataSent is sent with the total number of user data events sent.
	// UserData events can still be sent after this, the migration is finished when n user data events is received.
	// where n is sent in this event.
	// orchestrator <- origin node: sent at the end or during the shard user data transfer, includes total number of events that will be sent, forwarded to destination node
	// orchestrator -> destination node: the above, directly forwarded to the destination node, when the provided number of user data events has been received, the transfer is complete, the node is responsible for tracking this
	EvtAllUserdataSent EventType = 23

	// EvtShardMigrationDataStartID isn't an event per se, but this marks where user id's start
	// events with higher ID than this are registered and fully handled by implementations of the node interface
	// and will not be decoded or touched by the orchestrator.
	//
	// This can be used to transfer any kind of data during shard migration from the old node to the new node
	// to do that you could register a new event for "Guild" states, and send those over one by one.
	EvtShardMigrationDataStartID EventType = 100
)

// EventsToStringMap is a mapping of events to their string names
var EventsToStringMap = map[EventType]string{
	// <10: misc control codes
	1: "Identify",
	2: "Identified",
	3: "Shutdown",

	// 1x: Shard control codes
	10: "StartShards",
	11: "StopShard",

	// 2x: migration codes
	20: "PrepareShardmigration",
	21: "StartShardMigration",
	22: "AllUserdataSent",
}

func (evt EventType) String() string {
	if s, ok := EventsToStringMap[evt]; ok {
		return s
	}

	return "Unknown"
}

// EvtDataMap is a mapping of events to structs for their data
var EvtDataMap = map[EventType]interface{}{
	EvtIdentify:              IdentifyData{},
	EvtIdentified:            IdentifiedData{},
	EvtStartShards:           StartShardsData{},
	EvtStopShard:             StopShardData{},
	EvtPrepareShardmigration: PrepareShardmigrationData{},
	EvtStartShardMigration:   StartshardMigrationData{},
	EvtAllUserdataSent:       AllUserDataSentData{},
}

// RegisterUserEvent registers a new user event to be used in shard migration for example
// calling this after opening a connection or otherwise concurrently will cause race conditions
// the reccomended way would be to call this in init()
//
// panics if id is less than 100, as that's reserved id's for inernal use
func RegisterUserEvent(name string, id EventType, dataType interface{}) {
	if id < EvtShardMigrationDataStartID {
		panic(errors.New("tried registering user event with event type less than 100"))
	}

	EvtDataMap[id] = dataType
	EventsToStringMap[id] = "UserEvt:" + name
}

// Message represents a protocol message
type Message struct {
	EvtID EventType

	// only 1 of RawBody or DecodeBody is present, not both
	RawBody     []byte
	DecodedBody interface{}
}

// EncodeMessage is the same as EncodeMessageRaw but also encodes the data passed using msgpack
func EncodeMessage(evtID EventType, data interface{}) ([]byte, error) {
	if data == nil {
		return EncodeMessageRaw(evtID, nil), nil
	}

	if c, ok := data.([]byte); ok {
		return EncodeMessageRaw(evtID, c), nil
	}

	serialized, err := msgpack.Marshal(data)
	if err != nil {
		return nil, errors.WithMessage(err, "msgpack.Marshal")
	}

	return EncodeMessageRaw(evtID, serialized), nil

}

// EncodeMessageRaw encodes the event to the wire format
// The wire format is pretty basic, first 4 bytes is a uin32 representing what type of event this is
// next 4 bytes is another uin32 which represents the length of the body
// next n bytes is the body itself, which can even be empty in some cases
func EncodeMessageRaw(evtID EventType, data []byte) []byte {
	var buf bytes.Buffer

	tmpBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(tmpBuf, uint32(evtID))
	buf.Write(tmpBuf)

	l := uint32(len(data))
	binary.LittleEndian.PutUint32(tmpBuf, l)
	buf.Write(tmpBuf)
	buf.Write(data)

	return buf.Bytes()
}

// UnknownEventError represents an error for unknown events, this is techincally impossible with protocol versions being enforced, but who knows if you write your own node
type UnknownEventError struct {
	Evt EventType
}

func (uee *UnknownEventError) Error() string {
	return fmt.Sprintf("Unknown event: %d", uee.Evt)
}

// DecodePayload decodes a event payload according to the specific event
func DecodePayload(evtID EventType, payload []byte) (interface{}, error) {
	t, ok := EvtDataMap[evtID]

	if !ok {
		if _, ok := EventsToStringMap[evtID]; ok {
			// valid event
			return nil, nil
		}
		return nil, &UnknownEventError{Evt: evtID}
	}

	if t == nil {
		return nil, nil
	}

	clone := reflect.New(reflect.TypeOf(t)).Interface()
	err := msgpack.Unmarshal(payload, clone)
	return clone, err
}
