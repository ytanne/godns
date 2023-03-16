package models

import "time"

type Record struct {
	Domain string
	IP     string
	Time   time.Time
}
