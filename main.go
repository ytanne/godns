package main

import (
	"errors"
	"fmt"
	"log"
	"strconv"

	"github.com/miekg/dns"
)

var (
	ErrNotImplemented = fmt.Errorf("not implemented")
)

type customServer struct{}

var records = map[string]string{}

func parseQuery(m *dns.Msg) error {
	for _, q := range m.Question {
		switch q.Qtype {
		case dns.TypeA:
			log.Printf("Query for %s\n", q.Name)
			ip, ok := records[q.Name]
			if !ok {
				log.Printf("%s is not cached", q.Name)
				var err error
				ip, err = lookupIP(q.Name)
				if err != nil {
					return err
				}
				records[q.Name] = ip
				log.Printf("%s is added to cache", q.Name)
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
		err := parseQuery(m)
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
	var c customServer
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
