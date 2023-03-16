package repo

import (
	"encoding/json"
	"log"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/ytanne/godns/pkg/models"
)

type repo struct {
	db   *leveldb.DB
	path string
}

func NewLevelDB(path string) (*repo, error) {
	db, err := leveldb.OpenFile(path, nil)
	if err != nil {
		return nil, err
	}

	return &repo{
		db:   db,
		path: path,
	}, nil
}

func (r *repo) Get(key string) (models.Record, error) {
	value, err := r.db.Get([]byte(key), nil)
	if err != nil {
		return models.Record{}, err
	}

	record := new(models.Record)

	err = json.Unmarshal(value, record)
	if err != nil {
		return models.Record{}, err
	}

	return *record, nil
}

func (r *repo) GetAll() ([]models.Record, error) {
	var records []models.Record

	iter := r.db.NewIterator(nil, nil)
	for iter.Next() {
		value := iter.Value()
		record := new(models.Record)

		err := json.Unmarshal(value, record)
		if err != nil {
			log.Println("could not retrieve records:", err)

			return nil, err
		}

		records = append(records, *record)
	}

	return records, nil
}

func (r *repo) Set(key string, record models.Record) error {
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}

	return r.db.Put([]byte(key), data, nil)
}

func (r *repo) Remove(key string) error {
	return r.db.Delete([]byte(key), nil)
}

func (r *repo) Close() error {
	return r.db.Close()
}
