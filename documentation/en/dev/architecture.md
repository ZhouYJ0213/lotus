# Node architecture

Not a complete reference, just introductory material to the key concepts and components in the code. Seen from the perspective of a syncing (not miner) node.

We start from the [Join Testnet](https://docs.lotu.sh/en+join-testnet) documentation. When we run `lotus daemon` the node starts and connects to (bootstrap) peers to download (sync) the chain.

We assume the use of an IDE that can follow Go symbolic reference or any other static analysis tool.

FIXME: Add Github links to every `github.com` listed here (this should be able to be done programmatically, every relative path is to Lotus and the rest are absolute paths). Alternatively, we can probably collapse most of the links with their corresponding functions and structures.

# CLI, API

Explain how do we communicate with the node, both in terms of the CLI and the programmatic way (to create our own tools).

## Client/server architecture

In terms of the Filecoin network the node is a peer on distributed hierarchy, but in terms of how we interact with the node we have client/server architecture.

The node itself was initiated with the `daemon` command, it already started syncing to the chain by default. Along with that service it also started a [JSON-RPC](https://en.wikipedia.org/wiki/JSON-RPC) server to allow a client to interact with it. (FIXME: Check if this client is local or can be remote, link to external documentation of connection API.)

We can connect to this server through the Lotus CLI. Virtually any other command other than `daemon` will run a client that will connect (by default) to the address specified in the `api` file in the repo associated with the node (by default in `~/.lotus`), e.g.,

```sh
cat  ~/.lotus/api && echo
# /ip4/127.0.0.1/tcp/1234/http

# With `lotus daemon` running in another terminal.
nc -v -z 127.0.0.1 1234 
# Connection to 127.0.0.1 1234 port [tcp/*] succeeded!
```

FIXME: We should be able to run everything in the same terminal, starting and stopping the daemon to make the queries, see https://github.com/filecoin-project/lotus/issues/1827 and https://github.com/filecoin-project/lotus/issues/1828:

```
lotus daemon &
lotus <wait-for-connection-command>
lotus log set-level error # Or a env.var in the daemon command.
nc -v -z 127.0.0.1 1234 
lotus <stop-daemon-command>
```

FIXME: Link to more in-depth documentation of the CLI architecture, maybe some IPFS documentation (since they share some common logic).

## Node API

The JSON-RPC server exposes the node API, the `FullNode` interface (defined in `api/api_full.go`). When we issue a command like `lotus sync status` to query the the progress of the node sync process we don't access the node's internals, those are decoupled in the separate daemon process, we call the `SyncState` function (of the `FullNode` API interface) through the RPC client started by our own command (see `NewFullNodeRPC` in `api/client/client.go` for more details).

FIXME: Link to (and create) documentation about API fulfillment.

Because we rely heavily on reflection for this part of the code the call chain is not easily visible by just following the references through the symbolic analysis of the IDE. If we start by the `lotus sync` command definition (in `cli/sync.go`), we eventually end up in the method interface `SyncState`, and when we look for its implementation we will find two functions:

* `(*SyncAPI).SyncState()` (in `node/impl/full/sync.go`): this is the actual implementation of the API function that shows what the node (here acting as the RPC server) will execute when it receives the RPC request issued from the CLI acting as the client.

* `(*FullNodeStruct).SyncState()`: this is an "empty placeholder" structure that will get later connected to the JSON-RPC client logic (see `NewMergeClient` in `lib/jsonrpc/client.go`, which is called by `NewFullNodeRPC`). (FIXME: check if this is accurate). The CLI (JSON-RPC client) will actually execute this function which will connect to the server and send the corresponding JSON request that will trigger the call of `(*SyncAPI).SyncState()` with the node implementation.

This means that when we are tracking the logic of a CLI command we will eventually find this bifurcation and study the code in server-side implementation in `node/impl/full` (mostly in the `common/` and `full/` directories). If we understand this architecture going directly to that part of the code abstracts away the JSON-RPC client/server logic and we can think that the CLI is actually running the node's logic.

FIXME: Explain that "*the* node" is actually an API structure like `impl.FullNodeAPI` with the different API subcomponents like `full.SyncAPI`. We won't see a *single* node structure, each API (full node, minder, etc) will gather the necessary subcomponents it needs to service its calls.

# Node build

When we start the daemon command (`cmd/lotus/daemon.go`) the node is created through [dependency injection](https://godoc.org/go.uber.org/fx) (again relying on reflection that might make some of the references hard to follow). The node sets up all of the subsystems it needs to run, e.g., the repository, the network connection, services like chain sync, etc. Each of those is order through a successive calls to the `node.Override` function (see `fx` linked above for more details). The structure of the call indicates first the type of component it will set up (many defined in `node/modules/dtypes/`) and the function that will provide it. The dependency is implicit in the argument of the provider function, for example, the `modules.ChainStore()` function that provide the `ChainStore` structure (as indicated by one of the `Override()` calls) takes as one of its parameters the `ChainBlockstore` type, which will be then one of its dependencies. For the node to be built successfully the `ChainBlockstore` will need to be provided before `ChainStore`, and this is indeed explicitly stated in another `Override()` call that sets the provider of that type the `ChainBlockstore()` function. (Most of these references can be followed through the IDE.)

## Repo

The repo is the directory where all the node information is stored. The repo is entirely defined by it making it easy to port to another location. This one-to-one relation means sometimes we will speak (or define structures in the code) of the node as just the repo it is associated to, instead of the daemon process that runs from that repo, both associations are correct and just depend on the context.

Only one daemon can run per node/repo at a time, if we try to start a second one (e.g., maybe because we have one running in the background without noticing it) we will get a `repo is already locked` error. This is the way for a process to signal it is running a node associated with that repo, a `repo.lock` will be created in it and seized by that process:

```sh
lsof ~/.lotus/repo.lock
# COMMAND   PID
# lotus   52356
```

FIXME: Replace the `repo is already locked` error with the actual explanation so we don't need to also translate error to reality here as well: https://github.com/filecoin-project/lotus/issues/1829.

The `node.Repo()` function (`node/builder.go`) contains most of the dependencies (specified as `Override()` calls) needed to properly set up the node's repo. We list the most salient ones here (defined in `node/modules/storage.go`).

`LockedRepo()`: a common pattern in the DI model is to not only provide actual structures the node will incorporate in itself but least any type of dependency in terms of "things that need to be done" before the node start, this is the case of this function that doesn't create and initialize any new structure but rather registers a `OnStop` [hook](https://godoc.org/go.uber.org/fx/internal/lifecycle#Hook) that will close the locked repository associated to it:

```Go
type LockedRepo interface {
	// Close closes repo and removes lock.
	Close() error

```

`Datastore` and `ChainBlockstore`: all data related to the node state is saved in the repo's `Datastore` an IPFS interface defined in `github.com/ipfs/go-datastore/datastore.go`. (See `(*fsLockedRepo).Datastore()` in `node/repo/fsrepo.go` for how Lotus creates it from a [Badger DB](https://github.com/dgraph-io/badger).) At the core every piece of data is a key-value pair in the `datastore` directory of the repo, but there are several abstractions we lay on top of it that appear through the code depending on *how* do we access it, but it is important to remember that the *where* is always the same.

FIXME: Maybe mention the `Batching` interface as the developer will stumble upon it before reaching the `Datastore` one.

The first abstraction is the `Blockstore` interface (`github.com/ipfs/go-ipfs-blockstore/blockstore.go`) which structures the key-value pair into the CID format for the key and the `Block` interface (`github.com/ipfs/go-block-format/blocks.go`) for the value, which is just a raw string of bytes addressed by its hashed (which is included in the CID key/identifier). `ChainBlockstore` will create a `Blockstore` in the repo under the `/blocks` namespace (basically every key stored there will have that prefix so it does not collide with other stores that use the same underlying repo).

FIXME: Link to IPFS documentation about DAG, CID, and related, especially we need a diagram that shows how do we wrap each datastore inside the next layer (datastore, batching, block store, gc, etc).

FIXME: Should we already do the IPFS block vs Filecoin block explanation here? Might be too distracting, but we should assume that the reader is already aware of the *block* term in the context of Filecoin so the confusion is likely to arise, maybe just link to another section explaining this.

Similarly, `modules.Datastore()` creates a `dtypes.MetadataDS` (alias for the basic `Datastore` interface) to store metadata under the `/metadata`. (FIXME: Explain *what* is metadata in contrast with the block store, namely we store the pointer to the heaviest chain, we might just link to that unwritten section here later.)

FIXME: Explain the key store related calls.

At the end of the `Repo()` function we see two mutually exclusive configuration calls based on the `RepoType` (`node/repo/fsrepo.go`).
```Go
			ApplyIf(isType(repo.FullNode), ConfigFullNode(c)),
			ApplyIf(isType(repo.StorageMiner), ConfigStorageMiner(c)),
```
As we said, the repo fully identified the node so a repo type is also a *node* type, in this case a full node or a storage miner. (FIXME: What is the difference between the two, does *full* imply miner?) In this case the `daemon` command will create a `FullNode`, this is specified in the command logic itself in `main.DaemonCmd()`, the `FsRepo` created (and passed to `node.Repo()`) will be initiated with that type (see `(*FsRepo).Init(t RepoType)`).

FIXME: Do we need to mention or link IPLD here? Where do we use IPLD stores?

	> In general we see the IPLD store everywhere, what do we need to know about IPLD in particular, can we just assume everything we save are raw key-value blocks?

## Online

The `node.Online()` configuration function (`node/builder.go`) sets up more than its name implies, the general theme though is that all of the components initialized here depend on the node connecting with the Filecoin network, being "online", through the libp2p stack (`libp2p()` discussed later). We list in this section the most relevant ones corresponding to the full node type (that is, included in the `ApplyIf(isType(repo.FullNode),` call).

`modules.ChainStore()`: creates the `store.ChainStore` structure (`chain/store/store.go`) that wraps the previous stores instantiated in `Repo()`, it is the main point of entry for the node to all chain-related data (FIXME: this is incorrect, we sometimes access its underlying block store directly, and probably shouldn't). Most important, it holds the pointer (`heaviest`) to the current head (see more details in the sync section later).

`ChainExchange()` and `ChainBlockservice()` establish a BitSwap connection (see libp2p() section below) to exchange chain information in the form of `blocks.Block`s stored in the repo. (See sync section for more details, the Filecoin blocks and messages are backed by this interconnected raw IPFS blocks that together form the different structures that define the state of the current/heaviest chain.)

`HandleIncomingBlocks()` and `HandleIncomingMessages()`: rather than create other structures they start the services in charge of processing new Filecoin blocks and messages from the network (see `<undefined>` for more information about the topics the node is subscribed to, FIXME: should that be part of the libp2p section or should we expand on gossipsub separately?).

`RunHello()`: starts the services to both send (`(*Service).SayHello()`) and receive (`(*Service).HandleStream`, `node/hello/hello.go`) `hello` messages with which nodes that establish a new connection with each other exchange chain-related information (namely their genesis block and their chain head, heavies tipset). (FIXME: Similar to above, should we expand on this in libp2p?)

`NewSyncer()`: creates the structure and starts the services related to the chain sync process (discussed in detailed in this document).

Although not necessarily listed in the dependency order here, we can establish those relations by looking at the parameters each function needs, and most importantly, by understanding the architecture of the node and how the different components relate to each other, the main objective of this document. For example, the sync mechanism depends on the node being able to exchange different IPFS blocks in the network to request the "missing pieces" to reconstruct the chain, this is reflected by `NewSyncer()` having a `blocksync.BlockSync` parameter, and this in turn depending on the above mentioned `ChainBlockservice()` and `ChainExchange()`. Similarly, the chain exchange service depends on the chain store to save and retrieve chain data,  this is reflected by `ChainExchange()` having `ChainGCBlockstore` as a parameter, which is just a wrapper around the `ChainBlockstore` with garbage collection support.

This block store is the same store underlying the chain store which in turn is an indirect dependency (through the `StateManager`) of the `NewSyncer()`. (FIXME: This last line is flaky, we need to resolve the hierarchy better, we sometimes refer to the chain store and sometimes its underlying block store.)

FIXME: We need a diagram to visualize all the different components just mentioned otherwise it is too hard to follow. We probably need to skip some of the connections mentioned.

## libp2p

FIXME: This should be a brief section with the relevant links to `libp2p` documentation and how do we use its services.

<<<<<<<<<<<<<<< SECOND PASS

# Sync

We follow the `NewSyncer` path in `Online()` to the `chain/` directory. `chain.NewSyncer()` in `chain/sync.go` creates the `Syncer` structure (with some attributes that will be explained as needed) which is started together with the application (see `fx.Hook`). When the `Syncer` is created it also creates in its turn a `SyncManager`, what is the domain separation between the two? The `Syncer` starts `SyncManager` which starts the different goroutines (`(*SyncManager).syncWorker()`) that actually process the incoming blocks, with the `doSync` function which is normally the `(*Syncer).Sync()` (going back to the previous structure now).

We receive blocks through the gossipsub topic (explain in a separate network section) but we later fetch the blocks needed through a different protocol in `(*BlockSyncService).HandleStream()` (is this correct? expand on graphsync).

In `(*SyncManager).Start()` we ignore the scheduling and focus just on the worker, `syncWorker()`, which will wait from the scheduler (`syncTargets`) new blocks (encapsulated in `TipSet`s) and call `doSync` (`(*Syncer).Sync`, set in `NewSyncer`) on them.

## Filecoin blocks

Normally we prefix them with the Filecoin qualifier as block is normally used in lower layers like IPFS to refer to a raw piece of binary data (normally associated with the stores).

At this point the reader should be familiarized with the IPFS model of storing and retrieving data by CIDs in a distributed manner. We normally don't transfer data itself in the model but their CIDs identifiers with which the node can fetch the actual data.

In `sub.HandleIncomingBlocks` (`chain/sub/incoming.go`, not to be confused with the one in the DI list), ignoring the verification steps, what arrives is the message with the CID of a block we should sync to (`chain/types/blockmsg.go`):

```Go
type BlockMsg struct {
	Header        *BlockHeader
	BlsMessages   []cid.Cid
	SecpkMessages []cid.Cid
}
```

The only actual data received is the header of the block, with the messages represented by their CIDs. Once we fetch the messages (through the normal IPFS channels, unrelated to the sync "topics", see later) we form the actual Filecoin block (`chain/types/fullblock.go`):

```Go
type FullBlock struct {
	Header        *BlockHeader
	BlsMessages   []*Message
	SecpkMessages []*SignedMessage
}
```

Messages from the same round are collected into a block set (`chain/store/fts.go`):

```Go
type FullTipSet struct {
	Blocks []*types.FullBlock
	tipset *types.TipSet
	cids   []cid.Cid
}
```

The "tipset" denomination might be a bit misleading as it doesn't refer *only* to the tip, the block set from the last round in the chain, but to *any* set of blocks, depending on the context the tipset is the actual tip or not. From its own perspective any block set is always the tip because it assumes nothing from following blocks.

FIXME: How are the tipsets connected? Explain the role of the CIDs and reference an IPFS document about hash chaining in general, how we cannot modify one without modifying all in the chain (Merkle tree).

## Back to sync

In `(*Syncer).collectChain` (which does more than just fetching), called from `(*Syncer).Sync`, we will have a (partial) tipset with the new block received from the network. We will first call `collectHeaders()` to fetch all the block sets that connect it to our current chain (handling potential fork cases).

Assuming a "fast forward" case where the received block connects directly to our current tipset (is that the *head* of the chain?) we will call `syncMessagesAndCheckState` to execute the messages inside the new blocks and update our state up to the new head.

The validations functions called thereafter (mainly `(*Syncer).ValidateBlock()`) will have as a side effect the execution of the messages which will generate the new state, see `(*StateManager).computeTipSetState()` accessed through the `Syncer` structure, and in turn `ApplyBlocks` and `(*VM).ApplyMessage`.

## What happens when we sync to a new head

Expand on how do we arrive to `(*ChainStore).takeHeaviestTipSet()`.

# Genesis block

Seems a good way to start exploring the VM state though the instantiation of its different actors like the storage power.

Explain where do we load the genesis block, the CAR entries, and we set the root of the state. Follow the daemon command option, `chain.LoadGenesis()` saves all the blocks of the CAR file into the store provided by `ChainBlockstore` (this should already be explained in the previous section). The CAR root (MT root?) of those blocks is decoded into the `BlockHeader` that will be the Filecoin (genesis) block of the chain, but most of the information was stored in the raw data (non-Filecoin, what's the correct term?) blocks forwarded directly to the chain, the block header just has a pointer to it.

`SetGenesis` block with name 0. `(ChainStore).SetGenesis()` stores it there.

`MakeInitialStateTree` (`chain/gen/genesis/genesis.go`, used to construct the genesis block (`MakeGenesisBlock()`), constructs the state tree (`NewStateTree`) which is just a "pointer" (root node in the HAMT) to the different actors. It will be continuously used in `(*StateTree).SetActor()` an `types.Actor` structure under a certain `Address` (in the HAMT). (How does the `stateSnaps` work? It has no comments.)

From this point we can follow different setup function like:

* `SetupInitActor()`: see the `AddressMap`.

* `SetupStoragePowerActor`: initial (zero) power state of the chain, most important attributes.

* Account actors in the `template.Accounts`: `SetActor`.

Which other actor type could be helpful at this point?

# Basic concepts

What should be clear at this point either from this document or the spec.

## Addresses

## Accounts

# Sync Topics PubSub

Gossip sub spec and some introduction.

# VM

Do we have a detailed intro to the VM in the spec?

The most important fact about the state is that it is just a collection of data (see actors later) stored under a root CID. Changing a state is just changing the value of that CID pointer (`StateTree.root`, `chain/state/statetree.go`). There is a different hierarchy of "pointers" here. Which is the top one? `ChainStore.heaviest`? (actually a pointer tipset)

This should continue the sync section at the part of the code flow where the messages from the blocks we are syncing are being applied.

In `ValidateBlock`, to check the messages we need to have the state from the previous tipset, we follow `(*StateManager).TipSetState` (`StateManager` structure seen before). Bypassing the cache logic, we would call `computeTipSetState()` on the tipset blocks. Note `ParentStateRoot` as the parent's parent on which we call `(*StateManager).ApplyBlocks`.

We first construct the VM (through an indirect pointer to `NewVM` in `chain/vm/vm.go`), which is at its most basic form the state tree (derived from the parent's state CID, now called `base`) and the `Store` where we will save the new information (state) generated to which the state tree will point.

For each message *independently* we call `(*VM).ApplyMessage()`, after many checks it calls `(*VM).send()` on the actor to which the message is targeted to (explain the send terminology or link to spec, send is the analog of a method call on an object). In the `vm.Runtime` (`(*VM).makeRuntime()`) all the contextual information is saved (including the VM itself), the runtime is the way we pass information to the actors on which we send the messages, used in ` vm.Invoke()`.

## Invoker

How is a message translated to an actual function? (Ugly and undocumented Go reflection ahead, we need to soften this.) The VM contains an `invoker` structure which is the responsible for making the transition, mapping a message code directed to an actor (each actor has its *own* set of codes defined in `specs-actors/actors/builtin/methods.go`) to an actual function (`(*invoker).Invoke()`) stored in itself (`invoker.builtInCode`), `invokeFunc`, which takes the runtime (state communication) and the serialized parameters (for the actor's logic):

```
type invoker struct {
	builtInCode  map[cid.Cid]nativeCode
	builtInState map[cid.Cid]reflect.Type
}

type invokeFunc func(rt runtime.Runtime, params []byte) ([]byte, aerrors.ActorError)
type nativeCode []invokeFunc
```

`(*invoker).Register()` stores for each actor available its set of methods exported through its interface.

```
type Invokee interface {
	Exports() []interface{}
}
```

Basic layout (without reflection details) of `(*invoker).transform()`. From each `Actor` structure take its `Exports()` methods converting them to `invokeFunc`. The actual method (`meth`) is wrapped in another function, that takes care of decoding the serialized parameters and the runtime, `shimCall` will contain the actors code which runs it inside a `defer` to `recover()` from panics (*we fail in the actors code with panics*). The return values will then be (CBOR) marshaled and returned to the VM.

## Back to the VM

`(*VM).ApplyMessage` will receive back from the invoker the serialized response and the `ActorError`. The returned message will be charged the corresponding gas to be stored in `rt.chargeGasSafe(rt.Pricelist().OnChainReturnValue(len(ret)))`, we don't process the return value any further than this (is anything else done with it?)

Besides charging the gas for the returned response, in `(*VM).makeRuntime()` the block store is wrapped around in another `gasChargingBlocks` store responsible for charging any other state information generated (through the runtime):

```
func (bs *gasChargingBlocks) Put(blk block.Block) error {
	bs.chargeGas(bs.pricelist.OnIpldPut(len(blk.RawData())))
```

(What happens when there is an error? Is that just discarded?)

# Look at the constructor of a miner

Follow the `lotus-storage-miner` command to see how a miner is created, from the command to the message to the storage power logic.

# Directory structure so far, main structures seen, their relation

List what are the main directories we should be looking at (e.g., `chain/`) and the most important structures (e.g., `StateTree`, `Runtime`, etc.)

# Tests

Run a few messages and observe state changes. What is the easiest test that also let's us "interact" with it (modify something and observe the difference).
