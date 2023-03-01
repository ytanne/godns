package main

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/miekg/dns"
)

var (
	ErrNotImplemented = fmt.Errorf("not implemented")
)

const (
	// Caching for a day
	timeLimit = 24 * time.Hour
)

type record struct {
	ip   string
	time time.Time
}

type customServer struct {
	sync.RWMutex
	records map[string]record
}

func (c *customServer) readRecord(hostname string) (string, bool) {
	c.RLock()
	record, ok := c.records[hostname]
	c.RUnlock()
	if !ok {
		return "", false
	}

	if time.Since(record.time) >= timeLimit {
		c.Lock()
		delete(c.records, hostname)
		c.Unlock()

		return "", false
	}

	return record.ip, true
}

func (c *customServer) writeRecord(hostname, ip string) {
	c.RLock()
	_, ok := c.records[hostname]
	c.RUnlock()
	if ok {
		log.Printf("%s is already cached", hostname)
		return
	}

	c.Lock()
	defer c.Unlock()
	log.Printf("%s is cached", hostname)
	c.records[hostname] = record{
		ip:   ip,
		time: time.Now(),
	}
}

func (c *customServer) parseQuery(m *dns.Msg) error {
	for _, q := range m.Question {
		switch q.Qtype {
		case dns.TypeA:
			log.Printf("Query for %s\n", q.Name)
			ip, ok := c.readRecord(q.Name)
			if !ok {
				log.Printf("%s is not cached", q.Name)
				var err error

				ip, err = lookupIP(q.Name)
				if err != nil {
					return err
				}

				c.writeRecord(q.Name, ip)
			} else {
				log.Printf("%s is cached", q.Name)
			}

			rr, err := dns.NewRR(fmt.Sprintf("%s A %s", q.Name, ip))
			if err == nil {
				m.Answer = append(m.Answer, rr)
			}
		default:
			return fmt.Errorf("%w - %s is not supported yet\n", ErrNotImplemented, dns.TypeToString[q.Qtype])
		}
	}
	return nil
}

func lookupIP(servername string) (string, error) {
	c := new(dns.Client)
	m := new(dns.Msg)
	m.SetQuestion(servername, dns.TypeA)
	r, _, err := c.Exchange(m, "8.8.8.8:53")
	if err != nil {
		return "", err
	}

	if len(r.Answer) < 1 {
		return "", fmt.Errorf("no A record found for %s", servername)
	}

	if t, ok := r.Answer[0].(*dns.A); ok {
		return t.A.String(), nil
	}

	return "", fmt.Errorf("no A record found for %s", servername)
}

func (c *customServer) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)
	m.Compress = false

	switch r.Opcode {
	case dns.OpcodeQuery:
		err := c.parseQuery(m)
		if err != nil && errors.Is(err, ErrNotImplemented) {
			m.SetRcode(r, dns.RcodeNotImplemented)
		} else if err != nil {
			m.SetRcode(r, dns.RcodeServerFailure)
		} else {
			m.SetRcode(r, dns.RcodeSuccess)
		}
	}

	w.WriteMsg(m)
}

func main() {
	c := customServer{
		records: make(map[string]record),
	}
	// start server
	port := 1773
	server := &dns.Server{
		Addr:    ":" + strconv.Itoa(port),
		Net:     "udp",
		Handler: &c,
	}
	log.Printf("Starting at %d\n", port)

	err := server.ListenAndServe()
	if err != nil {
		log.Fatalf("Failed to start server: %s\n ", err.Error())
	}

	defer server.Shutdown()
}
