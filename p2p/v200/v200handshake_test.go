/*
 * @file
 * @copyright defined in aergo/LICENSE.txt
 */

package v200

import (
	"context"
	"encoding/hex"
	"fmt"
	"reflect"
	"testing"

	"github.com/aergoio/aergo-lib/log"
	"github.com/aergoio/aergo/config"
	"github.com/aergoio/aergo/p2p/p2pcommon"
	"github.com/aergoio/aergo/p2p/p2pkey"
	"github.com/aergoio/aergo/p2p/p2pmock"
	"github.com/aergoio/aergo/p2p/p2putil"
	"github.com/aergoio/aergo/types"
	"github.com/golang/mock/gomock"
)

var (
	myChainID, newVerChainID, theirChainID       *types.ChainID
	myChainBytes, newVerChainBytes, theirChainBytes []byte
	samplePeerID, _                      = types.IDB58Decode("16Uiu2HAmFqptXPfcdaCdwipB2fhHATgKGVFVPehDAPZsDKSU7jRm")
	dummyBlockHash, _                    = hex.DecodeString("4f461d85e869ade8a0544f8313987c33a9c06534e50c4ad941498299579bd7ac")
	dummyBlockHeight              uint64 = 100215

	dummyGenHash = []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	diffGenesis = []byte{0xff, 0xfe, 0xfd, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
)

func init() {
	myChainID = types.NewChainID()
	myChainID.Magic = "itSmain1"
	myChainBytes, _ = myChainID.Bytes()

	newVerChainID = types.NewChainID()
	newVerChainID.Read(myChainBytes)
	newVerChainID.Version = 3
	newVerChainBytes, _ = newVerChainID.Bytes()

	theirChainID = types.NewChainID()
	theirChainID.Read(myChainBytes)
	theirChainID.Magic = "itsdiff2"
	theirChainBytes, _ = theirChainID.Bytes()

	sampleKeyFile := "../../test/sample.key"
	baseCfg := &config.BaseConfig{AuthDir: "test"}
	p2pCfg := &config.P2PConfig{NPKey: sampleKeyFile}
	p2pkey.InitNodeInfo(baseCfg, p2pCfg, "0.0.1-test", log.NewLogger("v200.test"))
}

func TestDeepEqual(t *testing.T) {
	b1, _ := myChainID.Bytes()
	b2 := make([]byte, len(b1), len(b1)<<1)
	copy(b2, b1)

	s1 := &types.Status{ChainID: b1}
	s2 := &types.Status{ChainID: b2}

	if !reflect.DeepEqual(s1, s2) {
		t.Errorf("byte slice cant do DeepEqual! %v, %v", b1, b2)
	}

}

func TestV200StatusHS_doForOutbound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	logger := log.NewLogger("test")
	mockActor := p2pmock.NewMockActorService(ctrl)
	mockCA := p2pmock.NewMockChainAccessor(ctrl)
	mockPM := p2pmock.NewMockPeerManager(ctrl)

	fc := newFC(*myChainID, 10000,20000,dummyBlockHeight+100)
	localChainID := *fc.getChainID(dummyBlockHeight)
	localChainBytes, _ := localChainID.Bytes()
	oldChainID := fc.getChainID(10000)
	oldChainBytes, _ := oldChainID.Bytes()
	newChainID := fc.getChainID(600000)
	newChainBytes, _ := newChainID.Bytes()

	diffBlockNo := dummyBlockHeight+100000
	dummyMeta := p2pcommon.NewMetaWith1Addr(samplePeerID, "dummy.aergo.io", 7846)
	dummyAddr := dummyMeta.ToPeerAddress()
	mockPM.EXPECT().SelfMeta().Return(dummyMeta).AnyTimes()
	dummyBlock := &types.Block{Hash: dummyBlockHash, Header: &types.BlockHeader{BlockNo: dummyBlockHeight}}
	mockActor.EXPECT().GetChainAccessor().Return(mockCA).AnyTimes()
	mockCA.EXPECT().GetBestBlock().Return(dummyBlock, nil).AnyTimes()

	dummyGenHash := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	diffGenesis := []byte{0xff, 0xfe, 0xfd, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	dummyStatusMsg := &types.Status{ChainID: localChainBytes, Sender: &dummyAddr, Genesis: dummyGenHash, BestBlockHash:dummyBlockHash, BestHeight:dummyBlockHeight}
	diffGenesisStatusMsg := &types.Status{ChainID: localChainBytes, Sender: &dummyAddr, Genesis: diffGenesis}
	nilGenesisStatusMsg := &types.Status{ChainID: localChainBytes, Sender: &dummyAddr, Genesis: nil}
	nilSenderStatusMsg := &types.Status{ChainID: localChainBytes, Sender: nil, Genesis: dummyGenHash}
	diffStatusMsg := &types.Status{ChainID: theirChainBytes, Sender: &dummyAddr, Genesis: dummyGenHash, BestHeight:diffBlockNo}
	olderStatusMsg := &types.Status{ChainID: oldChainBytes, Sender: &dummyAddr, Genesis: dummyGenHash, BestHeight:10000}
	newerStatusMsg := &types.Status{ChainID: newChainBytes, Sender: &dummyAddr, Genesis: dummyGenHash, BestHeight:600000}
	diffVersionStatusMsg := &types.Status{ChainID: newVerChainBytes, Sender: &dummyAddr, Genesis: dummyGenHash}

	tests := []struct {
		name       string
		readReturn *types.Status
		readError  error
		writeError error
		want       *types.Status
		wantErr    bool
		wantGoAway bool
	}{
		{"TSuccess", dummyStatusMsg, nil, nil, dummyStatusMsg, false, false},
		{"TOldChain", olderStatusMsg, nil, nil, dummyStatusMsg, false, false},
		{"TNewChain", newerStatusMsg, nil, nil, dummyStatusMsg, false, false},
		{"TUnexpMsg", nil, nil, nil, nil, true, true},
		{"TRFail", dummyStatusMsg, fmt.Errorf("failed"), nil, nil, true, true},
		{"TRNoSender", nilSenderStatusMsg, nil, nil, nil, true, true},
		{"TWFail", dummyStatusMsg, nil, fmt.Errorf("failed"), nil, true, false},
		{"TDiffChain", diffStatusMsg, nil, nil, nil, true, true},
		{"TNilGenesis", nilGenesisStatusMsg, nil, nil, nil, true, true},
		{"TDiffGenesis", diffGenesisStatusMsg, nil, nil, nil, true, true},
		{"TDiffChainVersion", diffVersionStatusMsg, nil, nil, nil, true, true},

		//{"TSuccess", dummyStatusMsg, nil, nil, dummyStatusMsg, false, false},
		//{"TUnexpMsg", nil, nil, nil, nil, true, true},
		//{"TRFail", dummyStatusMsg, fmt.Errorf("failed"), nil, nil, true, true},
		//{"TRNoSender", nilSenderStatusMsg, nil, nil, nil, true, true},
		//{"TWFail", dummyStatusMsg, nil, fmt.Errorf("failed"), nil, true, false},
		//{"TDiffChain", diffStatusMsg, nil, nil, nil, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dummyReader := p2pmock.NewMockReadWriteCloser(ctrl)
			mockRW := p2pmock.NewMockMsgReadWriter(ctrl)
			mockVM := p2pmock.NewMockVersionedManager(ctrl)
			var containerMsg *p2pcommon.MessageValue
			if tt.readReturn != nil {
				containerMsg = p2pcommon.NewSimpleMsgVal(p2pcommon.StatusRequest, p2pcommon.NewMsgID())
				statusBytes, _ := p2putil.MarshalMessageBody(tt.readReturn)
				containerMsg.SetPayload(statusBytes)
			} else {
				containerMsg = p2pcommon.NewSimpleMsgVal(p2pcommon.AddressesRequest, p2pcommon.NewMsgID())
			}
			mockRW.EXPECT().ReadMsg().Return(containerMsg, tt.readError).AnyTimes()
			if tt.wantGoAway {
				mockRW.EXPECT().WriteMsg(&MsgMatcher{p2pcommon.GoAway}).Return(tt.writeError)
			}
			mockRW.EXPECT().WriteMsg(&MsgMatcher{p2pcommon.StatusRequest}).Return(tt.writeError).MaxTimes(1)
			mockVM.EXPECT().GetBestChainID().Return(myChainID).AnyTimes()
			mockVM.EXPECT().GetChainID(gomock.Any()).DoAndReturn(fc.getChainID).AnyTimes()

			h := NewV200VersionedHS(mockPM, mockActor, logger, mockVM, samplePeerID, dummyReader, dummyGenHash)
			h.msgRW = mockRW
			got, err := h.DoForOutbound(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("PeerHandshaker.handshakeOutboundPeer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != nil && tt.want != nil {
				if !reflect.DeepEqual(got.ChainID, tt.want.ChainID) {
					fmt.Printf("got:(%d) %s \n", len(got.ChainID), hex.EncodeToString(got.ChainID))
					fmt.Printf("got:(%d) %s \n", len(tt.want.ChainID), hex.EncodeToString(tt.want.ChainID))
					t.Errorf("PeerHandshaker.handshakeOutboundPeer() = %v, want %v", got.ChainID, tt.want.ChainID)
				}
			} else if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("PeerHandshaker.handshakeOutboundPeer() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestV200VersionedHS_DoForInbound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// t.SkipNow()
	logger := log.NewLogger("test")
	mockActor := p2pmock.NewMockActorService(ctrl)
	mockCA := p2pmock.NewMockChainAccessor(ctrl)
	mockPM := p2pmock.NewMockPeerManager(ctrl)

	fc := newFC(*myChainID, 10000,20000,dummyBlockHeight+100)

	dummyMeta := p2pcommon.NewMetaWith1Addr(samplePeerID,"dummy.aergo.io", 7846)
	dummyAddr := dummyMeta.ToPeerAddress()
	mockPM.EXPECT().SelfMeta().Return(dummyMeta).AnyTimes()
	dummyBlock := &types.Block{Hash: dummyBlockHash, Header: &types.BlockHeader{BlockNo: dummyBlockHeight}}
	//dummyBlkRsp := message.GetBestBlockRsp{Block: dummyBlock}
	mockActor.EXPECT().GetChainAccessor().Return(mockCA).AnyTimes()
	mockCA.EXPECT().GetBestBlock().Return(dummyBlock, nil).AnyTimes()

	dummyGenHash := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	diffGenHash := []byte{0xff, 0xfe, 0xfd, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	dummyStatusMsg := &types.Status{ChainID: myChainBytes, Sender: &dummyAddr, Genesis: dummyGenHash}
	diffGenesisStatusMsg := &types.Status{ChainID: myChainBytes, Sender: &dummyAddr, Genesis: diffGenHash}
	nilGenesisStatusMsg := &types.Status{ChainID: myChainBytes, Sender: &dummyAddr, Genesis: nil}
	nilSenderStatusMsg := &types.Status{ChainID: myChainBytes, Sender: nil, Genesis: dummyGenHash}
	diffStatusMsg := &types.Status{ChainID: theirChainBytes, Sender: &dummyAddr, Genesis: dummyGenHash}
	diffVersionStatusMsg := &types.Status{ChainID: newVerChainBytes, Sender: &dummyAddr, Genesis: dummyGenHash}
	tests := []struct {
		name       string
		readReturn *types.Status
		readError  error
		writeError error
		want       *types.Status
		wantErr    bool
		wantGoAway bool
	}{
		{"TSuccess", dummyStatusMsg, nil, nil, dummyStatusMsg, false, false},
		{"TUnexpMsg", nil, nil, nil, nil, true, true},
		{"TRFail", dummyStatusMsg, fmt.Errorf("failed"), nil, nil, true, true},
		{"TRNoSender", nilSenderStatusMsg, nil, nil, nil, true, true},
		{"TWFail", dummyStatusMsg, nil, fmt.Errorf("failed"), nil, true, false},
		{"TDiffChain", diffStatusMsg, nil, nil, nil, true, true},
		{"TNilGenesis", nilGenesisStatusMsg, nil, nil, nil, true, true},
		{"TDiffGenesis", diffGenesisStatusMsg, nil, nil, nil, true, true},
		{"TDiffChainVersion", diffVersionStatusMsg, nil, nil, nil, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dummyReader := p2pmock.NewMockReadWriteCloser(ctrl)
			mockRW := p2pmock.NewMockMsgReadWriter(ctrl)
			mockVM := p2pmock.NewMockVersionedManager(ctrl)

			containerMsg := &p2pcommon.MessageValue{}
			if tt.readReturn != nil {
				containerMsg = p2pcommon.NewSimpleMsgVal(p2pcommon.StatusRequest, p2pcommon.NewMsgID())
				statusBytes, _ := p2putil.MarshalMessageBody(tt.readReturn)
				containerMsg.SetPayload(statusBytes)
			} else {
				containerMsg = p2pcommon.NewSimpleMsgVal(p2pcommon.AddressesRequest, p2pcommon.NewMsgID())
			}

			mockRW.EXPECT().ReadMsg().Return(containerMsg, tt.readError).AnyTimes()
			if tt.wantGoAway {
				mockRW.EXPECT().WriteMsg(&MsgMatcher{p2pcommon.GoAway}).Return(tt.writeError)
			}
			mockRW.EXPECT().WriteMsg(gomock.Any()).Return(tt.writeError).AnyTimes()
			mockVM.EXPECT().GetBestChainID().Return(myChainID).AnyTimes()
			mockVM.EXPECT().GetChainID(gomock.Any()).DoAndReturn(fc.getChainID).AnyTimes()

			h := NewV200VersionedHS(mockPM, mockActor, logger, mockVM, samplePeerID, dummyReader, dummyGenHash)
			h.msgRW = mockRW
			got, err := h.DoForInbound(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("PeerHandshaker.DoForInbound() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != nil && tt.want != nil {
				if !reflect.DeepEqual(got.ChainID, tt.want.ChainID) {
					fmt.Printf("got:(%d) %s \n", len(got.ChainID), hex.EncodeToString(got.ChainID))
					fmt.Printf("got:(%d) %s \n", len(tt.want.ChainID), hex.EncodeToString(tt.want.ChainID))
					t.Errorf("PeerHandshaker.DoForInbound() = %v, want %v", got.ChainID, tt.want.ChainID)
				}
			} else if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("PeerHandshaker.DoForInbound() = %v, want %v", got, tt.want)
			}
		})
	}
}

func ___TestV200StatusHS_DoForInbound_(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// t.SkipNow()
	logger := log.NewLogger("test")
	mockActor := p2pmock.NewMockActorService(ctrl)
	mockCA := p2pmock.NewMockChainAccessor(ctrl)
	mockPM := p2pmock.NewMockPeerManager(ctrl)

	dummyMeta := p2pcommon.NewMetaWith1Addr(samplePeerID, "dummy.aergo.io", 7846)
	dummyAddr := dummyMeta.ToPeerAddress()
	mockPM.EXPECT().SelfMeta().Return(dummyMeta).AnyTimes()
	dummyBlock := &types.Block{Hash: dummyBlockHash, Header: &types.BlockHeader{BlockNo: dummyBlockHeight}}
	//dummyBlkRsp := message.GetBestBlockRsp{Block: dummyBlock}
	mockActor.EXPECT().GetChainAccessor().Return(mockCA).AnyTimes()
	mockCA.EXPECT().GetBestBlock().Return(dummyBlock, nil).AnyTimes()

	dummyStatusMsg := &types.Status{ChainID: myChainBytes, Sender: &dummyAddr}
	nilSenderStatusMsg := &types.Status{ChainID: myChainBytes, Sender: nil}
	diffStatusMsg := &types.Status{ChainID: theirChainBytes, Sender: &dummyAddr}
	tests := []struct {
		name       string
		readReturn *types.Status
		readError  error
		writeError error
		want       *types.Status
		wantErr    bool
		wantGoAway bool
	}{
		{"TSuccess", dummyStatusMsg, nil, nil, dummyStatusMsg, false, false},
		{"TUnexpMsg", nil, nil, nil, nil, true, true},
		{"TRFail", dummyStatusMsg, fmt.Errorf("failed"), nil, nil, true, true},
		{"TRNoSender", nilSenderStatusMsg, nil, nil, nil, true, true},
		{"TWFail", dummyStatusMsg, nil, fmt.Errorf("failed"), nil, true, false},
		{"TDiffChain", diffStatusMsg, nil, nil, nil, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dummyReader := p2pmock.NewMockReadWriteCloser(ctrl)
			mockRW := p2pmock.NewMockMsgReadWriter(ctrl)
			mockVM := p2pmock.NewMockVersionedManager(ctrl)

			containerMsg := &p2pcommon.MessageValue{}
			if tt.readReturn != nil {
				containerMsg = p2pcommon.NewSimpleMsgVal(p2pcommon.StatusRequest, p2pcommon.NewMsgID())
				statusBytes, _ := p2putil.MarshalMessageBody(tt.readReturn)
				containerMsg.SetPayload(statusBytes)
			} else {
				containerMsg = p2pcommon.NewSimpleMsgVal(p2pcommon.AddressesRequest, p2pcommon.NewMsgID())
			}

			mockRW.EXPECT().ReadMsg().Return(containerMsg, tt.readError).AnyTimes()
			if tt.wantGoAway {
				mockRW.EXPECT().WriteMsg(&MsgMatcher{p2pcommon.GoAway}).Return(tt.writeError)
			}
			mockRW.EXPECT().WriteMsg(&MsgMatcher{p2pcommon.StatusRequest}).Return(tt.writeError).MaxTimes(1)

			h := NewV200VersionedHS(mockPM, mockActor, logger, mockVM, samplePeerID, dummyReader, dummyGenHash)
			h.msgRW = mockRW
			got, err := h.DoForInbound(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("PeerHandshaker.handshakeInboundPeer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != nil && tt.want != nil {
				if !reflect.DeepEqual(got.ChainID, tt.want.ChainID) {
					fmt.Printf("got:(%d) %s \n", len(got.ChainID), hex.EncodeToString(got.ChainID))
					fmt.Printf("got:(%d) %s \n", len(tt.want.ChainID), hex.EncodeToString(tt.want.ChainID))
					t.Errorf("PeerHandshaker.handshakeOutboundPeer() = %v, want %v", got.ChainID, tt.want.ChainID)
				}
			} else if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("PeerHandshaker.handshakeInboundPeer() = %v, want %v", got, tt.want)
			}
		})
	}
}

type MsgMatcher struct {
	sub p2pcommon.SubProtocol
}

func (m MsgMatcher) Matches(x interface{}) bool {
	return x.(p2pcommon.Message).Subprotocol() == m.sub
}

func (m MsgMatcher) String() string {
	return "matcher "+m.sub.String()
}

func Test_createMessage(t *testing.T) {
	type args struct {
		protocolID p2pcommon.SubProtocol
		msgBody    p2pcommon.MessageBody
	}
	tests := []struct {
		name string
		args args
		wantNil bool
	}{
		{"TStatus", args{protocolID:p2pcommon.StatusRequest,msgBody:&types.Status{Version:"11"}}, false},
		{"TGOAway", args{protocolID:p2pcommon.GoAway,msgBody:&types.GoAwayNotice{Message:"test"}}, false},
		{"TNil", args{protocolID:p2pcommon.StatusRequest,msgBody:nil}, true},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := createMessage(tt.args.protocolID, p2pcommon.NewMsgID(), tt.args.msgBody)
			if (got == nil) != tt.wantNil {
				t.Errorf("createMessage() = %v, want nil %v", got, tt.wantNil)
			}
			if got != nil &&  got.Subprotocol() != tt.args.protocolID {
				t.Errorf("status.ProtocolID = %v, want %v", got.Subprotocol() , tt.args.protocolID)
			}
		})
	}
}
