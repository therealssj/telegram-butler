package auction_butler

import (
	"errors"
	"time"

	"database/sql"

	"strconv"

	"github.com/jmoiron/sqlx"
)

type DB struct {
	*sqlx.DB
}

func (db *DB) ScheduleEvent(coins int, start time.Time, duration Duration, surprise bool) error {
	//TODO (therealssj): implement
	return nil
}

func (db *DB) StartNewEvent(coins int, duration Duration) error {
	//TODO (therealssj): implement

	return nil
}

func (db *DB) StartEvent(e *Event) error {
	//TODO (therealssj): implement

	return nil
}

func (db *DB) EndEvent(e *Event) error {
	if e.EndedAt.Valid {
		return errors.New("already ended")
	}
	t := NewNullTime(time.Now())
	_, err := db.Exec(
		db.Rebind("update event set ended_at = ? where id = ?"),
		t, e.ID,
	)
	if err == nil {
		e.EndedAt = t
	}
	return err
}

func (db *DB) GetCurrentEvent() *Event {
	var event Event

	err := db.Get(&event, "SELECT * FROM event WHERE ended_at IS NULL")

	if err == sql.ErrNoRows {
		return nil
	}

	if err != nil {
		panic(err)
		return nil
	}

	return &event
}

func (db *DB) GetLastEvent() *Event {
	var event Event

	err := db.Get(&event, "SELECT * FROM event WHERE ended_at IS NOT NULL AND started_at IS NOT NULL ORDER BY id DESC LIMIT 1")

	if err == sql.ErrNoRows {
		return nil
	}

	if err != nil {
		panic(err)
		return nil
	}

	return &event
}

func NewDB(config *DatabaseConfig) (*DB, error) {
	if config == nil {
		errors.New("config should not be nil in NewDB()")
	}
	db, err := sqlx.Open(config.Driver, config.Source)
	if err != nil {
		return nil, err
	}
	return &DB{db}, nil
}

func (db *DB) GetUser(id int) *User {
	var user User
	err := db.Get(&user, db.Rebind("select * from botuser where id=?"), id)
	if err == sql.ErrNoRows {
		return nil
	}

	if err != nil {
		panic(err)
		return nil
	}

	user.exists = true
	return &user
}

func (db *DB) GetUserByName(name string) *User {
	var user User
	err := db.Get(&user, db.Rebind("select * from botuser where username=?"), name)

	if err == sql.ErrNoRows {
		return nil
	}

	if err != nil {
		panic(err)
		return nil
	}

	user.exists = true
	return &user
}

func (db *DB) GetUserByNameOrId(identifier string) *User {
	var user User
	var err error

	// Check if string is an integer
	// If its an int then check for user id
	// otherwise check for username
	if _, err := strconv.Atoi(identifier); err == nil {
		err = db.Get(&user, db.Rebind("select * from botuser where id=?"), identifier)
	} else {
		err = db.Get(&user, db.Rebind("select * from botuser where username=?"), identifier)
	}

	// @TODO improve this check
	if err == sql.ErrNoRows || user.ID == 0 {
		return nil
	}

	if err != nil {
		panic(err)
		return nil
	}

	user.exists = true
	return &user
}

func (db *DB) GetUsers(banned bool) ([]User, error) {
	var users []User

	err := db.Select(&users, db.Rebind("select * from botuser where banned = ? order by username"), banned)
	if err != nil {
		return nil, err
	}

	return users, nil
}

func (db *DB) GetAdmins() ([]User, error) {
	var users []User

	err := db.Select(&users, db.Rebind("select * from botuser where admin=true"))

	if err != nil {
		return nil, err
	}

	return users, nil
}

func (db *DB) GetUserCount(banned bool) (int, error) {
	var count int

	err := db.Get(&count, db.Rebind("select count(*) from botuser where banned = ?"), banned)
	if err != nil {
		return 0, err
	}

	return count, nil
}
