package drand

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/filecoin-project/lotus/build"
	"github.com/filecoin-project/lotus/chain/beacon"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"golang.org/x/xerrors"

	logging "github.com/ipfs/go-log"

	dbeacon "github.com/drand/drand/beacon"
	dclient "github.com/drand/drand/client"
	dkey "github.com/drand/drand/key"
	dproto "github.com/drand/drand/protobuf/drand"
)

var log = logging.Logger("drand")

var drandServers = []string{
	"https://dev1.drand.sh",
	"https://dev2.drand.sh",
}

var drandGroup *dkey.Group

func init() {
	var protoGroup dproto.GroupPacket
	err := json.Unmarshal([]byte(build.DrandGroup), &protoGroup)
	if err != nil {
		panic("could not unmarshal group info: " + err.Error())
	}
	drandGroup, err = dkey.GroupFromProto(&protoGroup)
	if err != nil {
		panic("could not convert from proto to key group: " + err.Error())
	}
}

type drandPeer struct {
	addr string
	tls  bool
}

func (dp *drandPeer) Address() string {
	return dp.addr
}

func (dp *drandPeer) IsTLS() bool {
	return dp.tls
}

type DrandBeacon struct {
	client dclient.Client

	pubkey *dkey.DistPublic

	// seconds
	interval time.Duration

	drandGenTime uint64
	filGenTime   uint64
	filRoundTime uint64

	cacheLk    sync.Mutex
	localCache map[uint64]types.BeaconEntry
}

func NewDrandBeacon(genesisTs, interval uint64) (*DrandBeacon, error) {
	if genesisTs == 0 {
		panic("what are you doing this cant be zero")
	}
	client, err := dclient.New(
		dclient.WithHTTPEndpoints(drandServers),
		dclient.WithGroup(drandGroup),
		dclient.WithCacheSize(1024),
	)
	if err != nil {
		return nil, xerrors.Errorf("creating drand client")
	}

	db := &DrandBeacon{
		client:     client,
		localCache: make(map[uint64]types.BeaconEntry),
	}

	db.pubkey = drandGroup.PublicKey
	db.interval = drandGroup.Period
	db.drandGenTime = uint64(drandGroup.GenesisTime)
	db.filRoundTime = interval
	db.filGenTime = genesisTs

	return db, nil
}

func (db *DrandBeacon) Entry(ctx context.Context, round uint64) <-chan beacon.Response {
	out := make(chan beacon.Response, 1)
	if round != 0 {
		be := db.getCachedValue(round)
		if be != nil {
			log.Warn("CACHE HIT")
			out <- beacon.Response{Entry: *be}
			close(out)
			return out
		}
	}

	go func() {
		log.Warnw("fetching randomness", "round", round)
		resp, err := db.client.Get(ctx, round)

		var br beacon.Response
		if err != nil {
			br.Err = xerrors.Errorf("drand failed Get request: %w", err)
		} else {
			br.Entry.Round = resp.Round()
			br.Entry.Data = resp.(*dclient.RandomData).Signature
		}

		out <- br
		close(out)
	}()

	return out
}
func (db *DrandBeacon) cacheValue(e types.BeaconEntry) {
	db.cacheLk.Lock()
	defer db.cacheLk.Unlock()
	db.localCache[e.Round] = e
}

func (db *DrandBeacon) getCachedValue(round uint64) *types.BeaconEntry {
	db.cacheLk.Lock()
	defer db.cacheLk.Unlock()
	v, ok := db.localCache[round]
	if !ok {
		return nil
	}
	return &v
}

func (db *DrandBeacon) VerifyEntry(curr types.BeaconEntry, prev types.BeaconEntry) error {
	if prev.Round == 0 {
		// TODO handle genesis better
		return nil
	}
	b := &dbeacon.Beacon{
		PreviousSig: prev.Data,
		Round:       curr.Round,
		Signature:   curr.Data,
	}
	err := dbeacon.VerifyBeacon(db.pubkey.Key(), b)
	if err == nil {
		db.cacheValue(curr)
	}
	return err
}

func (db *DrandBeacon) MaxBeaconRoundForEpoch(filEpoch abi.ChainEpoch, prevEntry types.BeaconEntry) uint64 {
	// TODO: sometimes the genesis time for filecoin is zero and this goes negative
	latestTs := ((uint64(filEpoch) * db.filRoundTime) + db.filGenTime) - db.filRoundTime
	dround := (latestTs - db.drandGenTime) / uint64(db.interval.Seconds())
	return dround
}

var _ beacon.RandomBeacon = (*DrandBeacon)(nil)
