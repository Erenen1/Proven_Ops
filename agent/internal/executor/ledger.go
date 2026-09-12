package executor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.etcd.io/bbolt"
)

type ExecutionStatus string

const (
	StatusReceived  ExecutionStatus = "RECEIVED"
	StatusRunning   ExecutionStatus = "RUNNING"
	StatusSucceeded ExecutionStatus = "SUCCEEDED"
	StatusFailed    ExecutionStatus = "FAILED"
	StatusUnknown   ExecutionStatus = "UNKNOWN"
)

type ExecutionRecord struct {
	ExecutionID string               `json:"execution_id"`
	TaskID      string               `json:"task_id"`
	StepID      string               `json:"step_id"`
	Action      string               `json:"action"`
	Status      ExecutionStatus      `json:"status"`
	StartedAt   time.Time            `json:"started_at"`
	FinishedAt  *time.Time           `json:"finished_at,omitempty"`
	Result      *StepExecutionResult `json:"result,omitempty"`
}

var (
	bucketName = []byte("execution_records")
)

type Ledger struct {
	dbPath string
	db     *bbolt.DB
	mu     sync.Mutex
}

func NewLedger(dbPath string) (*Ledger, error) {
	if dbPath == "" {
		dbPath = "/var/lib/opspilot/execution_ledger.db"
	}

	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		// Fallback to local directory if root permission is missing
		dbPath = "execution_ledger.db"
	}

	db, err := bbolt.Open(dbPath, 0600, &bbolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("failed to open bbolt ledger at %s: %w", dbPath, err)
	}

	// Ensure bucket exists
	err = db.Update(func(tx *bbolt.Tx) error {
		_, bErr := tx.CreateBucketIfNotExists(bucketName)
		return bErr
	})
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to initialize ledger bucket: %w", err)
	}

	// Scan on startup: any record left in RUNNING when agent crashed is converted to UNKNOWN
	_ = db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketName)
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var rec ExecutionRecord
			if err := json.Unmarshal(v, &rec); err == nil {
				if rec.Status == StatusRunning || rec.Status == StatusReceived {
					rec.Status = StatusUnknown
					updatedBytes, _ := json.Marshal(rec)
					_ = b.Put(k, updatedBytes)
				}
			}
		}
		return nil
	})

	return &Ledger{
		dbPath: dbPath,
		db:     db,
	}, nil
}

func (l *Ledger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.db != nil {
		return l.db.Close()
	}
	return nil
}

func (l *Ledger) Get(executionID string) (*ExecutionRecord, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	var rec *ExecutionRecord
	err := l.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketName)
		val := b.Get([]byte(executionID))
		if val == nil {
			return nil
		}
		var r ExecutionRecord
		if err := json.Unmarshal(val, &r); err != nil {
			return err
		}
		rec = &r
		return nil
	})
	return rec, err
}

func (l *Ledger) RecordStart(executionID, taskID, stepID, action string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	rec := &ExecutionRecord{
		ExecutionID: executionID,
		TaskID:      taskID,
		StepID:      stepID,
		Action:      action,
		Status:      StatusRunning,
		StartedAt:   time.Now(),
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}

	return l.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketName)
		return b.Put([]byte(executionID), data)
	})
}

func (l *Ledger) RecordCompletion(executionID string, status ExecutionStatus, res *StepExecutionResult) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketName)
		val := b.Get([]byte(executionID))
		var rec ExecutionRecord
		if val != nil {
			_ = json.Unmarshal(val, &rec)
		} else {
			rec = ExecutionRecord{
				ExecutionID: executionID,
				StartedAt:   time.Now(),
			}
		}

		now := time.Now()
		rec.FinishedAt = &now
		rec.Status = status
		rec.Result = res

		data, err := json.Marshal(rec)
		if err != nil {
			return err
		}
		return b.Put([]byte(executionID), data)
	})
}
