package build

import (
	"math/big"
	"sort"

	"github.com/libp2p/go-libp2p-core/protocol"

	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/actors/builtin/miner"

	"github.com/filecoin-project/lotus/node/modules/dtypes"
)

func DefaultSectorSize() abi.SectorSize {
	szs := make([]abi.SectorSize, 0, len(miner.SupportedProofTypes))
	for spt := range miner.SupportedProofTypes {
		ss, err := spt.SectorSize()
		if err != nil {
			panic(err)
		}

		szs = append(szs, ss)
	}

	sort.Slice(szs, func(i, j int) bool {
		return szs[i] < szs[j]
	})

	return szs[0]
}

// Core network constants

func BlocksTopic(netName dtypes.NetworkName) string   { return "/fil/blocks/" + string(netName) }
func MessagesTopic(netName dtypes.NetworkName) string { return "/fil/msgs/" + string(netName) }
func DhtProtocolName(netName dtypes.NetworkName) protocol.ID {
	return protocol.ID("/fil/kad/" + string(netName))
}

// /////
// Storage

const UnixfsChunkSize uint64 = 1 << 20
const UnixfsLinksPerLevel = 1024

// /////
// Consensus / Network

// Seconds
const AllowableClockDrift = 1

// Epochs
const ForkLengthThreshold = Finality

// Blocks (e)
var BlocksPerEpoch = uint64(builtin.ExpectedLeadersPerEpoch)

// Epochs
const Finality = miner.ChainFinalityish

// constants for Weight calculation
// The ratio of weight contributed by short-term vs long-term factors in a given round
const WRatioNum = int64(1)
const WRatioDen = 2

// /////
// Proofs

// Epochs
const SealRandomnessLookback = Finality

// Epochs
const SealRandomnessLookbackLimit = SealRandomnessLookback + 2000 // TODO: Get from spec specs-actors

// Maximum lookback that randomness can be sourced from for a seal proof submission
const MaxSealLookback = SealRandomnessLookbackLimit + 2000 // TODO: Get from specs-actors

// /////
// Mining

// Epochs
const TicketRandomnessLookback = 1

const WinningPoStSectorSetLookback = 10

// /////
// Devnet settings

const TotalFilecoin = 2_000_000_000
const MiningRewardTotal = 1_400_000_000

const FilecoinPrecision = 1_000_000_000_000_000_000

var InitialRewardBalance *big.Int

// TODO: Move other important consts here

func init() {
	InitialRewardBalance = big.NewInt(MiningRewardTotal)
	InitialRewardBalance = InitialRewardBalance.Mul(InitialRewardBalance, big.NewInt(FilecoinPrecision))
}

// Sync
const BadBlockCacheSize = 1 << 15

// assuming 4000 messages per round, this lets us not lose any messages across a
// 10 block reorg.
const BlsSignatureCacheSize = 40000

// Size of signature verification cache
// 32k keeps the cache around 10MB in size, max
const VerifSigCacheSize = 32000

// ///////
// Limits

// TODO: If this is gonna stay, it should move to specs-actors
const BlockMessageLimit = 512
const BlockGasLimit = 100_000_000

var DrandGroup = `{"nodes":[{"public":{"address":"dev2.rpc.drand.sh:4444","key":"kSlQYhQQxqXtHE3MQLYprjJEi0zZeS3HN2ZI/ym2fN2SGlQPQM4gyWDL6h3ynsv+","tls":true}},{"public":{"address":"dev1.rpc.drand.sh:4444","key":"s15BGGuNOv2gqi34SqPGyTckWPMWunR+ttqLLluoHYdymEQ2G4yQdfZa7DDuSJC4","tls":true},"index":1}],"threshold":2,"period":30,"genesis_time":1589461830,"genesis_seed":"jxbwEFJQtR805B+4RdCWaLLj2wCNrLPC1GHwuyNJuFQ=","dist_key":["iP228i/L5nG/kb779yPhWeWTT3hRaLQ3wDQkzeY2HP9fXTA0OQJg8hBDiUbyHYZ9","o4eqEPj2y+8GTx2bAnLidpstbFjjdLWl33womAULluUJvuahbdYyl+1hYP4D+fR0"]}`
