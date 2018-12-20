/*
 * @file
 * @copyright defined in aergo/LICENSE.txt
 */

package pmap

import (
	"bufio"
	"fmt"
	"github.com/aergoio/aergo/p2p"
	"github.com/aergoio/aergo/p2p/p2putil"
	"github.com/aergoio/aergo/types"
	"github.com/libp2p/go-libp2p-net"
	"sync"
	"sync/atomic"
	"time"
)

type PeerHealth int

// PeersState
const (
	PeerHealth_GOOD PeerHealth = 0
	PeerHealth_MID PeerHealth = 1
	PeerHealth_BAD PeerHealth = 2
)

type peerState struct {
	*PeerMapService

	meta p2p.PeerMeta
	addr types.PeerAddress

	bestHash   []byte
	bestNo     int64
	lCheckTime time.Time
	contFail   int32
}

func (hc *peerState) health() PeerHealth {
	// TODO make more robust if needed
	switch {
	case atomic.LoadInt32(&hc.contFail) == 0 :
		return PeerHealth_GOOD
	default :
		return PeerHealth_BAD
	}
}

func (hc *peerState) lastCheck() time.Time {
	return hc.lCheckTime
}

func (hc *peerState) check(wg *sync.WaitGroup, timeout time.Duration) {
	defer wg.Done()
	hc.Logger.Debug().Str(p2p.LogPeerID, hc.meta.ID.Pretty()).Msg("staring up healthcheck")
	hc.lCheckTime = time.Now()
	s, err := hc.nt.GetOrCreateStreamWithTTL(hc.meta, PolarisPingSub, PolarisConnectionTTL)
	if err != nil {
		hc.contFail++
		hc.Logger.Debug().Err(err).Msg("Healthcheck failed to get network stream")
		hc.unregisterPeer(hc.meta.ID)
		return
	}
	defer s.Close()

	pc := &pingChecker{peerState:hc,s:s}
	_, err = p2putil.InvokeWithTimer(pc, time.NewTimer(timeout))
	if err != nil {
		hc.unregisterPeer(hc.meta.ID)
	} else if pc.health() == PeerHealth_BAD {
		hc.unregisterPeer(hc.meta.ID)
	}
}

// this method MUST be called in same go routine as AergoPeer.RunPeer()
func (hc *peerState) sendPing(wt p2p.MsgWriter) (p2p.MsgID, error) {
	// find my best block
	ping := &types.Ping{}
	msgID := p2p.NewMsgID()
	pingMsg, err := createV030Message(msgID, EmptyMsgID, p2p.PingRequest, ping)
	if err != nil {
		hc.Logger.Warn().Err(err).Msg("failed to create ping message")
		return EmptyMsgID, err
	}

	err = wt.WriteMsg(pingMsg)
	if err != nil {
		hc.Logger.Warn().Err(err).Msg("failed to write ping message")
		return EmptyMsgID, err
	}

	return msgID, nil
}

// tryAddPeer will do check connecting peer and add. it will return peer meta information received from
// remote peer setup some
func (hc *peerState) receivePingResp(reqID p2p.MsgID, rd p2p.MsgReader) (p2p.Message, *types.Ping, error) {
	resp, err := rd.ReadMsg()
	if err != nil {
		return nil, nil, err
	}
	if resp.Subprotocol() != p2p.PingResponse || reqID != resp.OriginalID() {
		return nil, nil, fmt.Errorf("Not expected response %s : req_id=%s", resp.Subprotocol().String(), resp.OriginalID().String())
	}
	pingResp := &types.Ping{}
	err = p2p.UnmarshalMessage(resp.Payload(), pingResp)
	if err != nil {
		return resp, nil, err
	}

	return resp, pingResp, nil
}


// pingChecker has ttl and will try to
type pingChecker struct {
	*peerState
	s net.Stream
	cancel int32
}

func (pc *pingChecker) DoCall(done chan<- interface{}) {
	defer func(){done<-struct{}{} }()

	rw := p2p.NewV030ReadWriter(bufio.NewReader(pc.s), bufio.NewWriter(pc.s))
	reqID, err := pc.sendPing(rw)
	if err != nil {
		atomic.AddInt32(&pc.contFail,1)
		pc.Logger.Debug().Err(err).Msg("Healthcheck failed to send ping message")
		return
	}
	if atomic.LoadInt32(&pc.cancel) != 0 {
		atomic.AddInt32(&pc.contFail,1)
		return
	}
	_, pingResp, err := pc.receivePingResp(reqID, rw)
	if err != nil {
		atomic.AddInt32(&pc.contFail,1)
		pc.Logger.Debug().Err(err).Msg("Healthcheck failed to receive ping response")
		return
	}
	if atomic.LoadInt32(&pc.cancel) != 0 {
		atomic.AddInt32(&pc.contFail,1)
		return
	}

	pc.Logger.Debug().Str(p2p.LogPeerID, pc.meta.ID.Pretty()).Interface("ping_resp", pingResp).Msg("Healthcheck finished successful")
	atomic.StoreInt32(&pc.contFail,0)
	return

}

func (pc *pingChecker) Cancel() {
	atomic.StoreInt32(&pc.cancel, 1)
}
