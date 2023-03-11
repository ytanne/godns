package server

import (
	"errors"
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/require"
	"github.com/ytanne/godns/pkg/models"
)

type mockDB struct {
	data map[string]models.Record
}

func NewMockDB() *mockDB {
	return &mockDB{
		data: make(map[string]models.Record),
	}
}

func (m *mockDB) Get(key string) (models.Record, error) {
	var result models.Record

	result, ok := m.data[key]
	if !ok {
		return result, errors.New("not found")
	}

	return result, nil
}

func (m *mockDB) Set(key string, value models.Record) error {
	m.data[key] = value
	return nil
}

func (m *mockDB) Remove(key string) error {
	delete(m.data, key)
	return nil
}

func (m *mockDB) Close() error {
	return nil
}

func TestDNS(t *testing.T) {
	db := NewMockDB()
	c := NewDnsServer(db)
	defer c.Close()

	testDomain := "domain.test."
	testIP := "192.168.0.1"
	c.cache.Set(testDomain, models.Record{
		IP:   "192.168.0.1",
		Time: time.Now(),
	})
	outdatedDomain := "outdated.test."
	c.cache.Set(outdatedDomain, models.Record{
		IP:   "",
		Time: time.Now().Add(-time.Hour * 24),
	})

	result1, _ := formRR(testDomain, testIP, dns.TypeA)
	result3, _ := formRR("example.com.", "93.184.216.34", dns.TypeA)

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
			expectedError: ErrOutdated,
		},
		{
			description:   "invalid query type",
			domain:        testDomain,
			queryType:     dns.TypeMX,
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
