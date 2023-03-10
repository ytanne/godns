package server

import (
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/miekg/dns"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/ytanne/godns/pkg/models"
)

var (
	ErrNotImplemented = fmt.Errorf("not implemented")
	ErrIPLookupFailed = fmt.Errorf("failed to lookup IP")
	ErrNotFound       = fmt.Errorf("not found")
	ErrOutdated       = fmt.Errorf("outdated")
)

const (
	// Caching for a day.
	timeLimit = 24 * time.Hour

	googleDNS  = "8.8.8.8:53"
	defaultTtl = 86400 // one day.
)

type keyDB interface {
	Get(key string) (models.Record, error)
	Set(key string, value models.Record) error
	Remove(key string) error
	Close() error
}

type dnsServer struct {
	sync.RWMutex
	cache keyDB
}

func NewDnsServer(cache keyDB) *dnsServer {
	return &dnsServer{
		cache: cache,
	}
}

func (c *dnsServer) readRecord(hostname string) (string, error) {
	c.RLock()
	record, err := c.cache.Get(hostname)
	c.RUnlock()

	if err != nil && errors.Is(err, leveldb.ErrNotFound) {
		return "", ErrNotFound
	}
	// record, ok := c.records[hostname]

	if time.Since(record.Time) >= timeLimit {
		c.Lock()
		c.cache.Remove(hostname)
		c.Unlock()

		return "", ErrOutdated
	}

	return record.IP, nil
}

func (c *dnsServer) writeRecord(hostname, ip string) error {
	c.RLock()
	_, err := c.cache.Get(hostname)
	c.RUnlock()

	if err == nil {
		log.Printf("%s is already cached", hostname)

		return nil
	}

	c.Lock()
	defer c.Unlock()
	log.Printf("%s is cached", hostname)

	return c.cache.Set(hostname, models.Record{
		IP:   ip,
		Time: time.Now(),
	})
}

func (c *dnsServer) parseQuery(m *dns.Msg) error {
	for _, q := range m.Question {
		log.Printf("%s query for %s", dns.TypeToString[q.Qtype], q.Name)

		switch q.Qtype {
		case dns.TypeA:
			ip, err := c.readRecord(q.Name)
			if err == nil {
				log.Printf("%s is cached", q.Name)
			} else if err != nil && errors.Is(err, ErrNotFound) {
				log.Printf("%s is not cached", q.Name)

				var err error

				ip, err = lookupIP(q.Name, googleDNS, dns.TypeA)
				if err != nil {
					return fmt.Errorf("could not resolve %s query - %w", dns.TypeToString[q.Qtype], err)
				}

				c.writeRecord(q.Name, ip)
			} else {
				log.Println("reading from cache failed:", err)

				return err
			}

			rr := new(dns.A)
			rr.Hdr = dns.RR_Header{Name: m.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: uint32(defaultTtl)}
			rr.A = net.ParseIP(ip)

			m.Answer = append(m.Answer, rr)
		case dns.TypeAAAA:
			ip, err := lookupIP(q.Name, googleDNS, dns.TypeAAAA)
			if err != nil {
				return fmt.Errorf("could not resolve %s query - %w", dns.TypeToString[q.Qtype], err)
			}

			rr := new(dns.AAAA)
			rr.Hdr = dns.RR_Header{Name: m.Question[0].Name, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: uint32(defaultTtl)}
			rr.AAAA = net.ParseIP(ip)

			m.Answer = append(m.Answer, rr)
		default:
			log.Println("obtained strange query %s", dns.TypeToString[q.Qtype])

			return fmt.Errorf("%w - %s is not supported yet", ErrNotImplemented, dns.TypeToString[q.Qtype])
		}

		log.Printf("%s query for %s was processed successfully", dns.TypeToString[q.Qtype], q.Name)
	}

	return nil
}

func lookupIP(servername, dnsServer string, reqType uint16) (string, error) {
	c := new(dns.Client)
	m := new(dns.Msg)
	m.SetQuestion(servername, reqType)

	r, _, err := c.Exchange(m, dnsServer)
	if err != nil {
		return "", err
	}

	if len(r.Answer) < 1 {
		return "", fmt.Errorf("%w - no A record found for %s", ErrIPLookupFailed, servername)
	}

	if t, ok := r.Answer[0].(*dns.A); ok {
		return t.A.String(), nil
	} else if t, ok := r.Answer[0].(*dns.AAAA); ok {
		return t.AAAA.String(), nil
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
			log.Println("Is not implemented:", err)

			m.SetRcode(r, dns.RcodeNotImplemented)
		} else if err != nil {
			log.Println("Server failed:", err)

			m.SetRcode(r, dns.RcodeServerFailure)
		} else {
			m.SetRcode(r, dns.RcodeSuccess)
		}
	}

	w.WriteMsg(m)
}

func (c *dnsServer) Close() {
	c.cache.Close()
}
