/*
Copyright IBM Corp. All Rights Reserved.
Copyright Xilinx Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package kvledger

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-protos-go/peer"
	"github.com/hyperledger/fabric/bccsp"
	"github.com/hyperledger/fabric/common/flogging"
	commonledger "github.com/hyperledger/fabric/common/ledger"
	"github.com/hyperledger/fabric/common/ledger/blkstorage"
	"github.com/hyperledger/fabric/common/util"
	"github.com/hyperledger/fabric/core/ledger"
	"github.com/hyperledger/fabric/core/ledger/cceventmgmt"
	"github.com/hyperledger/fabric/core/ledger/confighistory"
	"github.com/hyperledger/fabric/core/ledger/kvledger/bookkeeping"
	"github.com/hyperledger/fabric/core/ledger/kvledger/history"
	"github.com/hyperledger/fabric/core/ledger/kvledger/txmgmt/privacyenabledstate"
	"github.com/hyperledger/fabric/core/ledger/kvledger/txmgmt/txmgr"
	"github.com/hyperledger/fabric/core/ledger/kvledger/txmgmt/validation"
	"github.com/hyperledger/fabric/core/ledger/pvtdatapolicy"
	"github.com/hyperledger/fabric/core/ledger/pvtdatastorage"
	"github.com/hyperledger/fabric/internal/pkg/txflags"
	"github.com/hyperledger/fabric/protoutil"
	"github.com/pkg/errors"
)

var logger = flogging.MustGetLogger("kvledger")

var (
	rwsetHashOpts    = &bccsp.SHA256Opts{}
	snapshotHashOpts = &bccsp.SHA256Opts{}
)

// kvLedger provides an implementation of `ledger.PeerLedger`.
// This implementation provides a key-value based data model
type kvLedger struct {
	ledgerID               string
	blockStore             *blkstorage.BlockStore
	pvtdataStore           *pvtdatastorage.Store
	txmgr                  *txmgr.LockBasedTxMgr
	historyDB              *history.DB
	configHistoryRetriever *confighistory.Retriever
	blockAPIsRWLock        *sync.RWMutex
	stats                  *ledgerStats
	commitHash             []byte
	hashProvider           ledger.HashProvider
	snapshotsConfig        *ledger.SnapshotsConfig
	// isPvtDataStoreAheadOfBlockStore is read during missing pvtData
	// reconciliation and may be updated during a regular block commit.
	// Hence, we use atomic value to ensure consistent read.
	isPvtstoreAheadOfBlkstore atomic.Value
}

type lgrInitializer struct {
	ledgerID                 string
	blockStore               *blkstorage.BlockStore
	pvtdataStore             *pvtdatastorage.Store
	stateDB                  *privacyenabledstate.DB
	historyDB                *history.DB
	configHistoryMgr         *confighistory.Mgr
	stateListeners           []ledger.StateListener
	bookkeeperProvider       bookkeeping.Provider
	ccInfoProvider           ledger.DeployedChaincodeInfoProvider
	ccLifecycleEventProvider ledger.ChaincodeLifecycleEventProvider
	stats                    *ledgerStats
	customTxProcessors       map[common.HeaderType]ledger.CustomTxProcessor
	hashProvider             ledger.HashProvider
	snapshotsConfig          *ledger.SnapshotsConfig
}

func newKVLedger(initializer *lgrInitializer) (*kvLedger, error) {
	ledgerID := initializer.ledgerID
	logger.Debugf("Creating KVLedger ledgerID=%s: ", ledgerID)
	l := &kvLedger{
		ledgerID:        ledgerID,
		blockStore:      initializer.blockStore,
		pvtdataStore:    initializer.pvtdataStore,
		historyDB:       initializer.historyDB,
		hashProvider:    initializer.hashProvider,
		snapshotsConfig: initializer.snapshotsConfig,
		blockAPIsRWLock: &sync.RWMutex{},
	}

	btlPolicy := pvtdatapolicy.ConstructBTLPolicy(&collectionInfoRetriever{ledgerID, l, initializer.ccInfoProvider})

	rwsetHashFunc := func(data []byte) ([]byte, error) {
		hash, err := initializer.hashProvider.GetHash(rwsetHashOpts)
		if err != nil {
			return nil, err
		}
		if _, err = hash.Write(data); err != nil {
			return nil, err
		}
		return hash.Sum(nil), nil
	}

	txmgrInitializer := &txmgr.Initializer{
		LedgerID:            ledgerID,
		DB:                  initializer.stateDB,
		StateListeners:      initializer.stateListeners,
		BtlPolicy:           btlPolicy,
		BookkeepingProvider: initializer.bookkeeperProvider,
		CCInfoProvider:      initializer.ccInfoProvider,
		CustomTxProcessors:  initializer.customTxProcessors,
		HashFunc:            rwsetHashFunc,
	}
	if err := l.initTxMgr(txmgrInitializer); err != nil {
		return nil, err
	}

	// btlPolicy internally uses queryexecuter and indirectly ends up using txmgr.
	// Hence, we need to init the pvtdataStore once the txmgr is initiated.
	l.pvtdataStore.Init(btlPolicy)

	var err error
	l.commitHash, err = l.lastPersistedCommitHash()
	if err != nil {
		return nil, err
	}

	isAhead, err := l.isPvtDataStoreAheadOfBlockStore()
	if err != nil {
		return nil, err
	}
	l.isPvtstoreAheadOfBlkstore.Store(isAhead)

	// TODO Move the function `GetChaincodeEventListener` to ledger interface and
	// this functionality of registering for events to ledgermgmt package so that this
	// is reused across other future ledger implementations
	ccEventListener := initializer.stateDB.GetChaincodeEventListener()
	logger.Debugf("Register state db for chaincode lifecycle events: %t", ccEventListener != nil)
	if ccEventListener != nil {
		cceventmgmt.GetMgr().Register(ledgerID, ccEventListener)
		initializer.ccLifecycleEventProvider.RegisterListener(ledgerID, &ccEventListenerAdaptor{ccEventListener})
	}

	//Recover both state DB and history DB if they are out of sync with block storage
	if err := l.recoverDBs(); err != nil {
		return nil, err
	}
	l.configHistoryRetriever = initializer.configHistoryMgr.GetRetriever(ledgerID, l)

	l.stats = initializer.stats
	return l, nil
}

func (l *kvLedger) initTxMgr(initializer *txmgr.Initializer) error {
	var err error
	txmgr, err := txmgr.NewLockBasedTxMgr(initializer)
	if err != nil {
		return err
	}
	l.txmgr = txmgr
	// This is a workaround for populating lifecycle cache.
	// See comments on this function for details
	qe, err := txmgr.NewQueryExecutorNoCollChecks()
	if err != nil {
		return err
	}
	defer qe.Done()
	for _, sl := range initializer.StateListeners {
		if err := sl.Initialize(l.ledgerID, qe); err != nil {
			return err
		}
	}
	return err
}

func (l *kvLedger) lastPersistedCommitHash() ([]byte, error) {
	bcInfo, err := l.GetBlockchainInfo()
	if err != nil {
		return nil, err
	}
	if bcInfo.Height == 0 {
		logger.Debugf("Chain is empty")
		return nil, nil
	}

	logger.Debugf("Fetching block [%d] to retrieve the currentCommitHash", bcInfo.Height-1)
	block, err := l.GetBlockByNumber(bcInfo.Height - 1)
	if err != nil {
		return nil, err
	}

	if len(block.Metadata.Metadata) < int(common.BlockMetadataIndex_COMMIT_HASH+1) {
		logger.Debugf("Last block metadata does not contain commit hash")
		return nil, nil
	}

	commitHash := &common.Metadata{}
	err = proto.Unmarshal(block.Metadata.Metadata[common.BlockMetadataIndex_COMMIT_HASH], commitHash)
	if err != nil {
		return nil, errors.Wrap(err, "error unmarshaling last persisted commit hash")
	}
	return commitHash.Value, nil
}

func (l *kvLedger) isPvtDataStoreAheadOfBlockStore() (bool, error) {
	blockStoreInfo, err := l.blockStore.GetBlockchainInfo()
	if err != nil {
		return false, err
	}
	pvtstoreHeight, err := l.pvtdataStore.LastCommittedBlockHeight()
	if err != nil {
		return false, err
	}
	return pvtstoreHeight > blockStoreInfo.Height, nil
}

func (l *kvLedger) recoverDBs() error {
	logger.Debugf("Entering recoverDB()")
	if err := l.syncStateAndHistoryDBWithBlockstore(); err != nil {
		return err
	}
	if err := l.syncStateDBWithOldBlkPvtdata(); err != nil {
		return err
	}
	return nil
}

func (l *kvLedger) syncStateAndHistoryDBWithBlockstore() error {
	//If there is no block in blockstorage, nothing to recover.
	info, _ := l.blockStore.GetBlockchainInfo()
	if info.Height == 0 {
		logger.Debug("Block storage is empty.")
		return nil
	}
	lastAvailableBlockNum := info.Height - 1
	recoverables := []recoverable{l.txmgr}
	if l.historyDB != nil {
		recoverables = append(recoverables, l.historyDB)
	}
	recoverers := []*recoverer{}
	for _, recoverable := range recoverables {
		recoverFlag, firstBlockNum, err := recoverable.ShouldRecover(lastAvailableBlockNum)
		if err != nil {
			return err
		}

		// During ledger reset/rollback, the state database must be dropped. If the state database
		// uses goleveldb, the reset/rollback code itself drop the DB. If it uses couchDB, the
		// DB must be dropped manually. Hence, we compare (only for the stateDB) the height
		// of the state DB and block store to ensure that the state DB is dropped.

		// firstBlockNum is nothing but the nextBlockNum expected by the state DB.
		// In other words, the firstBlockNum is nothing but the height of stateDB.
		if firstBlockNum > lastAvailableBlockNum+1 {
			dbName := recoverable.Name()
			return fmt.Errorf("the %s database [height=%d] is ahead of the block store [height=%d]. "+
				"This is possible when the %s database is not dropped after a ledger reset/rollback. "+
				"The %s database can safely be dropped and will be rebuilt up to block store height upon the next peer start.",
				dbName, firstBlockNum, lastAvailableBlockNum+1, dbName, dbName)
		}
		if recoverFlag {
			recoverers = append(recoverers, &recoverer{firstBlockNum, recoverable})
		}
	}
	if len(recoverers) == 0 {
		return nil
	}
	if len(recoverers) == 1 {
		return l.recommitLostBlocks(recoverers[0].firstBlockNum, lastAvailableBlockNum, recoverers[0].recoverable)
	}

	// both dbs need to be recovered
	if recoverers[0].firstBlockNum > recoverers[1].firstBlockNum {
		// swap (put the lagger db at 0 index)
		recoverers[0], recoverers[1] = recoverers[1], recoverers[0]
	}
	if recoverers[0].firstBlockNum != recoverers[1].firstBlockNum {
		// bring the lagger db equal to the other db
		if err := l.recommitLostBlocks(recoverers[0].firstBlockNum, recoverers[1].firstBlockNum-1,
			recoverers[0].recoverable); err != nil {
			return err
		}
	}
	// get both the db upto block storage
	return l.recommitLostBlocks(recoverers[1].firstBlockNum, lastAvailableBlockNum,
		recoverers[0].recoverable, recoverers[1].recoverable)
}

func (l *kvLedger) syncStateDBWithOldBlkPvtdata() error {
	// TODO: syncStateDBWithOldBlkPvtdata, GetLastUpdatedOldBlocksPvtData(),
	// and ResetLastUpdatedOldBlocksList() can be removed in > v2 LTS.
	// From v2.0 onwards, we do not store the last updatedBlksList.
	// Only to support the rolling upgrade from v14 LTS to v2 LTS, we
	// retain these three functions in v2.0 - FAB-16294.

	blocksPvtData, err := l.pvtdataStore.GetLastUpdatedOldBlocksPvtData()
	if err != nil {
		return err
	}

	// Assume that the peer has restarted after a rollback or a reset.
	// As the pvtdataStore can contain pvtData of yet to be committed blocks,
	// we need to filter them before passing it to the transaction manager
	// for stateDB updates.
	if err := l.filterYetToCommitBlocks(blocksPvtData); err != nil {
		return err
	}

	if err = l.applyValidTxPvtDataOfOldBlocks(blocksPvtData); err != nil {
		return err
	}

	return l.pvtdataStore.ResetLastUpdatedOldBlocksList()
}

func (l *kvLedger) filterYetToCommitBlocks(blocksPvtData map[uint64][]*ledger.TxPvtData) error {
	info, err := l.blockStore.GetBlockchainInfo()
	if err != nil {
		return err
	}
	for blkNum := range blocksPvtData {
		if blkNum > info.Height-1 {
			logger.Infof("found pvtdata associated with yet to be committed block [%d]", blkNum)
			delete(blocksPvtData, blkNum)
		}
	}
	return nil
}

//recommitLostBlocks retrieves blocks in specified range and commit the write set to either
//state DB or history DB or both
func (l *kvLedger) recommitLostBlocks(firstBlockNum uint64, lastBlockNum uint64, recoverables ...recoverable) error {
	logger.Infof("Recommitting lost blocks - firstBlockNum=%d, lastBlockNum=%d, recoverables=%#v", firstBlockNum, lastBlockNum, recoverables)
	var err error
	var blockAndPvtdata *ledger.BlockAndPvtData
	for blockNumber := firstBlockNum; blockNumber <= lastBlockNum; blockNumber++ {
		if blockAndPvtdata, err = l.GetPvtDataAndBlockByNum(blockNumber, nil); err != nil {
			return err
		}
		for _, r := range recoverables {
			if err := r.CommitLostBlock(blockAndPvtdata); err != nil {
				return err
			}
		}
	}
	logger.Infof("Recommitted lost blocks - firstBlockNum=%d, lastBlockNum=%d, recoverables=%#v", firstBlockNum, lastBlockNum, recoverables)
	return nil
}

// GetTransactionByID retrieves a transaction by id
func (l *kvLedger) GetTransactionByID(txID string) (*peer.ProcessedTransaction, error) {
	l.blockAPIsRWLock.RLock()
	defer l.blockAPIsRWLock.RUnlock()
	tranEnv, err := l.blockStore.RetrieveTxByID(txID)
	if err != nil {
		return nil, err
	}
	txVResult, err := l.blockStore.RetrieveTxValidationCodeByTxID(txID)
	if err != nil {
		return nil, err
	}
	processedTran := &peer.ProcessedTransaction{TransactionEnvelope: tranEnv, ValidationCode: int32(txVResult)}
	return processedTran, nil
}

// GetBlockchainInfo returns basic info about blockchain
func (l *kvLedger) GetBlockchainInfo() (*common.BlockchainInfo, error) {
	l.blockAPIsRWLock.RLock()
	defer l.blockAPIsRWLock.RUnlock()
	bcInfo, err := l.blockStore.GetBlockchainInfo()
	return bcInfo, err
}

// GetBlockByNumber returns block at a given height
// blockNumber of  math.MaxUint64 will return last block
func (l *kvLedger) GetBlockByNumber(blockNumber uint64) (*common.Block, error) {
	l.blockAPIsRWLock.RLock()
	defer l.blockAPIsRWLock.RUnlock()
	block, err := l.blockStore.RetrieveBlockByNumber(blockNumber)
	return block, err
}

// GetBlocksIterator returns an iterator that starts from `startBlockNumber`(inclusive).
// The iterator is a blocking iterator i.e., it blocks till the next block gets available in the ledger
// ResultsIterator contains type BlockHolder
func (l *kvLedger) GetBlocksIterator(startBlockNumber uint64) (commonledger.ResultsIterator, error) {
	blkItr, err := l.blockStore.RetrieveBlocks(startBlockNumber)
	if err != nil {
		return nil, err
	}
	return &blocksItr{l.blockAPIsRWLock, blkItr}, nil
}

// GetBlockByHash returns a block given it's hash
func (l *kvLedger) GetBlockByHash(blockHash []byte) (*common.Block, error) {
	block, err := l.blockStore.RetrieveBlockByHash(blockHash)
	l.blockAPIsRWLock.RLock()
	l.blockAPIsRWLock.RUnlock()
	return block, err
}

// GetBlockByTxID returns a block which contains a transaction
func (l *kvLedger) GetBlockByTxID(txID string) (*common.Block, error) {
	l.blockAPIsRWLock.RLock()
	defer l.blockAPIsRWLock.RUnlock()
	block, err := l.blockStore.RetrieveBlockByTxID(txID)
	return block, err
}

func (l *kvLedger) GetTxValidationCodeByTxID(txID string) (peer.TxValidationCode, error) {
	l.blockAPIsRWLock.RLock()
	defer l.blockAPIsRWLock.RUnlock()
	txValidationCode, err := l.blockStore.RetrieveTxValidationCodeByTxID(txID)
	return txValidationCode, err
}

// NewTxSimulator returns new `ledger.TxSimulator`
func (l *kvLedger) NewTxSimulator(txid string) (ledger.TxSimulator, error) {
	return l.txmgr.NewTxSimulator(txid)
}

// NewQueryExecutor gives handle to a query executor.
// A client can obtain more than one 'QueryExecutor's for parallel execution.
// Any synchronization should be performed at the implementation level if required
func (l *kvLedger) NewQueryExecutor() (ledger.QueryExecutor, error) {
	return l.txmgr.NewQueryExecutor(util.GenerateUUID())
}

// NewHistoryQueryExecutor gives handle to a history query executor.
// A client can obtain more than one 'HistoryQueryExecutor's for parallel execution.
// Any synchronization should be performed at the implementation level if required
// Pass the ledger blockstore so that historical values can be looked up from the chain
func (l *kvLedger) NewHistoryQueryExecutor() (ledger.HistoryQueryExecutor, error) {
	if l.historyDB != nil {
		return l.historyDB.NewQueryExecutor(l.blockStore)
	}
	return nil, nil
}

// CommitLegacy commits the block and the corresponding pvt data in an atomic operation
func (l *kvLedger) CommitLegacy(pvtdataAndBlock *ledger.BlockAndPvtData, commitOpts *ledger.CommitOptions) error {
	var err error
	block := pvtdataAndBlock.Block
	blockNo := pvtdataAndBlock.Block.Header.Number

	startBlockProcessing := time.Now()
	if commitOpts.FetchPvtDataFromLedger {
		// when we reach here, it means that the pvtdata store has the
		// pvtdata associated with this block but the stateDB might not
		// have it. During the commit of this block, no update would
		// happen in the pvtdata store as it already has the required data.

		// if there is any missing pvtData, reconciler will fetch them
		// and update both the pvtdataStore and stateDB. Hence, we can
		// fetch what is available in the pvtDataStore. If any or
		// all of the pvtdata associated with the block got expired
		// and no longer available in pvtdataStore, eventually these
		// pvtdata would get expired in the stateDB as well (though it
		// would miss the pvtData until then)
		txPvtData, err := l.pvtdataStore.GetPvtDataByBlockNum(blockNo, nil)
		if err != nil {
			return err
		}
		pvtdataAndBlock.PvtData = convertTxPvtDataArrayToMap(txPvtData)
	}

	logger.Debugf("[%s] Validating state for block [%d]", l.ledgerID, blockNo)
	txstatsInfo, updateBatchBytes, err := l.txmgr.ValidateAndPrepare(pvtdataAndBlock, true)
	if err != nil {
		return err
	}
	l.printTxsFilter(block)
	elapsedBlockProcessing := time.Since(startBlockProcessing)

	startBlockstorageAndPvtdataCommit := time.Now()
	logger.Debugf("[%s] Adding CommitHash to the block [%d]", l.ledgerID, blockNo)
	// we need to ensure that only after a genesis block, commitHash is computed
	// and added to the block. In other words, only after joining a new channel
	// or peer reset, the commitHash would be added to the block
	if block.Header.Number == 1 || l.commitHash != nil {
		l.addBlockCommitHash(pvtdataAndBlock.Block, updateBatchBytes)
	}

	logger.Debugf("[%s] Committing pvtdata and block [%d] to storage", l.ledgerID, blockNo)
	l.blockAPIsRWLock.Lock()
	defer l.blockAPIsRWLock.Unlock()
	if err = l.commitToPvtAndBlockStore(pvtdataAndBlock); err != nil {
		return err
	}
	elapsedBlockstorageAndPvtdataCommit := time.Since(startBlockstorageAndPvtdataCommit)

	startCommitState := time.Now()
	logger.Debugf("[%s] Committing block [%d] transactions to state database", l.ledgerID, blockNo)
	if err = l.txmgr.Commit(); err != nil {
		panic(errors.WithMessage(err, "error during commit to txmgr"))
	}
	elapsedCommitState := time.Since(startCommitState)

	// History database could be written in parallel with state and/or async as a future optimization,
	// although it has not been a bottleneck...no need to clutter the log with elapsed duration.
	if l.historyDB != nil {
		logger.Debugf("[%s] Committing block [%d] transactions to history database", l.ledgerID, blockNo)
		if err := l.historyDB.Commit(block); err != nil {
			panic(errors.WithMessage(err, "Error during commit to history db"))
		}
	}

	logger.Infof("[%s] Committed block [%d] with %d transaction(s) in %dus (state_validation=%dus block_and_pvtdata_commit=%dus state_commit=%dus)"+
		" commitHash=[%x]",
		l.ledgerID, block.Header.Number, len(block.Data.Data),
		time.Since(startBlockProcessing)/time.Microsecond,
		elapsedBlockProcessing/time.Microsecond,
		elapsedBlockstorageAndPvtdataCommit/time.Microsecond,
		elapsedCommitState/time.Microsecond,
		l.commitHash,
	)
	l.updateBlockStats(
		elapsedBlockProcessing,
		elapsedBlockstorageAndPvtdataCommit,
		elapsedCommitState,
		txstatsInfo,
	)
	return nil
}

// printTxsFilter prints transactions filter from the block as 1-bit per transaction flags
// where tx0 is the least-significant bit.
// NOTE: This function can also use the TxStatInfo array returned by ValidateAndPrepare() called in
// the CommitWithPvtData() above.
func (l *kvLedger) printTxsFilter(block *common.Block) {
	txsFilter := txflags.ValidationFlags(block.Metadata.Metadata[common.BlockMetadataIndex_TRANSACTIONS_FILTER])
	var str strings.Builder
	res := uint32(0)

	for i := len(txsFilter) - 1; i >= 0; i-- {
		res <<= 1
		if txsFilter.IsValid(i) {
			res |= 1
		}
		if (i % 32) == 0 {
			str.WriteString(fmt.Sprintf("%#08X ", res))
			res = 0
		}
	}
	logger.Infof("Block [%d] transaction validation flags: %s", block.Header.Number, str.String())
}

func (l *kvLedger) commitToPvtAndBlockStore(blockAndPvtdata *ledger.BlockAndPvtData) error {
	pvtdataStoreHt, err := l.pvtdataStore.LastCommittedBlockHeight()
	if err != nil {
		return err
	}
	blockNum := blockAndPvtdata.Block.Header.Number

	if !l.isPvtstoreAheadOfBlkstore.Load().(bool) {
		logger.Debugf("Writing block [%d] to pvt data store", blockNum)
		// If a state fork occurs during a regular block commit,
		// we have a mechanism to drop all blocks followed by refetching of blocks
		// and re-processing them. In the current way of doing this, we only drop
		// the block files (and related artifacts) but we do not drop/overwrite the
		// pvtdatastorage as it might leads to data loss.
		// During block reprocessing, as there is a possibility of an invalid pvtdata
		// transaction to become valid, we store the pvtdata of invalid transactions
		// too in the pvtdataStore as we do for the publicdata in the case of blockStore.
		// Hence, we pass all pvtData present in the block to the pvtdataStore committer.
		pvtData, missingPvtData := constructPvtDataAndMissingData(blockAndPvtdata)
		if err := l.pvtdataStore.Commit(blockNum, pvtData, missingPvtData); err != nil {
			return err
		}
	} else {
		logger.Debugf("Skipping writing pvtData to pvt block store as it ahead of the block store")
	}

	if err := l.blockStore.AddBlock(blockAndPvtdata.Block); err != nil {
		return err
	}

	if pvtdataStoreHt == blockNum+1 {
		// Only when the pvtdataStore was ahead of blockStore
		// during the ledger initialization time, we reach here.
		// The pvtdataStore would be ahead of blockstore when
		// the peer restarts after a reset of rollback.
		l.isPvtstoreAheadOfBlkstore.Store(false)
	}

	return nil
}

func convertTxPvtDataArrayToMap(txPvtData []*ledger.TxPvtData) ledger.TxPvtDataMap {
	txPvtDataMap := make(ledger.TxPvtDataMap)
	for _, pvtData := range txPvtData {
		txPvtDataMap[pvtData.SeqInBlock] = pvtData
	}
	return txPvtDataMap
}

func (l *kvLedger) updateBlockStats(
	blockProcessingTime time.Duration,
	blockstorageAndPvtdataCommitTime time.Duration,
	statedbCommitTime time.Duration,
	txstatsInfo []*validation.TxStatInfo,
) {
	l.stats.updateBlockProcessingTime(blockProcessingTime)
	l.stats.updateBlockstorageAndPvtdataCommitTime(blockstorageAndPvtdataCommitTime)
	l.stats.updateStatedbCommitTime(statedbCommitTime)
	l.stats.updateTransactionsStats(txstatsInfo)
}

// GetMissingPvtDataInfoForMostRecentBlocks returns the missing private data information for the
// most recent `maxBlock` blocks which miss at least a private data of a eligible collection.
func (l *kvLedger) GetMissingPvtDataInfoForMostRecentBlocks(maxBlock int) (ledger.MissingPvtDataInfo, error) {
	// the missing pvtData info in the pvtdataStore could belong to a block which is yet
	// to be processed and committed to the blockStore and stateDB (such a scenario is possible
	// after a peer rollback). In such cases, we cannot return missing pvtData info. Otherwise,
	// we would end up in an inconsistent state database.
	if l.isPvtstoreAheadOfBlkstore.Load().(bool) {
		return nil, nil
	}
	// it is safe to not acquire a read lock on l.blockAPIsRWLock. Without a lock, the value of
	// lastCommittedBlock can change due to a new block commit. As a result, we may not
	// be able to fetch the missing data info of truly the most recent blocks. This
	// decision was made to ensure that the regular block commit rate is not affected.
	return l.pvtdataStore.GetMissingPvtDataInfoForMostRecentBlocks(maxBlock)
}

func (l *kvLedger) addBlockCommitHash(block *common.Block, updateBatchBytes []byte) {
	var valueBytes []byte

	txValidationCode := block.Metadata.Metadata[common.BlockMetadataIndex_TRANSACTIONS_FILTER]
	valueBytes = append(valueBytes, proto.EncodeVarint(uint64(len(txValidationCode)))...)
	valueBytes = append(valueBytes, txValidationCode...)
	valueBytes = append(valueBytes, updateBatchBytes...)
	valueBytes = append(valueBytes, l.commitHash...)

	l.commitHash = util.ComputeSHA256(valueBytes)
	block.Metadata.Metadata[common.BlockMetadataIndex_COMMIT_HASH] = protoutil.MarshalOrPanic(&common.Metadata{Value: l.commitHash})
}

// GetPvtDataAndBlockByNum returns the block and the corresponding pvt data.
// The pvt data is filtered by the list of 'collections' supplied
func (l *kvLedger) GetPvtDataAndBlockByNum(blockNum uint64, filter ledger.PvtNsCollFilter) (*ledger.BlockAndPvtData, error) {
	l.blockAPIsRWLock.RLock()
	defer l.blockAPIsRWLock.RUnlock()

	var block *common.Block
	var pvtdata []*ledger.TxPvtData
	var err error

	if block, err = l.blockStore.RetrieveBlockByNumber(blockNum); err != nil {
		return nil, err
	}

	if pvtdata, err = l.pvtdataStore.GetPvtDataByBlockNum(blockNum, filter); err != nil {
		return nil, err
	}

	return &ledger.BlockAndPvtData{Block: block, PvtData: constructPvtdataMap(pvtdata)}, nil
}

// GetPvtDataByNum returns only the pvt data  corresponding to the given block number
// The pvt data is filtered by the list of 'collections' supplied
func (l *kvLedger) GetPvtDataByNum(blockNum uint64, filter ledger.PvtNsCollFilter) ([]*ledger.TxPvtData, error) {
	l.blockAPIsRWLock.RLock()
	defer l.blockAPIsRWLock.RUnlock()
	var pvtdata []*ledger.TxPvtData
	var err error
	if pvtdata, err = l.pvtdataStore.GetPvtDataByBlockNum(blockNum, filter); err != nil {
		return nil, err
	}
	return pvtdata, nil
}

// DoesPvtDataInfoExist returns true when
// (1) the ledger has pvtdata associated with the given block number (or)
// (2) a few or all pvtdata associated with the given block number is missing but the
//     missing info is recorded in the ledger (or)
// (3) the block is committed but it does not contain even a single
//     transaction with pvtData.
func (l *kvLedger) DoesPvtDataInfoExist(blockNum uint64) (bool, error) {
	pvtStoreHt, err := l.pvtdataStore.LastCommittedBlockHeight()
	if err != nil {
		return false, err
	}
	return blockNum+1 <= pvtStoreHt, nil
}

func (l *kvLedger) GetConfigHistoryRetriever() (ledger.ConfigHistoryRetriever, error) {
	return l.configHistoryRetriever, nil
}

func (l *kvLedger) CommitPvtDataOfOldBlocks(reconciledPvtdata []*ledger.ReconciledPvtdata, unreconciled ledger.MissingPvtDataInfo) ([]*ledger.PvtdataHashMismatch, error) {
	logger.Debugf("[%s:] Comparing pvtData of [%d] old blocks against the hashes in transaction's rwset to find valid and invalid data",
		l.ledgerID, len(reconciledPvtdata))

	hashVerifiedPvtData, hashMismatches, err := constructValidAndInvalidPvtData(reconciledPvtdata, l.blockStore)
	if err != nil {
		return nil, err
	}

	err = l.applyValidTxPvtDataOfOldBlocks(hashVerifiedPvtData)
	if err != nil {
		return nil, err
	}

	logger.Debugf("[%s:] Committing pvtData of [%d] old blocks to the pvtdatastore", l.ledgerID, len(reconciledPvtdata))

	err = l.pvtdataStore.CommitPvtDataOfOldBlocks(hashVerifiedPvtData, unreconciled)
	if err != nil {
		return nil, err
	}

	return hashMismatches, nil
}

func (l *kvLedger) applyValidTxPvtDataOfOldBlocks(hashVerifiedPvtData map[uint64][]*ledger.TxPvtData) error {
	logger.Debugf("[%s:] Filtering pvtData of invalidation transactions", l.ledgerID)
	committedPvtData, err := filterPvtDataOfInvalidTx(hashVerifiedPvtData, l.blockStore)
	if err != nil {
		return err
	}

	// Assume the peer fails after storing the pvtData of old block in the stateDB but before
	// storing it in block store. When the peer starts again, the reconciler finds that the
	// pvtData is missing in the ledger store and hence, it would fetch those data again. As
	// a result, RemoveStaleAndCommitPvtDataOfOldBlocks gets already existing data. In this
	// scenario, RemoveStaleAndCommitPvtDataOfOldBlocks just replaces the old entry as we
	// always makes the comparison between hashed version and this pvtData. There is no
	// problem in terms of data consistency. However, if the reconciler is disabled before
	// the peer restart, then the pvtData in stateDB may not be in sync with the pvtData in
	// ledger store till the reconciler is enabled.
	logger.Debugf("[%s:] Committing pvtData of [%d] old blocks to the stateDB", l.ledgerID, len(hashVerifiedPvtData))
	return l.txmgr.RemoveStaleAndCommitPvtDataOfOldBlocks(committedPvtData)
}

func (l *kvLedger) GetMissingPvtDataTracker() (ledger.MissingPvtDataTracker, error) {
	return l, nil
}

// Close closes `KVLedger`
func (l *kvLedger) Close() {
	l.blockStore.Shutdown()
	l.txmgr.Shutdown()
}

type blocksItr struct {
	blockAPIsRWLock *sync.RWMutex
	blocksItr       commonledger.ResultsIterator
}

func (itr *blocksItr) Next() (commonledger.QueryResult, error) {
	block, err := itr.blocksItr.Next()
	if err != nil {
		return nil, err
	}
	itr.blockAPIsRWLock.RLock()
	itr.blockAPIsRWLock.RUnlock()
	return block, nil
}

func (itr *blocksItr) Close() {
	itr.blocksItr.Close()
}

type collectionInfoRetriever struct {
	ledgerID     string
	ledger       ledger.PeerLedger
	infoProvider ledger.DeployedChaincodeInfoProvider
}

func (r *collectionInfoRetriever) CollectionInfo(chaincodeName, collectionName string) (*peer.StaticCollectionConfig, error) {
	qe, err := r.ledger.NewQueryExecutor()
	if err != nil {
		return nil, err
	}
	defer qe.Done()
	return r.infoProvider.CollectionInfo(r.ledgerID, chaincodeName, collectionName, qe)
}

type ccEventListenerAdaptor struct {
	legacyEventListener cceventmgmt.ChaincodeLifecycleEventListener
}

func (a *ccEventListenerAdaptor) HandleChaincodeDeploy(chaincodeDefinition *ledger.ChaincodeDefinition, dbArtifactsTar []byte) error {
	return a.legacyEventListener.HandleChaincodeDeploy(&cceventmgmt.ChaincodeDefinition{
		Name:              chaincodeDefinition.Name,
		Hash:              chaincodeDefinition.Hash,
		Version:           chaincodeDefinition.Version,
		CollectionConfigs: chaincodeDefinition.CollectionConfigs,
	},
		dbArtifactsTar,
	)
}

func (a *ccEventListenerAdaptor) ChaincodeDeployDone(succeeded bool) {
	a.legacyEventListener.ChaincodeDeployDone(succeeded)
}

func filterPvtDataOfInvalidTx(hashVerifiedPvtData map[uint64][]*ledger.TxPvtData, blockStore *blkstorage.BlockStore) (map[uint64][]*ledger.TxPvtData, error) {
	committedPvtData := make(map[uint64][]*ledger.TxPvtData)
	for blkNum, txsPvtData := range hashVerifiedPvtData {

		// TODO: Instead of retrieving the whole block, we need to retrieve only
		// the TxValidationFlags from the block metadata. For that, we would need
		// to add a new index for the block metadata - FAB-15808
		block, err := blockStore.RetrieveBlockByNumber(blkNum)
		if err != nil {
			return nil, err
		}
		blockValidationFlags := txflags.ValidationFlags(block.Metadata.Metadata[common.BlockMetadataIndex_TRANSACTIONS_FILTER])

		var blksPvtData []*ledger.TxPvtData
		for _, pvtData := range txsPvtData {
			if blockValidationFlags.IsValid(int(pvtData.SeqInBlock)) {
				blksPvtData = append(blksPvtData, pvtData)
			}
		}
		committedPvtData[blkNum] = blksPvtData
	}
	return committedPvtData, nil
}

func constructPvtdataMap(pvtdata []*ledger.TxPvtData) ledger.TxPvtDataMap {
	if pvtdata == nil {
		return nil
	}
	m := make(map[uint64]*ledger.TxPvtData)
	for _, pvtdatum := range pvtdata {
		m[pvtdatum.SeqInBlock] = pvtdatum
	}
	return m
}

func constructPvtDataAndMissingData(blockAndPvtData *ledger.BlockAndPvtData) ([]*ledger.TxPvtData,
	ledger.TxMissingPvtDataMap) {

	var pvtData []*ledger.TxPvtData
	missingPvtData := make(ledger.TxMissingPvtDataMap)

	numTxs := uint64(len(blockAndPvtData.Block.Data.Data))

	for txNum := uint64(0); txNum < numTxs; txNum++ {
		if pvtdata, ok := blockAndPvtData.PvtData[txNum]; ok {
			pvtData = append(pvtData, pvtdata)
		}

		if missingData, ok := blockAndPvtData.MissingPvtData[txNum]; ok {
			for _, missing := range missingData {
				missingPvtData.Add(txNum, missing.Namespace,
					missing.Collection, missing.IsEligible)
			}
		}
	}
	return pvtData, missingPvtData
}
