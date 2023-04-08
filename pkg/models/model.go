package models

import "time"

type Record struct {
	Domain string    `json:"domain"`
	IP     string    `json:"ipv4"`
	Time   time.Time `json:"time"`
}
