package server

import (
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
	"github.com/ytanne/godns/pkg/models"
	"go.uber.org/zap"
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

type server interface {
	ListenAndServe() error
	Shutdown() error
}

type handler struct {
	sync.RWMutex
	cache keyDB
}

var _ dns.Handler = (*handler)(nil)

type keyDB interface {
	Get(key string) (models.Record, error)
	Set(key string, value models.Record) error
	Remove(key string) error
}

type dnsServer struct {
	server server
	log    *zap.Logger
}

func WithLogger(logger *zap.Logger) func(d *dnsServer) {
	return func(d *dnsServer) {
		d.log = logger
	}
}

func NewDnsServer(dnsPort string, cache keyDB, sets ...func(d *dnsServer)) *dnsServer {
	h := &handler{
		cache: cache,
	}
	s := &dns.Server{
		Addr:    ":" + dnsPort,
		Net:     "udp",
		Handler: h,
	}

	ds := &dnsServer{
		server: s,
	}

	for _, set := range sets {
		set(ds)
	}

	return ds
}

func (d *dnsServer) ListenAndServe() error {
	return d.server.ListenAndServe()
}

func (d *dnsServer) Shutdown() error {
	return d.server.Shutdown()
}

func (c *handler) readRecord(hostname string) (string, error) {
	c.RLock()
	record, err := c.cache.Get(hostname)
	c.RUnlock()

	if err != nil && strings.Contains(err.Error(), "not found") {
		return "", ErrNotFound
	}

	if time.Since(record.Time) >= timeLimit {
		c.Lock()
		c.cache.Remove(hostname)
		c.Unlock()

		return "", ErrOutdated
	}

	return record.IP, nil
}

func (c *handler) writeRecord(hostname, ip string) error {
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
		Domain: hostname,
		IP:     ip,
		Time:   time.Now(),
	})
}

func (c *handler) parseQuery(m *dns.Msg) error {
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
					return fmt.Errorf("%w - could not resolve %s query - %s", ErrIPLookupFailed, dns.TypeToString[q.Qtype], err)
				}

				c.writeRecord(q.Name, ip)
			} else {
				log.Println("reading from cache failed:", err)

				return err
			}

			rr, err := formRR(m.Question[0].Name, ip, dns.TypeA)
			if err != nil {
				return err
			}

			m.Answer = append(m.Answer, rr)
		case dns.TypeAAAA:
			ip, err := lookupIP(q.Name, googleDNS, dns.TypeAAAA)
			if err != nil {
				return fmt.Errorf("%w - could not resolve %s query - %s", ErrIPLookupFailed, dns.TypeToString[q.Qtype], err)
			}

			rr, err := formRR(m.Question[0].Name, ip, dns.TypeAAAA)
			if err != nil {
				return err
			}

			m.Answer = append(m.Answer, rr)
		default:
			log.Printf("obtained strange query %s", dns.TypeToString[q.Qtype])

			return fmt.Errorf("%w - %s is not supported yet", ErrNotImplemented, dns.TypeToString[q.Qtype])
		}

		log.Printf("%s query for %s was processed successfully", dns.TypeToString[q.Qtype], q.Name)
	}

	return nil
}

func formRR(hostname, ip string, qType uint16) (dns.RR, error) {
	var rr dns.RR

	switch qType {
	case dns.TypeA:
		r := new(dns.A)
		r.Hdr = dns.RR_Header{Name: hostname, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: uint32(defaultTtl)}
		r.A = net.ParseIP(ip)
		rr = r
	case dns.TypeAAAA:
		r := new(dns.AAAA)
		r.Hdr = dns.RR_Header{Name: hostname, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: uint32(defaultTtl)}
		r.AAAA = net.ParseIP(ip)
		rr = r
	default:
		return nil, errors.New("unknown query type")
	}

	return rr, nil
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

func (c *handler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
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

func (c *handler) Close() {
}
