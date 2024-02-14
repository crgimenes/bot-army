package database

import (
	_ "embed"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

const (
	connectionString = `file:bot.db?mode=rwc&_journal_mode=WAL&_busy_timeout=10000`
)

type Database struct {
	db *sqlx.DB
}

type Message struct {
	ID      int    `db:"id"`
	Tag     string `db:"tag"`
	Message string `db:"message"`
}

func New() (*Database, error) {
	db, err := sqlx.Open("sqlite", connectionString)
	if err != nil {
		return nil, err
	}

	const sqlStmt = `CREATE TABLE IF NOT EXISTS messages (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
	tag TEXT NOT NULL,
	message TEXT NOT NULL,
	response TEXT NOT NULL);`
	_, err = db.Exec(sqlStmt)
	if err != nil {
		return nil, err
	}

	return &Database{
		db: db,
	}, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}

func (d *Database) AddMessage(tag, message, response string) error {
	const sqlStmt = `INSERT INTO messages (tag, message, response) VALUES ($1, $2, $3)`
	_, err := d.db.Exec(
		sqlStmt,
		tag,
		message,
		response,
	)
	return err
}

func (d *Database) GetNLastMesssages(tag string, n int) ([]Message, error) {
	messages := []Message{}
	const sqlStmt = `SELECT * FROM messages WHERE tag = $1 ORDER BY id DESC LIMIT $2`
	err := d.db.Select(
		&messages,
		sqlStmt,
		tag,
		n)
	return messages, err
}
