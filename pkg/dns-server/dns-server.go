package server

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/miekg/dns"
)

var (
	ErrNotImplemented = fmt.Errorf("not implemented")
	ErrIPLookupFailed = fmt.Errorf("failed to lookup IP")
)

const (
	// Caching for a day.
	timeLimit = 24 * time.Hour

	googleDNS = "8.8.8.8:53"
)

type Record struct {
	IP   string
	Time time.Time
}

type dnsServer struct {
	sync.RWMutex
	records map[string]Record
}

func NewCustomServer() *dnsServer {
	return &dnsServer{
		records: make(map[string]Record),
	}
}

func (c *dnsServer) AddRecord(hostname string, record Record) {
	c.Lock()
	defer c.Unlock()
	c.records[hostname] = record
}

func (c *dnsServer) readRecord(hostname string) (string, bool) {
	c.RLock()
	record, ok := c.records[hostname]
	c.RUnlock()

	if !ok {
		return "", false
	}

	if time.Since(record.Time) >= timeLimit {
		c.Lock()
		delete(c.records, hostname)
		c.Unlock()

		return "", false
	}

	return record.IP, true
}

func (c *dnsServer) writeRecord(hostname, ip string) {
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

	c.records[hostname] = Record{
		IP:   ip,
		Time: time.Now(),
	}
}

func (c *dnsServer) parseQuery(m *dns.Msg) error {
	for _, q := range m.Question {
		switch q.Qtype {
		case dns.TypeA:
			log.Printf("Query for %s\n", q.Name)

			ip, ok := c.readRecord(q.Name)
			if !ok {
				log.Printf("%s is not cached", q.Name)

				var err error

				ip, err = lookupIP(q.Name, googleDNS)
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

func lookupIP(servername, dnsServer string) (string, error) {
	c := new(dns.Client)
	m := new(dns.Msg)
	m.SetQuestion(servername, dns.TypeA)

	r, _, err := c.Exchange(m, dnsServer)
	if err != nil {
		return "", err
	}

	if len(r.Answer) < 1 {
		return "", fmt.Errorf("%w - no A record found for %s", ErrIPLookupFailed, servername)
	}

	if t, ok := r.Answer[0].(*dns.A); ok {
		return t.A.String(), nil
	}

	return "", fmt.Errorf("%w - no A record found for %s", ErrIPLookupFailed, servername)
}

func (c *dnsServer) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
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
