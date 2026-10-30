// Package messagestore keeps every chat message the user sent or received,
// one row each in time order, whatever app carried it: texts, Skype,
// Messenger, Discord, and what multichat logs live.
//
// Conversations, messages and conversation days are cited elsewhere by their
// ids (conversation_000001, chat_message_000001, conversation_day_000001),
// which kanban-store's registry claims, so a timeline can link one as a chip.
package messagestore

import (
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed schema.sql
var schemaSQL string

var (
	ErrNotFound = errors.New("not found")
	ErrInvalid  = errors.New("invalid")
)

// Id prefixes. Each is claimed by a kanban-store registry type of the same
// name; none contains another's pattern at a word boundary.
const (
	prefixConversation    = "conversation_"
	prefixMessage         = "chat_message_"
	prefixConversationDay = "conversation_day_"
	prefixImport          = "message_import_"
)

// Store owns the database.
type Store struct {
	db      *sql.DB
	dataDir string
	// writeMutex serializes every write; the next seq is MAX(seq)+1 read
	// inside the write's transaction.
	writeMutex sync.Mutex
}

// Open creates the data directory (0700) if needed, opens the database and
// applies the schema.
func Open(dataDir string) (*Store, error) {
	if dataDir == "" {
		return nil, errors.New("open store: no data directory given")
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	if err := os.Chmod(dataDir, 0o700); err != nil {
		return nil, err
	}
	dbPath := filepath.Join(dataDir, "message-store.db")
	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL: %w", err)
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	if err := os.Chmod(dbPath, 0o600); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db, dataDir: dataDir}, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// DataDir is where the database lives.
func (s *Store) DataDir() string { return s.dataDir }

func formatID(prefix string, seq int64) string { return fmt.Sprintf("%s%06d", prefix, seq) }

func nowMillis() int64 { return time.Now().UnixMilli() }

// nextSeq reads the next seq of table inside tx.
func nextSeq(tx *sql.Tx, table string) (int64, error) {
	var max sql.NullInt64
	if err := tx.QueryRow("SELECT MAX(seq) FROM " + table).Scan(&max); err != nil {
		return 0, err
	}
	return max.Int64 + 1, nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
