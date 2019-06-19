/*
 * @file
 * @copyright defined in aergo/LICENSE.txt
 */

//go:generate mockgen -source=peermanager.go -package=p2pmock -destination=../p2pmock/mock_peermanager.go
package p2pcommon

import (
	"github.com/aergoio/aergo/message"
	"github.com/aergoio/aergo/types"
)

// PeerManager is internal service that provide peer management
type PeerManager interface {
	AddPeerEventListener(l PeerEventListener)

	Start() error
	Stop() error

	//NetworkTransport
	SelfMeta() PeerMeta
	SelfNodeID() types.PeerID

	AddNewPeer(meta PeerMeta)
	// Remove peer from peer list. Peer dispose relative resources and stop itself, and then call PeerManager.RemovePeer
	RemovePeer(peer RemotePeer)
	UpdatePeerRole(changes []AttrModifier)

	NotifyPeerAddressReceived([]PeerMeta)

	// GetPeer return registered(handshaked) remote peer object. It is thread safe
	GetPeer(ID types.PeerID) (RemotePeer, bool)
	// GetPeers return all registered(handshaked) remote peers. It is thread safe
	GetPeers() []RemotePeer
	GetPeerAddresses(noHidden bool, showSelf bool) []*message.PeerInfo

	GetPeerBlockInfos() []types.PeerBlockInfo
}
