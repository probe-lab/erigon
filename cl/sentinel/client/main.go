package main

import (
	"context"
	"log"
	"os"
	"path"
	"time"

	sentinel2 "github.com/ledgerwatch/erigon-lib/gointerfaces/sentinel"

	"google.golang.org/grpc/credentials"

	"github.com/ledgerwatch/erigon-lib/chain/networkname"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	"github.com/ledgerwatch/erigon/cl/clparams"
	"github.com/ledgerwatch/erigon/cl/cltypes"
	"github.com/ledgerwatch/erigon/cl/fork"
	"github.com/ledgerwatch/erigon/cl/persistence"
	"github.com/ledgerwatch/erigon/cl/persistence/db_config"
	"github.com/ledgerwatch/erigon/cl/phase1/core"
	"github.com/ledgerwatch/erigon/cl/sentinel"
	"github.com/ledgerwatch/erigon/cl/sentinel/service"
	"github.com/ledgerwatch/erigon/params"
	lwlog "github.com/ledgerwatch/log/v3"
)

func main() {
	ctx := context.Background()

	logger := lwlog.Root()
	logger.SetHandler(lwlog.LvlFilterHandler(lwlog.LvlDebug, lwlog.StdoutHandler))

	pathCaplinHistory := "caplin/history"
	pathCaplinIndexing := "caplin/indexing"

	netId := params.NetworkIDByChainName(networkname.MainnetChainName)
	genesisCfg, networkCfg, beaconCfg := clparams.GetConfigsByNetwork(clparams.NetworkType(netId))

	log.Println("AferoRawBeaconBlockChainFromOsPath")
	rawBeaconBlockChainDb, _ := persistence.AferoRawBeaconBlockChainFromOsPath(beaconCfg, pathCaplinHistory)

	dataDirIndexer := path.Join(pathCaplinIndexing, "beacon_indicies")

	os.MkdirAll(pathCaplinIndexing, 0o700)

	indiciesDB := mdbx.MustOpen(dataDirIndexer)

	log.Println("BeginRw")
	tx, err := indiciesDB.BeginRw(ctx)
	if err != nil {
		log.Fatalln(err)
	}
	defer tx.Rollback()

	log.Println("WriteConfigurationIfNotExist")
	if err := db_config.WriteConfigurationIfNotExist(ctx, tx, db_config.DefaultDatabaseConfiguration); err != nil {
		log.Fatalln(err)
	}

	if err := tx.Commit(); err != nil {
		log.Fatalln(err)
	}
	{ // start ticking forkChoice
		go func() {
			<-ctx.Done()
			indiciesDB.Close() // close sql database here
		}()
	}

	log.Println("RetrieveBeaconState")
	state, err := core.RetrieveBeaconState(ctx, beaconCfg, genesisCfg, clparams.GetCheckpointSyncEndpoint(clparams.NetworkType(netId)))
	if err != nil {
		log.Fatalln(err)
	}

	forkDigest, err := fork.ComputeForkDigest(beaconCfg, genesisCfg)
	if err != nil {
		log.Fatalln(err)
	}

	var creds credentials.TransportCredentials
	log.Println("StartSentinelService")
	client, err := service.StartSentinelService(&sentinel.SentinelConfig{
		IpAddr:        "127.0.0.1",
		Port:          4002,
		TCPPort:       4003,
		GenesisConfig: genesisCfg,
		NetworkConfig: networkCfg,
		BeaconConfig:  beaconCfg,
		TmpDir:        "./tmp",
		EnableBlocks:  true,
	}, rawBeaconBlockChainDb, indiciesDB, &service.ServerConfig{Network: "tcp", Addr: "localhost:7777"}, creds, &cltypes.Status{
		ForkDigest:     forkDigest,
		FinalizedRoot:  state.FinalizedCheckpoint().BlockRoot(),
		FinalizedEpoch: state.FinalizedCheckpoint().Epoch(),
		HeadSlot:       state.FinalizedCheckpoint().Epoch() * beaconCfg.SlotsPerEpoch,
		HeadRoot:       state.FinalizedCheckpoint().BlockRoot(),
	}, logger)
	if err != nil {
		log.Fatalln(err)
	}

	log.Println("for")
	for {
		select {
		case <-time.Tick(10 * time.Second):
			p, err := client.GetPeers(ctx, &sentinel2.EmptyMessage{})
			if err != nil {
				log.Println("error getting peers", err)
			} else {
				log.Println("Peers", p.Amount)
			}
		case <-ctx.Done():
			return
		}
	}

	//
	//gconn, err := grpc.Dial("localhost:7777", grpc.WithInsecure(), grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(math.MaxInt)))
	//if err != nil {
	//	panic(err)
	//}
	//sc := sentinel.NewSentinelClient(gconn)
	//subscription, err := sc.SubscribeGossip(ctx, &sentinel.SubscriptionData{})
	//if err != nil {
	//	return
	//}
	//
	//defer func() {
	//	recover()
	//	gconn.Close()
	//}()
	//
	//p, err := sc.GetPeers(ctx, &sentinel.EmptyMessage{})
	//if err != nil {
	//	panic(err)
	//}
	//
	//fmt.Println(p.Amount)
	//
	//fmt.Println("Start")
	//for {
	//	data, err := subscription.Recv()
	//	if err != nil {
	//		log.Warn("[Beacon Gossip] Fatal error receiving gossip", "err", err)
	//		break
	//	}
	//	fmt.Printf("Received %s\n", data.Name)
	//}
	//fmt.Println("Done")
}
