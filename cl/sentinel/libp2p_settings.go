package sentinel

import (
	"fmt"
	"math"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pubsubpb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/peer"
)

// determines the decay rate from the provided time period till
// the decayToZero value. Ex: ( 1 -> 0.01)
func (s *Sentinel) scoreDecay(totalDurationDecay time.Duration) float64 {
	numOfTimes := totalDurationDecay / s.oneSlotDuration()
	return math.Pow(decayToZero, 1/float64(numOfTimes))
}

func (s *Sentinel) pubsubOptions() []pubsub.Option {
	thresholds := &pubsub.PeerScoreThresholds{
		GossipThreshold:             -4000,
		PublishThreshold:            -8000,
		GraylistThreshold:           -16000,
		AcceptPXThreshold:           100,
		OpportunisticGraftThreshold: 5,
	}
	scoreParams := &pubsub.PeerScoreParams{
		Topics:        make(map[string]*pubsub.TopicScoreParams),
		TopicScoreCap: 32.72,
		AppSpecificScore: func(p peer.ID) float64 {
			return 0
		},
		AppSpecificWeight:           1,
		IPColocationFactorWeight:    -35.11,
		IPColocationFactorThreshold: 10,
		IPColocationFactorWhitelist: nil,
		BehaviourPenaltyWeight:      -15.92,
		BehaviourPenaltyThreshold:   6,
		BehaviourPenaltyDecay:       s.scoreDecay(10 * s.oneEpochDuration()), // 10 epochs
		DecayInterval:               s.oneSlotDuration(),
		DecayToZero:                 decayToZero,
		RetainScore:                 100 * s.oneEpochDuration(), // Retain for 100 epochs
	}
	pubsubQueueSize := 600
	psOpts := []pubsub.Option{
		pubsub.WithMessageSignaturePolicy(pubsub.StrictNoSign),
		pubsub.WithMessageIdFn(s.msgId),
		pubsub.WithNoAuthor(),
		pubsub.WithPeerOutboundQueueSize(pubsubQueueSize),
		pubsub.WithMaxMessageSize(int(s.cfg.NetworkConfig.GossipMaxSizeBellatrix)),
		pubsub.WithValidateQueueSize(pubsubQueueSize),
		pubsub.WithPeerScore(scoreParams, thresholds),
		pubsub.WithGossipSubParams(pubsubGossipParam()),
		pubsub.WithEventTracer(&MyTracer{}),
	}
	return psOpts
}

type MyTracer struct{}

func (m MyTracer) Trace(evt *pubsubpb.TraceEvent) {
	pid, _ := peer.IDFromBytes(evt.PeerID)

	switch *evt.Type {
	case pubsubpb.TraceEvent_PUBLISH_MESSAGE:
		fmt.Printf("[%s] [%s] Received event: %s\n", evt.Type, pid.ShortString(), *evt.PublishMessage.Topic)
	case pubsubpb.TraceEvent_REJECT_MESSAGE:
		fmt.Printf("[%s] [%s] Received event: %s\n", evt.Type, pid.ShortString(), *evt.RejectMessage.Topic)
	case pubsubpb.TraceEvent_DUPLICATE_MESSAGE:
		fmt.Printf("[%s] [%s] Received event: %s\n", evt.Type, pid.ShortString(), *evt.DuplicateMessage.Topic)
	case pubsubpb.TraceEvent_DELIVER_MESSAGE:
		fmt.Printf("[%s] [%s] Received event: %s\n", evt.Type, pid.ShortString(), *evt.DeliverMessage.Topic)
	case pubsubpb.TraceEvent_ADD_PEER:
		fmt.Printf("[%s] [%s] Received event\n", evt.Type, pid.ShortString())
	case pubsubpb.TraceEvent_REMOVE_PEER:
		fmt.Printf("[%s] [%s] Received event\n", evt.Type, pid.ShortString())
	case pubsubpb.TraceEvent_RECV_RPC:
		fmt.Printf("[%s] [%s] Received event\n", evt.Type, pid.ShortString())
	case pubsubpb.TraceEvent_SEND_RPC:
		fmt.Printf("[%s] [%s] Received event\n", evt.Type, pid.ShortString())
	case pubsubpb.TraceEvent_DROP_RPC:
		fmt.Printf("[%s] [%s] Received event\n", evt.Type, pid.ShortString())
	case pubsubpb.TraceEvent_JOIN:
		fmt.Printf("[%s] [%s] Received event: %s\n", evt.Type, pid.ShortString(), *evt.Join.Topic)
	case pubsubpb.TraceEvent_LEAVE:
		fmt.Printf("[%s] [%s] Received event: %s\n", evt.Type, pid.ShortString(), *evt.Leave.Topic)
	case pubsubpb.TraceEvent_GRAFT:
		fmt.Printf("[%s] [%s] Received event: %s\n", evt.Type, pid.ShortString(), *evt.Graft.Topic)
	case pubsubpb.TraceEvent_PRUNE:
		fmt.Printf("[%s] [%s] Received event: %s\n", evt.Type, pid.ShortString(), *evt.Prune.Topic)
	default:
		fmt.Printf("Received event: %s \n", evt.Type)
	}
}

var _ pubsub.EventTracer = (*MyTracer)(nil)

//_ pubsub.RawTracer   = (*MyTracer)(nil)

// creates a custom gossipsub parameter set.
func pubsubGossipParam() pubsub.GossipSubParams {
	gParams := pubsub.DefaultGossipSubParams()
	gParams.Dlo = gossipSubDlo
	gParams.D = gossipSubD
	gParams.HeartbeatInterval = gossipSubHeartbeatInterval
	gParams.HistoryLength = gossipSubMcacheLen
	gParams.HistoryGossip = gossipSubMcacheGossip
	return gParams
}
