package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/require"
)

func TestDNS(t *testing.T) {
	c := NewCustomServer()

	testDomain := "domain.test"
	testIP := "192.168.0.1"
	c.records[testDomain] = record{
		ip:   "192.168.0.1",
		time: time.Now(),
	}
	outdatedDomain := "outdated.test."
	c.records[outdatedDomain] = record{
		ip:   "",
		time: time.Now().Add(-time.Hour * 24),
	}

	result1, _ := dns.NewRR(fmt.Sprintf("%s A %s", testDomain, testIP))
	result3, _ := dns.NewRR(fmt.Sprintf("%s A %s", "example.com.", "93.184.216.34"))

	tss := []struct {
		description       string
		domain            string
		queryType         uint16
		expectedResult    any
		expectedLenResult int
		expectedError     error
	}{
		{
			description:       "cached result",
			domain:            testDomain,
			queryType:         dns.TypeA,
			expectedResult:    result1,
			expectedLenResult: 1,
		},
		{
			description:   "testing outdated invalid record scenario",
			domain:        outdatedDomain,
			queryType:     dns.TypeA,
			expectedError: ErrIPLookupFailed,
		},
		{
			description:   "invalid query type",
			domain:        testDomain,
			queryType:     dns.TypeAAAA,
			expectedError: ErrNotImplemented,
		},
		{
			description:       "valid query",
			domain:            "example.com.",
			queryType:         dns.TypeA,
			expectedLenResult: 1,
			expectedResult:    result3,
		},
	}

	for _, ts := range tss {
		t.Run(ts.description, func(t *testing.T) {
			m := new(dns.Msg)
			m.SetQuestion(ts.domain, ts.queryType)

			require.ErrorIs(t, c.parseQuery(m), ts.expectedError)
			if ts.expectedError != nil {
				return
			}
			require.Len(t, m.Answer, ts.expectedLenResult)
			require.Equal(t, ts.expectedResult, m.Answer[0])
		})
	}
}
