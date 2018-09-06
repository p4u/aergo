package p2p

import (
	"bufio"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aergoio/aergo-lib/log"
	"github.com/aergoio/aergo/message"
	"github.com/aergoio/aergo/types"
	inet "github.com/libp2p/go-libp2p-net"
	"github.com/libp2p/go-libp2p-peer"
	"github.com/libp2p/go-libp2p-protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const testDuration = time.Second >> 1

var samplePeerID peer.ID
var sampleErr error

var logger *log.Logger

func init() {
	logger = log.NewLogger("test")

	samplePeerID, _ = peer.IDB58Decode("16Uiu2HAkvvhjxVm2WE9yFBDdPQ9qx6pX9taF6TTwDNHs8VPi1EeR")
	sampleErr = fmt.Errorf("err in unittest")
}

// TODO refactor rw and modify this test
func TestAergoPeer_RunPeer(t *testing.T) {
	t.SkipNow()
	mockActorServ := new(MockActorService)
	dummyP2PServ := new(MockPeerManager)

	dummyRW := &bufio.ReadWriter{Reader: &bufio.Reader{}}
	target := newRemotePeer(PeerMeta{ID: peer.ID("ddddd")}, dummyP2PServ, mockActorServ,
		logger)
	target.rw = dummyRW

	target.pingDuration = time.Second * 10
	dummyBestBlock := types.Block{Hash: []byte("testHash"), Header: &types.BlockHeader{BlockNo: 1234}}
	mockActorServ.On("requestSync", mock.Anything, mock.AnythingOfType("message.GetBlockRsp")).Return(dummyBestBlock, true)

	go target.runPeer()

	time.Sleep(testDuration)
	target.stop()
}

func TestAergoPeer_writeToPeer(t *testing.T) {
	sampleMeta := PeerMeta{ID: samplePeerID, IPAddress: "192.168.1.2", Port: 7845}

	type args struct {
		StreamResult error
		needSign     bool
		signErr      error
		needResponse bool
		sendErr      error
	}
	type wants struct {
		sendCnt   int
		expReqCnt int
	}
	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{"TNReq1", args{}, wants{1, 0}},
		{"TNReq2", args{needSign: true}, wants{1, 0}},
		{"TNReqWResp1", args{needResponse: true}, wants{1, 1}},
		{"TNReqWResp2", args{needSign: true, needResponse: true}, wants{1, 1}},

		// no sign no error
		{"TFSign1", args{needSign: false, signErr: sampleErr, needResponse: true}, wants{1, 1}},
		// error while signing
		{"TFSign2", args{needSign: true, signErr: sampleErr, needResponse: true}, wants{0, 0}},
		// error while get stream

		{"TFSend1", args{needSign: true, needResponse: true, sendErr: sampleErr}, wants{1, 0}},

		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockActorServ := new(MockActorService)
			mockPeerManager := new(MockPeerManager)
			mockOrder := new(MockMsgOrder)
			dummyRW := &bufio.ReadWriter{Reader: &bufio.Reader{}}
			mockOrder.On("IsNeedSign").Return(tt.args.needSign)
			if tt.args.needSign {
				mockOrder.On("SignWith", mockPeerManager).Return(tt.args.signErr)
			}
			mockOrder.On("IsRequest", mockPeerManager).Return(true)
			mockOrder.On("SendOver", mock.AnythingOfType("*bufio.ReadWriter")).Return(tt.args.sendErr)
			mockOrder.On("GetProtocolID").Return(pingRequest)
			mockOrder.On("GetRequestID").Return("test_req")
			mockOrder.On("ResponseExpected").Return(tt.args.needResponse)

			p := newRemotePeer(sampleMeta, mockPeerManager, mockActorServ, logger)
			p.rw = dummyRW
			p.setState(types.RUNNING)
			go p.runWrite()

			p.writeToPeer(mockOrder)

			// FIXME wait in more relaiable way
			time.Sleep(50 * time.Millisecond)
			p.closeWrite <- struct{}{}
			mockOrder.AssertNumberOfCalls(t, "SendOver", tt.wants.sendCnt)
			assert.Equal(t, tt.wants.expReqCnt, len(p.requests))
		})
	}
}

func TestRemotePeer_sendPing(t *testing.T) {
	selfPeerID, _ := peer.IDB58Decode("16Uiu2HAmFqptXPfcdaCdwipB2fhHATgKGVFVPehDAPZsDKSU7jRm")
	sampleSelf := PeerMeta{ID: selfPeerID, IPAddress: "192.168.1.1", Port: 6845}

	sampleMeta := PeerMeta{ID: samplePeerID, IPAddress: "192.168.1.2", Port: 7845}
	dummyBestBlockRsp := message.GetBestBlockRsp{Block: &types.Block{Header: &types.BlockHeader{}}}
	type wants struct {
		wantWrite bool
	}
	tests := []struct {
		name        string
		getBlockErr error
		wants       wants
	}{
		{"TN", nil, wants{wantWrite: true}},
		{"TF", sampleErr, wants{wantWrite: false}},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockActorServ := new(MockActorService)
			mockPeerManager := new(MockPeerManager)

			mockActorServ.On("CallRequest", message.ChainSvc, mock.AnythingOfType("*message.GetBestBlock")).Return(dummyBestBlockRsp, tt.getBlockErr)
			mockPeerManager.On("SelfMeta").Return(sampleSelf)

			p := newRemotePeer(sampleMeta, mockPeerManager, mockActorServ, logger)
			go p.sendPing()

			time.Sleep(200 * time.Millisecond)

			actualWrite := false
			select {
			case msg := <-p.write:
				assert.Equal(t, pingRequest, msg.GetProtocolID())
				actualWrite = true
			default:
			}
			assert.Equal(t, tt.wants.wantWrite, actualWrite)
			mockPeerManager.AssertNotCalled(t, "SelfMeta")
			mockActorServ.AssertNumberOfCalls(t, "CallRequest", 1)
		})
	}
}

// TODO sendStatus will be deleted
func IgnoreTestRemotePeer_sendStatus(t *testing.T) {
	selfPeerID, _ := peer.IDB58Decode("16Uiu2HAmFqptXPfcdaCdwipB2fhHATgKGVFVPehDAPZsDKSU7jRm")
	sampleSelf := PeerMeta{ID: selfPeerID, IPAddress: "192.168.1.1", Port: 6845}

	sampleMeta := PeerMeta{ID: samplePeerID, IPAddress: "192.168.1.2", Port: 7845}
	dummyBestBlockRsp := message.GetBestBlockRsp{Block: &types.Block{Header: &types.BlockHeader{}}}
	type wants struct {
		wantWrite bool
	}
	tests := []struct {
		name        string
		getBlockErr error
		wants       wants
	}{
		{"TN", nil, wants{wantWrite: true}},
		{"TF", sampleErr, wants{wantWrite: false}},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockActorServ := new(MockActorService)
			mockPeerManager := new(MockPeerManager)

			mockActorServ.On("CallRequest", message.ChainSvc, mock.AnythingOfType("*message.GetBestBlock")).Return(dummyBestBlockRsp, tt.getBlockErr)
			mockPeerManager.On("SelfMeta").Return(sampleSelf)

			p := newRemotePeer(sampleMeta, mockPeerManager, mockActorServ, logger)
			go p.sendStatus()

			time.Sleep(200 * time.Millisecond)

			actualWrite := false
			select {
			case msg := <-p.write:
				assert.Equal(t, statusRequest, msg.GetProtocolID())
				actualWrite = true
			default:
			}
			assert.Equal(t, tt.wants.wantWrite, actualWrite)
			mockActorServ.AssertNumberOfCalls(t, "CallRequest", 1)
		})
	}
}

func TestRemotePeer_pruneRequests(t *testing.T) {
	sampleMeta := PeerMeta{ID: samplePeerID, IPAddress: "192.168.1.2", Port: 7845}
	tests := []struct {
		name     string
		loglevel string
	}{
		{"T1", "info"},
		//		{"T2", "debug"},
		// TODO: Add test cases.
	}
	// prevLevel := logger.Level()
	// defer logger.SetLevel(prevLevel)
	for _, tt := range tests {
		// logger.SetLevel(tt.loglevel)
		mockActorServ := new(MockActorService)
		mockPeerManager := new(MockPeerManager)
		p := newRemotePeer(sampleMeta, mockPeerManager, mockActorServ, logger)
		t.Run(tt.name, func(t *testing.T) {
			p.requests["r1"] = &pbMessageOrder{message: &types.P2PMessage{Header: &types.MessageData{Id: "r1", Timestamp: time.Now().Add(time.Minute * -61).Unix()}}}
			p.requests["r2"] = &pbMessageOrder{message: &types.P2PMessage{Header: &types.MessageData{Id: "r2", Timestamp: time.Now().Add(time.Minute*-60 - time.Second).Unix()}}}
			p.requests["rn"] = &pbMessageOrder{message: &types.P2PMessage{Header: &types.MessageData{Id: "rn", Timestamp: time.Now().Add(time.Minute * -60).Unix()}}}
			p.pruneRequests()

			assert.Equal(t, 1, len(p.requests))
		})
	}
}

func TestRemotePeer_tryGetStream(t *testing.T) {
	sampleMeta := PeerMeta{ID: samplePeerID, IPAddress: "192.168.1.2", Port: 7845}
	mockStream := new(MockStream)
	type args struct {
		msgID    string
		protocol protocol.ID
		timeout  time.Duration
	}
	tests := []struct {
		name    string
		args    args
		timeout bool
		want    inet.Stream
	}{
		{"TN", args{"m1", "p1", time.Millisecond * 100}, false, mockStream},
		{"TTimeout", args{"m1", "p1", time.Millisecond * 100}, true, nil},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		mockActorServ := new(MockActorService)
		mockPeerManager := new(MockPeerManager)
		if tt.timeout {
			mockPeerManager.On("NewStream", mock.Anything, mock.Anything, mock.Anything).After(time.Second).Return(mockStream, nil)
		} else {
			mockPeerManager.On("NewStream", mock.Anything, mock.Anything, mock.Anything).Return(mockStream, nil)
		}
		t.Run(tt.name, func(t *testing.T) {
			p := newRemotePeer(sampleMeta, mockPeerManager, mockActorServ, logger)
			got := p.tryGetStream(tt.args.msgID, tt.args.protocol, tt.args.timeout)

			assert.Equal(t, got, tt.want)
		})
	}
}

func TestRemotePeer_sendMessage(t *testing.T) {
	sampleMeta := PeerMeta{ID: samplePeerID, IPAddress: "192.168.1.2", Port: 7845}

	type args struct {
		msgID    string
		protocol protocol.ID
		timeout  time.Duration
	}
	tests := []struct {
		name    string
		args    args
		timeout bool
	}{
		{"TSucc", args{"m1", "p1", time.Millisecond * 100}, false},
		{"TTimeout", args{"mtimeout", "p1", time.Millisecond * 100}, true},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		mockActorServ := new(MockActorService)
		mockPeerManager := new(MockPeerManager)
		mockMsg := new(MockMsgOrder)
		mockMsg.On("GetRequestID").Return(tt.args.msgID)
		mockMsg.On("GetProtocolID").Return(newBlockNotice)

		writeCnt := int32(0)
		t.Run(tt.name, func(t *testing.T) {
			wg := &sync.WaitGroup{}
			wg.Add(1)
			wg2 := &sync.WaitGroup{}
			wg2.Add(1)
			p := newRemotePeer(sampleMeta, mockPeerManager, mockActorServ, logger)

			if !tt.timeout {
				go func() {
					wg.Wait()
					for {
						select {
						case ord := <-p.write:
							p.logger.Info().Str("msg_id", ord.GetRequestID()).Msg("Got order")
							atomic.AddInt32(&writeCnt, 1)
							wg2.Done()
							continue
						default:
							return
						}
					}
				}()
			} else {
				wg2.Done()
			}
			wg.Done()
			p.sendMessage(mockMsg)
			wg2.Wait()
			if !tt.timeout {
				assert.Equal(t, int32(1), atomic.LoadInt32(&writeCnt))
			}
		})
	}
}

func TestRemotePeer_handleMsg(t *testing.T) {
	sampleMeta := PeerMeta{ID: samplePeerID, IPAddress: "192.168.1.2", Port: 7845}
	mockMsg := new(MockMsgOrder)
	mockMsg.On("GetRequestID").Return("m1")
	mockMsg.On("GetProtocolID").Return(newBlockNotice)

	type args struct {
		nohandler bool
		parerr    error
		autherr   error
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{"TSucc", args{false, nil, nil}, false},
		{"Tnopro", args{true, nil, nil}, true},
		{"Tparcefail", args{false, fmt.Errorf("not proto"), nil}, true},
		{"Tauthfail", args{false, nil, fmt.Errorf("no permission")}, true},

		// TODO: make later use
		//{"TTimeout", args{false, nil, fmt.Errorf("no permission")}, true},
	}
	for _, tt := range tests {
		mockActorServ := new(MockActorService)
		mockPeerManager := new(MockPeerManager)
		mockMsgHandler := new(MockMessageHandler)
		t.Run(tt.name, func(t *testing.T) {
			msg := &types.P2PMessage{Header: &types.MessageData{Subprotocol: pingRequest.Uint32()}}
			if tt.args.nohandler {
				msg.Header.Subprotocol = 3999999999
			}
			bodyStub := &types.Ping{}
			mockMsgHandler.On("parsePayload", mock.AnythingOfType("[]uint8")).Return(bodyStub, tt.args.parerr)
			mockMsgHandler.On("checkAuth", mock.AnythingOfType("*types.MessageData"), mock.Anything).Return(tt.args.autherr)
			mockMsgHandler.On("handle", mock.AnythingOfType("*types.MessageData"), mock.Anything)

			target := newRemotePeer(sampleMeta, mockPeerManager, mockActorServ, logger)
			target.handlers[pingRequest] = mockMsgHandler

			if err := target.handleMsg(msg); (err != nil) != tt.wantErr {
				t.Errorf("RemotePeer.handleMsg() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.args.nohandler {
				mockMsgHandler.AssertNotCalled(t, "parsePayload", mock.AnythingOfType("[]uint8"))
			} else {
				mockMsgHandler.AssertCalled(t, "parsePayload", mock.AnythingOfType("[]uint8"))
			}
			if tt.args.nohandler || tt.args.parerr != nil {
				mockMsgHandler.AssertNotCalled(t, "checkAuth", mock.Anything, mock.Anything)
			} else {
				mockMsgHandler.AssertCalled(t, "checkAuth", msg.Header, bodyStub)
			}
			if tt.args.nohandler || tt.args.parerr != nil || tt.args.autherr != nil {
				mockMsgHandler.AssertNotCalled(t, "handle", mock.Anything, mock.Anything)
			} else {
				mockMsgHandler.AssertCalled(t, "handle", msg.Header, bodyStub)
			}
		})
	}
}
