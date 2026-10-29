package storage

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const HealthInterval = 10 * time.Second

var healthClient = &http.Client{
	Timeout:       3 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	Transport: &http.Transport{
		Proxy:                 nil,
		DialContext:           (&net.Dialer{Timeout: 2 * time.Second}).DialContext,
		TLSHandshakeTimeout:   2 * time.Second,
		ResponseHeaderTimeout: 2 * time.Second,
		MaxIdleConns:          16, MaxIdleConnsPerHost: 2, IdleConnTimeout: 30 * time.Second,
	},
}

// ParseTarget accepts the original host:port format as well as HTTP(S) URLs.
func ParseTarget(target string) (*url.URL, error) {
	target = strings.TrimSpace(target)
	if !strings.Contains(target, "://") {
		target = "http://" + target
	}
	u, err := url.Parse(target)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return nil, fmt.Errorf("Укажите адрес HTTP(S), например localhost:8099")
	}
	return u, nil
}

func (s *RuleStore) StartHealthChecks(ctx context.Context) {
	for {
		s.CheckServices(ctx)
		timer := time.NewTimer(HealthInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

// A bounded pool probes without holding the rule lock during network I/O.
func (s *RuleStore) CheckServices(ctx context.Context) {
	s.healthMu.Lock()
	defer s.healthMu.Unlock()
	rules := s.All()
	jobs := make(chan *Rule)
	var wg sync.WaitGroup
	for i := 0; i < min(8, len(rules)); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for rule := range jobs {
				revision := rule.healthRevision
				err := probeService(ctx, rule)
				if ctx.Err() != nil {
					continue
				}
				s.applyHealth(rule, revision, err, time.Now())
			}
		}()
	}
	for _, rule := range rules {
		if ctx.Err() != nil {
			break
		}
		jobs <- rule
	}
	close(jobs)
	wg.Wait()
}

func probeService(ctx context.Context, rule *Rule) error {
	u, err := ParseTarget(rule.Target)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf("Некорректный адрес сервиса")
	}
	// Match the normal proxy Host behavior, including WorkService's trusted domain.
	if strings.EqualFold(rule.Host, "appworks.avayusstroi.by") {
		req.Host = "appworks.avayusstroi.by"
	}
	req.Header.Set("User-Agent", "Router-Healthcheck/1.0")
	req.Header.Set("Cache-Control", "no-cache")
	response, err := healthClient.Do(req)
	if err != nil {
		return fmt.Errorf("Нет ответа от сервиса")
	}
	defer response.Body.Close()
	// Read only a small part: a health check must not download a large response.
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	if response.StatusCode >= 500 {
		return fmt.Errorf("Сервис отвечает HTTP %d", response.StatusCode)
	}
	return nil // Login redirects, 401, 403 and 404 still prove the server is alive.
}

func (s *RuleStore) applyHealth(snapshot *Rule, revision uint64, err error, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.rules[snapshot.Host]
	if !ok || current.Target != snapshot.Target || current.healthGeneration != snapshot.healthGeneration || current.healthRevision != revision {
		return
	}
	current.HealthCheckedAt = now
	if err != nil {
		current.healthFailures++
		current.healthSuccesses = 0
		current.HealthError = err.Error()
		if current.healthFailures >= 2 {
			current.ServiceDown = true
		}
	} else {
		current.healthFailures = 0
		current.healthSuccesses++
		current.HealthError = ""
		if current.healthSuccesses >= 2 {
			current.ServiceDown = false
		}
	}
	current.healthRevision++
}

// Proxy transport failures take effect immediately; manual maintenance is untouched.
func (s *RuleStore) ReportServiceFailure(host, target string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.rules[host]
	if !ok || current.Target != target {
		return
	}
	current.ServiceDown = true
	current.healthFailures = 2
	current.healthSuccesses = 0
	current.HealthError = "Соединение с сервисом прервано"
	current.HealthCheckedAt = time.Now()
	current.healthRevision++
}
