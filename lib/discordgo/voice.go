// Discordgo - Discord bindings for Go
// Available at https://github.com/bwmarrin/discordgo

// Copyright 2015-2016 Bruce Marriner <bruce@sqls.net>.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file contains code related to Discord voice suppport

package discordgo

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/chacha20poly1305"
)

// ------------------------------------------------------------------------------------------------
// Code related to both VoiceConnection Websocket and UDP connections.
// ------------------------------------------------------------------------------------------------

// A VoiceConnection struct holds all the data and functions related to a Discord Voice Connection.
type VoiceConnection struct {
	sync.RWMutex

	Debug        bool // If true, print extra logging -- DEPRECATED
	LogLevel     int
	Ready        bool // If true, voice is ready to send/receive audio
	UserID       int64
	GuildID      int64
	ChannelID    int64
	deaf         bool
	mute         bool
	speaking     bool
	reconnecting bool // If true, voice connection is trying to reconnect

	OpusSend chan []byte  // Chan for sending opus audio
	OpusRecv chan *Packet // Chan for receiving opus audio

	wsConn             *websocket.Conn
	wsMutex            sync.Mutex
	udpConn            *net.UDPConn
	session            *Session
	gatewayConn        *GatewayConnection
	gatewayConnManager *GatewayConnectionManager

	sessionID string
	token     string
	endpoint  string

	// Used to send a close signal to goroutines
	close chan struct{}

	// Used to allow blocking until connected
	Connected chan bool

	// Used to pass the sessionid from onVoiceStateUpdate
	// sessionRecv chan string UNUSED ATM

	aead cipher.AEAD

	dave *DAVESession

	ssrcToUserID map[uint32]string

	pendingReWelcome bool

	seqAck int

	op4 voiceOP4
	op2 voiceOP2
	op8 voiceOP8

	voiceSpeakingUpdateHandlers []VoiceSpeakingUpdateHandler
}

// VoiceSpeakingUpdateHandler type provides a function definition for the
// VoiceSpeakingUpdate event
type VoiceSpeakingUpdateHandler func(vc *VoiceConnection, vs *VoiceSpeakingUpdate)

// Speaking sends a speaking notification to Discord over the voice websocket.
// This must be sent as true prior to sending audio and should be set to false
// once finished sending audio.
//
//	b  : Send true if speaking, false if not.
func (v *VoiceConnection) Speaking(b bool) (err error) {

	v.log(LogDebug, "called (%t)", b)

	type voiceSpeakingData struct {
		Speaking bool `json:"speaking"`
		Delay    int  `json:"delay"`
	}

	type voiceSpeakingOp struct {
		Op   int               `json:"op"` // Always 5
		Data voiceSpeakingData `json:"d"`
	}

	if v.wsConn == nil {
		return fmt.Errorf("no VoiceConnection websocket")
	}

	data := voiceSpeakingOp{5, voiceSpeakingData{b, 0}}
	v.wsMutex.Lock()
	err = v.wsConn.WriteJSON(data)
	v.wsMutex.Unlock()

	v.Lock()
	defer v.Unlock()
	if err != nil {
		v.speaking = false
		v.log(LogError, "Speaking() write json error: %v", err)
		return
	}

	v.speaking = b

	return
}

// ChangeChannel sends Discord a request to change channels within a Guild
// !!! NOTE !!! This function may be removed in favour of just using ChannelVoiceJoin
func (v *VoiceConnection) ChangeChannel(channelID int64, mute, deaf bool) (err error) {

	v.log(LogInformational, "called")

	v.Lock()

	strGID := StrID(v.GuildID)
	strCID := StrID(v.ChannelID)

	data := outgoingEvent{
		Operation: GatewayOPVoiceStateUpdate,
		Data:      voiceChannelJoinData{&strGID, &strCID, mute, deaf, 1},
	}

	v.gatewayConn.writer.Queue(data)
	v.Unlock()

	v.ChannelID = channelID
	v.deaf = deaf
	v.mute = mute
	v.speaking = false

	return
}

// Disconnect disconnects from this voice channel and closes the websocket
// and udp connections to Discord.
// !!! NOTE !!! this function may be removed in favour of ChannelVoiceLeave
func (v *VoiceConnection) Disconnect() (err error) {

	v.Lock()
	// Send a OP4 with a nil channel to disconnect
	strGID := StrID(v.GuildID)

	data := outgoingEvent{
		Operation: GatewayOPVoiceStateUpdate,
		Data:      voiceChannelJoinData{&strGID, nil, true, true, 1},
	}

	v.gatewayConn.writer.Queue(data)

	v.Unlock()

	// Close websocket and udp connections
	v.Close()

	return
}

// Close closes the voice ws and udp connections
func (v *VoiceConnection) Close() {

	v.log(LogInformational, "called")

	v.Lock()
	defer v.Unlock()

	v.Ready = false
	v.speaking = false
	v.dave = nil

	if v.close != nil {
		v.log(LogInformational, "closing v.close")
		close(v.close)
		v.close = nil
	}

	if v.udpConn != nil {
		v.log(LogInformational, "closing udp")
		err := v.udpConn.Close()
		if err != nil {
			v.log(LogError, "error closing udp connection: %v", err)
		}
		v.udpConn = nil
	}

	if v.wsConn != nil {
		v.log(LogInformational, "sending close frame")

		// To cleanly close a connection, a client should send a close
		// frame and wait for the server to close the connection.
		v.wsMutex.Lock()
		err := v.wsConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		v.wsMutex.Unlock()
		if err != nil {
			v.log(LogError, "error closing websocket, %s", err)
		}

		// TODO: Wait for Discord to actually close the connection.
		time.Sleep(1 * time.Second)

		v.log(LogInformational, "closing websocket")
		err = v.wsConn.Close()
		if err != nil {
			v.log(LogError, "error closing websocket, %s", err)
		}

		v.wsConn = nil
	}
}

// AddHandler adds a Handler for VoiceSpeakingUpdate events.
func (v *VoiceConnection) AddHandler(h VoiceSpeakingUpdateHandler) {
	v.Lock()
	defer v.Unlock()

	v.voiceSpeakingUpdateHandlers = append(v.voiceSpeakingUpdateHandlers, h)
}

// VoiceSpeakingUpdate is a struct for a VoiceSpeakingUpdate event.
type VoiceSpeakingUpdate struct {
	UserID   string `json:"user_id"`
	SSRC     int    `json:"ssrc"`
	Speaking int    `json:"speaking"`
}

// ------------------------------------------------------------------------------------------------
// Unexported Internal Functions Below.
// ------------------------------------------------------------------------------------------------

type voiceWebsocketMessage struct {
	Operation int             `json:"op"`
	RawData   json.RawMessage `json:"d"`
	Sequence  *int            `json:"seq"`
}

// A voiceOP4 stores the data for the voice operation 4 websocket event
// which provides us with the NaCl SecretBox encryption key
type voiceOP4 struct {
	SecretKey           []byte    `json:"secret_key"`
	Mode                VoiceMode `json:"mode"`
	DAVEProtocolVersion int       `json:"dave_protocol_version"`
}

// A voiceOP2 stores the data for the voice operation 2 websocket event
// which is sort of like the voice READY packet
type voiceOP2 struct {
	SSRC  uint32   `json:"ssrc"`
	Port  int      `json:"port"`
	Modes []string `json:"modes"`
	IP    string   `json:"ip"`
}

// A voiceOP8 stores the data for the voice operation 8 websocket event HELLO
type voiceOP8 struct {
	HeartbeatInterval int `json:"heartbeat_interval"`
}

var ErrTimeoutWaitingForVoice = errors.New("timeout waiting for voice")

// WaitUntilConnected waits for the Voice Connection to
// become ready, if it does not become ready it returns an err
func (v *VoiceConnection) waitUntilConnected() error {

	v.log(LogInformational, "called")

	i := 0
	for {
		v.RLock()
		ready := v.Ready
		v.RUnlock()
		if ready {
			return nil
		}

		if i > 10 {
			return ErrTimeoutWaitingForVoice
		}

		time.Sleep(1 * time.Second)
		i++
	}
}

// Open opens a voice connection.  This should be called
// after VoiceChannelJoin is used and the data VOICE websocket events
// are captured.
func (v *VoiceConnection) open() (err error) {

	v.log(LogInformational, "called")

	v.Lock()
	defer v.Unlock()

	// Don't open a websocket if one is already open
	if v.wsConn != nil {
		v.log(LogWarning, "refusing to overwrite non-nil websocket")
		return
	}

	// TODO temp? loop to wait for the SessionID
	i := 0
	for {
		if v.sessionID != "" {
			break
		}
		if i > 20 { // only loop for up to 1 second total
			return fmt.Errorf("did not receive voice Session ID in time")
		}
		time.Sleep(50 * time.Millisecond)
		i++
	}

	// Connect to VoiceConnection Websocket
	vg := fmt.Sprintf("wss://%s?v=8", strings.TrimSuffix(v.endpoint, ":80"))
	v.log(LogInformational, "connecting to voice endpoint %s", vg)
	v.wsConn, _, err = websocket.DefaultDialer.Dial(vg, nil)
	if err != nil {
		v.log(LogWarning, "error connecting to voice endpoint %s, %s", vg, err)
		v.log(LogDebug, "voice struct: %#v\n", v)
		return
	}

	type voiceHandshakeData struct {
		ServerID               int64  `json:"server_id,string"`
		UserID                 int64  `json:"user_id,string"`
		SessionID              string `json:"session_id"`
		Token                  string `json:"token"`
		MaxDAVEProtocolVersion int    `json:"max_dave_protocol_version"`
	}
	type voiceHandshakeOp struct {
		Op   int                `json:"op"` // Always 0
		Data voiceHandshakeData `json:"d"`
	}
	data := voiceHandshakeOp{0, voiceHandshakeData{v.GuildID, v.UserID, v.sessionID, v.token, 1}}

	err = v.wsConn.WriteJSON(data)
	if err != nil {
		v.log(LogWarning, "error sending init packet, %s", err)
		return
	}

	v.close = make(chan struct{})
	go v.wsListen(v.wsConn, v.close)

	// add loop/check for Ready bool here?
	// then return false if not ready?
	// but then wsListen will also err.

	return
}

// wsListen listens on the voice websocket for messages and passes them
// to the voice event handler.  This is automatically called by the Open func
func (v *VoiceConnection) wsListen(wsConn *websocket.Conn, close <-chan struct{}) {

	v.log(LogInformational, "called")

	for {
		messageType, message, err := v.wsConn.ReadMessage()
		if err != nil {
			// Detect if we have been closed manually. If a Close() has already
			// happened, the websocket we are listening on will be different to the
			// current session.
			v.RLock()
			sameConnection := v.wsConn == wsConn
			v.RUnlock()
			if sameConnection {
				// 4014 indicates a manual disconnection by someone in the guild;
				// 4017 indicates DAVE protocol required but not supported;
				// we shouldn't reconnect.
				if websocket.IsCloseError(err, 4014, 4017) {
					v.log(LogInformational, "received manual disconnection or DAVE error: %v", err)

					// Abandon the voice WS connection
					v.wsMutex.Lock()
					v.wsConn = nil
					v.wsMutex.Unlock()

					// Close everything
					v.Close()
					return
				}

				v.log(LogError, "voice endpoint %s websocket closed unexpectantly, %s", v.endpoint, err)

				// Start reconnect goroutine then exit.
				go v.reconnect(nil)
			}
			return
		}

		// Pass received message to voice event handler
		select {
		case <-close:
			return
		default:
			go v.onEvent(messageType == websocket.BinaryMessage, message)
		}
	}
}

// wsEvent handles any voice websocket events. This is only called by the
// wsListen() function.
func (v *VoiceConnection) onEvent(isBinary bool, message []byte) {
	if isBinary {
		if len(message) >= 4 {
			v.log(LogDebug, "received binary: len=%d first_bytes=[%02x %02x %02x %02x]", len(message), message[0], message[1], message[2], message[3])
		} else {
			v.log(LogDebug, "received binary: len=%d bytes=%x", len(message), message)
		}
		v.handleDAVEBinary(message)
		return
	}

	v.log(LogDebug, "received: %s", string(message))

	var e voiceWebsocketMessage
	if err := json.Unmarshal(message, &e); err != nil {
		v.log(LogError, "unmarshall error, %s", err)
		return
	}

	if e.Sequence != nil {
		v.Lock()
		v.seqAck = *e.Sequence
		v.Unlock()
	}

	switch e.Operation {

	case 2: // READY
		if err := json.Unmarshal(e.RawData, &v.op2); err != nil {
			v.log(LogError, "OP2 unmarshall error, %s, %s", err, string(e.RawData))
			return
		}

		// Start the UDP connection
		err := v.udpOpen()
		if err != nil {
			v.log(LogError, "error opening udp connection, %s", err)
			return
		}

		return

	case 3: // HEARTBEAT response
		return

	case 4: // udp encryption secret key
		v.Lock()

		v.op4 = voiceOP4{}
		if err := json.Unmarshal(e.RawData, &v.op4); err != nil {
			v.Unlock()
			v.log(LogError, "OP4 unmarshall error, %s, %s", err, string(e.RawData))
			return
		}

		v.log(LogInformational, "OP4 received: mode=%s, dave_version=%d",
			v.op4.Mode, v.op4.DAVEProtocolVersion)

		switch v.op4.Mode {
		case VoiceModeAeadAes256GcmRtpsize:
			block, err := aes.NewCipher(v.op4.SecretKey)
			if err != nil {
				v.Unlock()
				v.log(LogError, "error creating AES cipher, %s", err)
				return
			}
			v.aead, err = cipher.NewGCM(block)
			if err != nil {
				v.Unlock()
				v.log(LogError, "error creating GCM, %s", err)
				return
			}
		case VoiceModeAeadXChaCha20Poly1305Rtpsize:
			var err error
			v.aead, err = chacha20poly1305.NewX(v.op4.SecretKey)
			if err != nil {
				v.Unlock()
				v.log(LogError, "error creating XChaCha20 cipher, %s", err)
				return
			}
		default:
			v.Unlock()
			v.log(LogError, "unknown encryption mode: %s", v.op4.Mode)
			return
		}

		var daveKPData []byte
		if v.op4.DAVEProtocolVersion > 0 {
			v.dave = NewDAVESession(StrID(v.UserID))
			for ssrc, userID := range v.ssrcToUserID {
				v.dave.SetSSRC(ssrc, userID)
			}

			var err error
			daveKPData, err = v.dave.GenerateKeyPackage()
			if err != nil {
				v.log(LogError, "DAVE key package generation failed: %s", err)
			}
		}

		if v.OpusSend == nil {
			v.OpusSend = make(chan []byte, 16)
		}
		go v.opusSender(v.udpConn, v.close, v.OpusSend, 48000, 960)

		if !v.deaf {
			if v.OpusRecv == nil {
				v.OpusRecv = make(chan *Packet, 2)
			}
			go v.opusReceiver(v.udpConn, v.close, v.OpusRecv)
		}

		v.Ready = true
		v.Unlock()

		if daveKPData != nil {
			v.sendDAVEKeyPackageBinary(daveKPData)
		}

		// Send the ready event
		v.Connected <- true
		return

	case 5:
		voiceSpeakingUpdate := &VoiceSpeakingUpdate{}
		if err := json.Unmarshal(e.RawData, voiceSpeakingUpdate); err != nil {
			v.log(LogError, "OP5 unmarshall error, %s, %s", err, string(e.RawData))
			return
		}

		v.Lock()
		if v.ssrcToUserID == nil {
			v.ssrcToUserID = make(map[uint32]string)
		}
		v.ssrcToUserID[uint32(voiceSpeakingUpdate.SSRC)] = voiceSpeakingUpdate.UserID
		dave := v.dave
		v.Unlock()
		if dave != nil {
			dave.SetSSRC(uint32(voiceSpeakingUpdate.SSRC), voiceSpeakingUpdate.UserID)
		}

		for _, h := range v.voiceSpeakingUpdateHandlers {
			h(v, voiceSpeakingUpdate)
		}

	case 8: // HELLO
		if err := json.Unmarshal(e.RawData, &v.op8); err != nil {
			v.log(LogError, "OP8 unmarshall error, %s, %s", err, string(e.RawData))
			return
		}

		// Start the voice websocket heartbeat to keep the connection alive
		go v.wsHeartbeat(v.wsConn, v.close, time.Duration(v.op8.HeartbeatInterval))

	case 12: // CLIENT CONNECT
		var op12 struct {
			UserID    string `json:"user_id"`
			AudioSSRC uint32 `json:"audio_ssrc"`
		}
		if err := json.Unmarshal(e.RawData, &op12); err != nil {
			v.log(LogError, "OP12 unmarshal error, %s, %s", err, string(e.RawData))
			return
		}
		if op12.AudioSSRC != 0 {
			v.Lock()
			if v.ssrcToUserID == nil {
				v.ssrcToUserID = make(map[uint32]string)
			}
			v.ssrcToUserID[op12.AudioSSRC] = op12.UserID
			dave := v.dave
			v.Unlock()
			if dave != nil {
				dave.SetSSRC(op12.AudioSSRC, op12.UserID)
			}
		}
		return

	case 13: // Client Disconnect
		v.log(LogDebug, "user disconnected: %s", string(e.RawData))
		return

	case 21: // DAVE prepare_transition
		v.handleDAVEPrepareTransition(e.RawData)
		return

	case 22: // DAVE execute_transition
		v.handleDAVEExecuteTransition(e.RawData)
		return

	case 24: // DAVE prepare_epoch
		v.handleDAVEPrepareEpoch(e.RawData)
		return

	default:
		v.log(LogDebug, "unknown voice operation, %d, %s", e.Operation, string(e.RawData))
	}
}

type voiceHeartbeatOp struct {
	Op   int                `json:"op"` // Always 3
	Data voiceHeartbeatData `json:"d"`
}

type voiceHeartbeatData struct {
	T      int64 `json:"t"`
	SeqAck int   `json:"seq_ack"`
}

// NOTE :: When a guild voice server changes how do we shut this down
// properly, so a new connection can be setup without fuss?
//
// wsHeartbeat sends regular heartbeats to voice Discord so it knows the client
// is still connected.  If you do not send these heartbeats Discord will
// disconnect the websocket connection after a few seconds.
func (v *VoiceConnection) wsHeartbeat(wsConn *websocket.Conn, close <-chan struct{}, i time.Duration) {

	if close == nil || wsConn == nil {
		return
	}

	var err error
	ticker := time.NewTicker(i * time.Millisecond)
	defer ticker.Stop()
	for {
		v.log(LogDebug, "sending heartbeat packet")
		v.RLock()
		seqAck := v.seqAck
		v.RUnlock()
		v.wsMutex.Lock()
		err = wsConn.WriteJSON(voiceHeartbeatOp{3, voiceHeartbeatData{time.Now().Unix(), seqAck}})
		v.wsMutex.Unlock()
		if err != nil {
			v.log(LogError, "error sending heartbeat to voice endpoint %s, %s", v.endpoint, err)
			return
		}

		select {
		case <-ticker.C:
			// continue loop and send heartbeat
		case <-close:
			return
		}
	}
}

// ------------------------------------------------------------------------------------------------
// Code related to the VoiceConnection UDP connection
// ------------------------------------------------------------------------------------------------

type voiceUDPData struct {
	Address string    `json:"address"` // Public IP of machine running this code
	Port    uint16    `json:"port"`    // UDP Port of machine running this code
	Mode    VoiceMode `json:"mode"`    // Discord voice mode
}

type VoiceMode string

const (
	VoiceModeXSalsa20Poly1305Lite         VoiceMode = "xsalsa20_poly1305_lite"
	VoiceModeXSalsa20Poly1305Suffix       VoiceMode = "xsalsa20_poly1305_suffix"
	VoiceModeXSalsa20Poly1305             VoiceMode = "xsalsa20_poly1305"
	VoiceModeAeadAes256GcmRtpsize         VoiceMode = "aead_aes256_gcm_rtpsize"
	VoiceModeAeadXChaCha20Poly1305Rtpsize VoiceMode = "aead_xchacha20_poly1305_rtpsize"
	VoiceModeXSalsa20Poly1305LiteRtpsize  VoiceMode = "xsalsa20_poly1305_lite_rtpsize"
)

type voiceUDPD struct {
	Protocol               string       `json:"protocol"`
	Data                   voiceUDPData `json:"data"`
	MaxDAVEProtocolVersion int          `json:"max_dave_protocol_version,omitempty"`
}

type voiceUDPOp struct {
	Op   int       `json:"op"` // Always 1
	Data voiceUDPD `json:"d"`
}

// udpOpen opens a UDP connection to the voice server and completes the
// initial required handshake.  This connection is left open in the session
// and can be used to send or receive audio.  This should only be called
// from voice.wsEvent OP2
func (v *VoiceConnection) udpOpen() (err error) {

	v.Lock()
	defer v.Unlock()

	if v.wsConn == nil {
		return fmt.Errorf("nil voice websocket")
	}

	if v.udpConn != nil {
		return fmt.Errorf("udp connection already open")
	}

	if v.close == nil {
		return fmt.Errorf("nil close channel")
	}

	if v.op2.IP == "" {
		return fmt.Errorf("empty endpoint")
	}

	host := fmt.Sprintf("%s:%d", strings.TrimSuffix(v.op2.IP, ":80"), v.op2.Port)
	addr, err := net.ResolveUDPAddr("udp", host)
	if err != nil {
		v.log(LogWarning, "error resolving udp host %s, %s", host, err)
		return
	}

	v.log(LogInformational, "connecting to udp addr %s", addr.String())
	v.udpConn, err = net.DialUDP("udp", nil, addr)
	if err != nil {
		v.log(LogWarning, "error connecting to udp addr %s, %s", addr.String(), err)
		return
	}

	// Create a 74 byte array and put the SSRC code from the Op 2 VoiceConnection event
	// into it.  Then send that over the UDP connection to Discord
	// Create a 74 byte array to store the packet data
	sb := make([]byte, 74)
	binary.BigEndian.PutUint16(sb, 1)              // Packet type (0x1 is request, 0x2 is response)
	binary.BigEndian.PutUint16(sb[2:], 70)         // Packet length (excluding type and length fields)
	binary.BigEndian.PutUint32(sb[4:], v.op2.SSRC) // The SSRC code from the Op 2 VoiceConnection event

	// And send that data over the UDP connection to Discord.
	v.log(LogInformational, "op2 SSRC: %d", v.op2.SSRC)
	_, err = v.udpConn.Write(sb)
	if err != nil {
		v.log(LogWarning, "udp write error to %s, %s", addr.String(), err)
		return
	}

	// Create a 74 byte array and listen for the initial handshake response
	// from Discord.  Once we get it parse the IP and PORT information out
	// of the response.  This should be our public IP and PORT as Discord
	// saw us.
	rb := make([]byte, 74)
	rChan := v.udpReadBackground(rb)
	select {
	case <-time.After(time.Second * 5):
		go func() {
			// empty the channel
			<-rChan
		}()
		v.log(LogError, "timed out waiting for handshake after 5 seconds")
		return
	case err = <-rChan:
	}

	if err != nil {
		v.log(LogWarning, "udp read error, %s, %s", addr.String(), err)
		return
	}

	// Loop over position 4 through 20 to grab the IP address
	// Should never be beyond position 20.
	var ip string
	for i := 8; i < len(rb)-2; i++ {
		if rb[i] == 0 {
			break
		}
		ip += string(rb[i])
	}

	// Grab port from position 72 and 73
	port := binary.BigEndian.Uint16(rb[len(rb)-2:])

	// Select the best mode
	encryptionMode := ""
	for _, mode := range v.op2.Modes {
		switch mode {
		case "aead_aes256_gcm_rtpsize":
			encryptionMode = mode
		case "aead_xchacha20_poly1305_rtpsize":
			if encryptionMode == "" {
				encryptionMode = mode
			}
		}
	}

	// Take the data from above and send it back to Discord to finalize
	// the UDP connection handshake.
	data := voiceUDPOp{1, voiceUDPD{
		Protocol:               "udp",
		Data:                   voiceUDPData{ip, port, VoiceMode(encryptionMode)},
		MaxDAVEProtocolVersion: 1,
	}}

	v.log(LogInformational, "External IP: %s, Port: %d", ip, port)
	v.log(LogInformational, "Selected mode: %s (Available: %v)", encryptionMode, v.op2.Modes)

	v.wsMutex.Lock()
	err = v.wsConn.WriteJSON(data)
	v.wsMutex.Unlock()
	if err != nil {
		v.log(LogWarning, "udp write error, %#v, %s", data, err)
		return
	}

	// start udpKeepAlive
	go v.udpKeepAlive(v.udpConn, v.close, 5*time.Second)
	// TODO: find a way to check that it fired off okay

	return
}

func (v *VoiceConnection) udpReadBackground(dstBuf []byte) chan error {
	c := make(chan error)
	go func() {

		rlen, _, err := v.udpConn.ReadFromUDP(dstBuf)
		if err != nil {
			c <- err
			return
		}

		if rlen < len(dstBuf) {
			c <- errors.New("received udp packet too small")
			return
		}

		c <- nil
	}()

	return c
}

// udpKeepAlive sends a udp packet to keep the udp connection open
// This is still a bit of a "proof of concept"
func (v *VoiceConnection) udpKeepAlive(udpConn *net.UDPConn, close <-chan struct{}, i time.Duration) {

	if udpConn == nil || close == nil {
		return
	}

	var err error
	var sequence uint64

	packet := make([]byte, 8)

	ticker := time.NewTicker(i)
	defer ticker.Stop()
	for {

		binary.LittleEndian.PutUint64(packet, sequence)
		sequence++

		_, err = udpConn.Write(packet)
		if err != nil {
			v.log(LogError, "write error, %s", err)
			return
		}

		select {
		case <-ticker.C:
			// continue loop and send keepalive
		case <-close:
			return
		}
	}
}

// opusSender will listen on the given channel and send any
// pre-encoded opus audio to Discord.  Supposedly.
func (v *VoiceConnection) opusSender(udpConn *net.UDPConn, close <-chan struct{}, opus <-chan []byte, rate, size int) {

	if udpConn == nil || close == nil {
		return
	}

	var sequence uint16
	var timestamp uint32
	var recvbuf []byte
	var ok bool
	udpHeader := make([]byte, 12)
	nonce := make([]byte, v.aead.NonceSize())

	// build the parts that don't change in the udpHeader
	udpHeader[0] = 0x80
	udpHeader[1] = 0x78
	binary.BigEndian.PutUint32(udpHeader[8:], v.op2.SSRC)

	// start a send loop that loops until buf chan is closed
	ticker := time.NewTicker(time.Millisecond * time.Duration(size/(rate/1000)))
	defer ticker.Stop()
	for i := uint32(0); ; i++ {

		// Get data from chan.  If chan is closed, return.
		select {
		case <-close:
			return
		case recvbuf, ok = <-opus:
			if !ok {
				return
			}
			// else, continue loop
		}

		v.RLock()
		daveActive := v.dave != nil && v.dave.CanEncrypt()
		speaking := v.speaking
		v.RUnlock()

		if !speaking {
			err := v.Speaking(true)
			if err != nil {
				v.log(LogError, "error sending speaking packet, %s", err)
			}
		}

		// Add sequence and timestamp to udpPacket
		binary.BigEndian.PutUint16(udpHeader[2:], sequence)
		binary.BigEndian.PutUint32(udpHeader[4:], timestamp)

		if daveActive {
			encrypted, err := v.dave.EncryptFrame(recvbuf)
			if err != nil {
				v.log(LogError, "DAVE encrypt error: %s", err)
			} else {
				recvbuf = encrypted
			}
		}

		binary.LittleEndian.PutUint32(nonce, i)
		sendbuf := make([]byte, len(udpHeader), len(udpHeader)+len(nonce)+len(recvbuf)+v.aead.Overhead())
		copy(sendbuf, udpHeader)
		v.RLock()
		sendbuf = v.aead.Seal(sendbuf, nonce, recvbuf, udpHeader)
		v.RUnlock()
		sendbuf = append(sendbuf, nonce[:4]...)

		// block here until we're exactly at the right time :)
		// Then send rtp audio packet to Discord over UDP
		select {
		case <-close:
			return
		case <-ticker.C:
			// continue
		}
		_, err := udpConn.Write(sendbuf)

		if err != nil {
			v.log(LogError, "udp write error, %s", err)
			v.log(LogDebug, "voice struct: %#v\n", v)
			return
		}

		// don't care if it overflows because it is already defined in Go spec
		// https://go.dev/ref/spec#Integer_overflow
		sequence++
		timestamp += uint32(size)
	}
}

// A Packet contains the headers and content of a received voice packet.
type Packet struct {
	Flags       byte // first byte of RTP header
	PayloadType byte // second byte of RTP header
	Sequence    uint16
	Timestamp   uint32
	SSRC        uint32
	CSRC        []uint32
	Extension   []byte // RTP header extension with extension header, can be nil
	Opus        []byte
}

// opusReceiver listens on the UDP socket for incoming packets
// and sends them across the given channel
// NOTE :: This function may change names later.
func (v *VoiceConnection) opusReceiver(udpConn *net.UDPConn, close <-chan struct{}, c chan *Packet) {

	if udpConn == nil || close == nil {
		return
	}

	recvbuf := make([]byte, 2048)
	nonce := make([]byte, v.aead.NonceSize())

	for {
		rlen, err := udpConn.Read(recvbuf)
		if err != nil {
			// Detect if we have been closed manually. If a Close() has already
			// happened, the udp connection we are listening on will be different
			// to the current session.
			v.RLock()
			sameConnection := v.udpConn == udpConn
			v.RUnlock()
			if sameConnection {

				v.log(LogError, "udp read error, %s, %s", v.endpoint, err)
				v.log(LogDebug, "voice struct: %#v\n", v)

				go v.reconnect(nil)
			}
			return
		}

		select {
		case <-close:
			return
		default:
			// continue loop
		}

		// For now, skip anything except audio.
		if rlen < 12 || (recvbuf[0] != 0x80 && recvbuf[0] != 0x90) {
			continue
		}

		// build a audio packet struct
		p := Packet{}
		p.Flags = recvbuf[0]
		p.PayloadType = recvbuf[1]
		extensionExist := (p.Flags & 0x10) != 0 // RFC 3550 5.1
		csrcCount := (p.Flags & 0x0f)           // RFC 3550 5.1
		p.Sequence = binary.BigEndian.Uint16(recvbuf[2:4])
		p.Timestamp = binary.BigEndian.Uint32(recvbuf[4:8])
		p.SSRC = binary.BigEndian.Uint32(recvbuf[8:12])
		p.CSRC = make([]uint32, csrcCount)
		for i := range p.CSRC {
			p.CSRC[i] = binary.BigEndian.Uint32(recvbuf[12+4*i : 12+4*(i+1)])
		}
		plainLength := 12 + 4*int(csrcCount)
		if extensionExist {
			plainLength += 4
		}

		// decrypt opus data
		copy(nonce, recvbuf[rlen-4:rlen])

		v.RLock()
		p.Opus, err = v.aead.Open(recvbuf[plainLength:plainLength], nonce, recvbuf[plainLength:rlen-4], recvbuf[:plainLength])
		v.RUnlock()
		if err != nil {
			v.log(LogInformational, "failed to open udp packet, %v", err)
			continue
		}

		if extensionExist {
			extensionBegin := 12 + 4*int(csrcCount)
			extensionLength := binary.BigEndian.Uint16(recvbuf[extensionBegin+2 : extensionBegin+4])
			p.Extension = recvbuf[extensionBegin : extensionBegin+4+int(extensionLength)*4]
			p.Opus = p.Opus[int(extensionLength)*4:]
		}

		v.RLock()
		dave := v.dave
		v.RUnlock()
		if dave != nil {
			decrypted, err := dave.DecryptFrame(p.SSRC, p.Opus)
			if err != nil {
				v.log(LogDebug, "DAVE decrypt error for SSRC %d: %s", p.SSRC, err)
				continue
			}
			p.Opus = decrypted
		}

		if c != nil {
			select {
			case c <- &p:
			case <-close:
				return
			}
		}
	}
}

// Reconnect will close down a voice connection then immediately try to
// reconnect to that session.
// NOTE : This func is messy and a WIP while I find what works.
// It will be cleaned up once a proven stable option is flushed out.
// aka: this is ugly shit code, please don't judge too harshly.
func (v *VoiceConnection) reconnect(newGWConn *GatewayConnection) {

	v.log(LogInformational, "called")

	v.Lock()
	if v.reconnecting {
		v.log(LogInformational, "already reconnecting to channel %d, exiting", v.ChannelID)
		v.Unlock()
		return
	}
	v.reconnecting = true

	if newGWConn != nil {
		v.gatewayConn = newGWConn
	}
	v.Unlock()

	defer func() {
		v.Lock()
		v.reconnecting = false
		v.Unlock()
	}()

	// Close any currently open connections
	v.Close()

	wait := time.Duration(1)
	for {

		<-time.After(wait * time.Second)
		wait *= 2
		if wait > 600 {
			wait = 600
		}

		gwStatus := v.gatewayConn.Status()
		if gwStatus == GatewayStatusDisconnected {
			v.log(LogError, "Gateway closed, can't reconnect voice")
			return
		}
		v.log(LogError, "status: %d", gwStatus)
		if gwStatus != GatewayStatusReady {
			v.log(LogInformational, "cannot reconnect to channel %d with unready gateway connection: %d", v.ChannelID, gwStatus)

			continue
		}

		v.log(LogInformational, "trying to reconnect to channel %d", v.ChannelID)

		_, err := v.gatewayConn.manager.ChannelVoiceJoin(v.GuildID, v.ChannelID, v.mute, v.deaf)
		if err == nil {
			v.log(LogInformational, "successfully reconnected to channel %d", v.ChannelID)
			return
		}

		v.log(LogInformational, "error reconnecting to channel %d, %s", v.ChannelID, err)

		// if the reconnect above didn't work lets just send a disconnect
		// packet to reset things.
		// Send a OP4 with a nil channel to disconnect
		strGID := StrID(v.GuildID)
		data := outgoingEvent{
			Operation: GatewayOPVoiceStateUpdate,
			Data:      voiceChannelJoinData{&strGID, nil, true, true, 1},
		}

		v.Lock()
		v.gatewayConn.writer.Queue(data)
		v.Unlock()
	}
}

func (v *VoiceConnection) handleDAVEBinary(message []byte) {
	if len(message) < 3 {
		v.log(LogWarning, "DAVE binary message too short: %d bytes", len(message))
		return
	}

	opcode := message[2]
	payload := message[3:]
	v.log(LogDebug, "DAVE binary opcode=%d len=%d", opcode, len(payload))

	switch opcode {
	case 25: // EXTERNAL_SENDER_PACKAGE
		v.RLock()
		dave := v.dave
		v.RUnlock()
		if dave != nil {
			if err := dave.HandleExternalSenderPackage(payload); err != nil {
				v.log(LogError, "DAVE external sender package failed: %s", err)
			}
		}

	case 27: // PROPOSALS (ignored for now)
		v.log(LogDebug, "DAVE proposals (%d bytes), ignoring", len(payload))

	case 29: // COMMIT/INVALID_COMMIT
		if len(payload) < 2 {
			v.log(LogWarning, "DAVE commit payload too short")
			return
		}
		transitionID := binary.BigEndian.Uint16(payload[0:2])
		v.log(LogInformational, "DAVE commit transition_id=%d, requesting re-Welcome", transitionID)

		v.RLock()
		dave := v.dave
		v.RUnlock()
		if dave == nil {
			return
		}

		v.sendDAVEInvalidCommitWelcome(transitionID)

		kpData, err := dave.ResetForReWelcome()
		if err != nil {
			v.log(LogError, "DAVE reset for re-Welcome failed: %s", err)
			return
		}
		v.sendDAVEKeyPackageBinary(kpData)

		v.Lock()
		v.pendingReWelcome = true
		v.Unlock()

	case 30: // WELCOME
		if len(payload) < 2 {
			v.log(LogWarning, "DAVE welcome payload too short")
			return
		}
		transitionID := binary.BigEndian.Uint16(payload[0:2])
		welcomeData := payload[2:]

		v.log(LogInformational, "DAVE welcome (%d bytes) transition_id=%d", len(welcomeData), transitionID)
		v.RLock()
		dave := v.dave
		v.RUnlock()
		if dave == nil {
			v.log(LogWarning, "DAVE welcome received but no session")
			return
		}

		if err := dave.HandleWelcome(welcomeData); err != nil {
			v.log(LogError, "DAVE welcome processing failed: %s", err)
			return
		}

		if err := dave.DeriveSenderKey(); err != nil {
			v.log(LogError, "DAVE sender key derivation failed: %s", err)
			return
		}

		dave.HandlePrepareTransition(transitionID, 1)
		v.log(LogInformational, "DAVE encryption prepared after Welcome")

		v.sendDAVEReadyForTransition(transitionID)

	default:
		v.log(LogDebug, "DAVE unknown binary opcode %d (%d bytes)", opcode, len(payload))
	}
}

func (v *VoiceConnection) sendDAVEKeyPackageBinary(kpData []byte) {
	v.log(LogInformational, "DAVE sending key package (%d bytes)", len(kpData))
	binMsg := make([]byte, 1+len(kpData))
	binMsg[0] = 26 // KEY_PACKAGE opcode
	copy(binMsg[1:], kpData)

	v.wsMutex.Lock()
	defer v.wsMutex.Unlock()
	if v.wsConn != nil {
		if err := v.wsConn.WriteMessage(websocket.BinaryMessage, binMsg); err != nil {
			v.log(LogError, "DAVE key package send failed: %s", err)
		}
	}
}

func (v *VoiceConnection) sendDAVEReadyForTransition(transitionID uint16) {
	v.log(LogDebug, "DAVE sending ready_for_transition id=%d", transitionID)

	type readyData struct {
		TransitionID uint16 `json:"transition_id"`
	}
	type readyOp struct {
		Op   int       `json:"op"`
		Data readyData `json:"d"`
	}

	v.wsMutex.Lock()
	defer v.wsMutex.Unlock()
	if v.wsConn != nil {
		if err := v.wsConn.WriteJSON(readyOp{23, readyData{transitionID}}); err != nil {
			v.log(LogError, "DAVE ready_for_transition send failed: %s", err)
		}
	}
}

func (v *VoiceConnection) sendDAVEInvalidCommitWelcome(transitionID uint16) {
	v.log(LogInformational, "DAVE sending invalid_commit_welcome id=%d", transitionID)

	type invalidData struct {
		TransitionID uint16 `json:"transition_id"`
	}
	type invalidOp struct {
		Op   int         `json:"op"`
		Data invalidData `json:"d"`
	}

	v.wsMutex.Lock()
	defer v.wsMutex.Unlock()
	if v.wsConn != nil {
		if err := v.wsConn.WriteJSON(invalidOp{31, invalidData{transitionID}}); err != nil {
			v.log(LogError, "DAVE invalid_commit_welcome send failed: %s", err)
		}
	}
}

func (v *VoiceConnection) handleDAVEPrepareTransition(data json.RawMessage) {
	var op21 struct {
		TransitionID    uint16 `json:"transition_id"`
		ProtocolVersion int    `json:"protocol_version"`
	}
	if err := json.Unmarshal(data, &op21); err != nil {
		v.log(LogError, "OP21 unmarshal error: %s", err)
		return
	}

	v.Lock()
	dave := v.dave
	v.Unlock()

	if dave != nil {
		dave.HandlePrepareTransition(op21.TransitionID, op21.ProtocolVersion)
		v.log(LogInformational, "DAVE prepare transition %d, version %d", op21.TransitionID, op21.ProtocolVersion)

		// Acknowledge prepare transition
		type op23Data struct {
			TransitionID uint16 `json:"transition_id"`
		}
		type op23 struct {
			Op   int      `json:"op"`
			Data op23Data `json:"d"`
		}
		v.wsMutex.Lock()
		v.wsConn.WriteJSON(op23{23, op23Data{op21.TransitionID}})
		v.wsMutex.Unlock()
	}
}

func (v *VoiceConnection) handleDAVEExecuteTransition(data json.RawMessage) {
	var op22 struct {
		TransitionID uint16 `json:"transition_id"`
	}
	if err := json.Unmarshal(data, &op22); err != nil {
		v.log(LogError, "OP22 unmarshal error: %s", err)
		return
	}

	v.Lock()
	dave := v.dave
	v.Unlock()

	if dave != nil {
		if err := dave.HandleExecuteTransition(op22.TransitionID); err != nil {
			v.log(LogError, "DAVE execute transition error: %s", err)
			return
		}
		v.log(LogInformational, "DAVE execute transition %d", op22.TransitionID)
	}
}

func (v *VoiceConnection) handleDAVEPrepareEpoch(data json.RawMessage) {
	var op24 struct {
		Epoch           uint64 `json:"epoch"`
		ProtocolVersion int    `json:"protocol_version"`
	}
	if err := json.Unmarshal(data, &op24); err != nil {
		v.log(LogError, "OP24 unmarshal error: %s", err)
		return
	}

	v.Lock()
	dave := v.dave
	v.Unlock()

	if dave != nil {
		kp, err := dave.HandlePrepareEpoch(op24.Epoch, op24.ProtocolVersion)
		if err != nil {
			v.log(LogError, "DAVE prepare epoch error: %s", err)
			return
		}
		v.log(LogInformational, "DAVE prepare epoch %d, version %d", op24.Epoch, op24.ProtocolVersion)

		if kp != nil {
			v.sendDAVEKeyPackageBinary(kp)
		}
	}
}
