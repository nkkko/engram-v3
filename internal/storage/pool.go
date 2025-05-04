package storage

import (
	"context"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v4"
)

// TransactionPool manages a pool of Badger transactions
type TransactionPool struct {
	txns        chan *badger.Txn
	txnMutex    sync.Mutex
	db          *badger.DB
	
	maxConnections int
	idleTimeout    time.Duration
	
	ctx    context.Context
	cancel context.CancelFunc
}

// NewTransactionPool creates a new transaction pool
func NewTransactionPool(db *badger.DB, maxConnections int, idleTimeout time.Duration) *TransactionPool {
	ctx, cancel := context.WithCancel(context.Background())
	
	pool := &TransactionPool{
		txns:           make(chan *badger.Txn, maxConnections),
		db:             db,
		maxConnections: maxConnections,
		idleTimeout:    idleTimeout,
		ctx:            ctx,
		cancel:         cancel,
	}
	
	// Start cleanup routine
	go pool.cleanup()
	
	return pool
}

// GetReadTxn gets a read-only transaction from the pool or creates a new one
func (p *TransactionPool) GetReadTxn() *badger.Txn {
	p.txnMutex.Lock()
	defer p.txnMutex.Unlock()
	
	select {
	case <-p.txns:
		// Discard the old transaction from the pool and create a new one
		return p.db.NewTransaction(false) // read-only
	default:
		// No free transactions, create a new one
		return p.db.NewTransaction(false) // read-only
	}
}

// GetWriteTxn creates a new write transaction (not pooled)
// Write transactions are not pooled as they are typically short-lived
// and should be committed or discarded promptly
func (p *TransactionPool) GetWriteTxn() *badger.Txn {
	return p.db.NewTransaction(true) // writable
}

// PutTxn returns a transaction to the pool if possible
func (p *TransactionPool) PutTxn(txn *badger.Txn) {
	if txn == nil {
		return
	}
	
	// In Badger v4, we can't directly check transaction state
	// So we'll create new transactions when getting from the pool
	p.txnMutex.Lock()
	defer p.txnMutex.Unlock()
	
	select {
	case p.txns <- txn:
		// Successfully returned to the pool
	default:
		// Pool is full, discard the transaction
		txn.Discard()
	}
}

// cleanup periodically refreshes transactions to prevent resource leaks
func (p *TransactionPool) cleanup() {
	ticker := time.NewTicker(p.idleTimeout)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			p.doCleanup()
		case <-p.ctx.Done():
			return
		}
	}
}

// doCleanup performs the actual cleanup operations
func (p *TransactionPool) doCleanup() {
	p.txnMutex.Lock()
	defer p.txnMutex.Unlock()
	
	var txns []*badger.Txn
	
	// Drain the channel
	for {
		select {
		case txn := <-p.txns:
			txns = append(txns, txn)
		default:
			goto done
		}
	}
	
done:
	// Discard all transactions and create fresh ones
	for _, txn := range txns {
		txn.Discard()
	}
}

// Close shuts down the transaction pool and discards all transactions
func (p *TransactionPool) Close() error {
	p.cancel()
	
	// Drain and discard all transactions
	p.txnMutex.Lock()
	defer p.txnMutex.Unlock()
	
	for {
		select {
		case txn := <-p.txns:
			txn.Discard()
		default:
			return nil
		}
	}
}