package weather

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/stubbedev/lys/desktop/feed"
	"github.com/stubbedev/lys/desktop/geo"
)

// Settings is everything a poll depends on (the builder's fields and
// the service setters). Keys are already resolved secrets; empty is
// unset.
type Settings struct {
	Interval          time.Duration
	Provider          ProviderKind
	Location          geo.Place
	VisualCrossingKey string
	WeatherAPIKey     string
}

// StatusKind is the fetch lifecycle.
type StatusKind int

// Statuses.
const (
	Loading StatusKind = iota
	Loaded
	Failed
)

// Status is the lifecycle with the failure of Failed: its category and
// the provider missing a key.
type Status struct {
	Kind     StatusKind
	Error    ErrorKind
	Provider string
}

// retry policy.
const (
	maxRetries        = 3
	initialRetryDelay = 5 * time.Second
	rateLimitDelay    = 60 * time.Second
)

// FetchFunc fetches one forecast: the providers, or a stand-in.
type FetchFunc func(ctx context.Context, s Settings, now time.Time) (*Weather, error)

// Service polls the configured provider and caches the last successful
// fetch. After the first poll it only fetches while something is
// subscribed.
type Service struct {
	mu       sync.Mutex
	settings Settings
	weather  *Weather
	status   Status
	cancel   context.CancelFunc
	life     context.Context
	stop     context.CancelFunc
	polls    sync.WaitGroup
	changes  *feed.Tick
	fetch    FetchFunc
	// retryDelay is the backoff; tests shorten it.
	retryDelay func(err error, attempt int) time.Duration
}

// New starts a service polling with s; Close stops it.
func New(s Settings) *Service {
	return newService(s, fetchWeather, retryDelay)
}

// NewWith polls through fetch instead of the providers (a fixture, a
// demo); retries and subscriber gating behave as in New.
func NewWith(s Settings, fetch FetchFunc) *Service {
	return newService(s, fetch, retryDelay)
}

func newService(s Settings, fetch FetchFunc, delay func(error, int) time.Duration) *Service {
	life, stop := context.WithCancel(context.Background())
	svc := &Service{life: life, stop: stop, changes: feed.NewTick(), fetch: fetch, retryDelay: delay}
	svc.mu.Lock()
	svc.restartLocked(s)
	svc.mu.Unlock()
	return svc
}

// Configure applies new settings; unchanged settings keep the running
// poll (a reload that touched other keys does not refetch).
func (s *Service) Configure(next Settings) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if next == s.settings {
		return
	}
	if !next.samePlace(s.settings) {
		s.weather = nil
	}
	s.restartLocked(next)
}

func (s Settings) samePlace(o Settings) bool {
	s.Interval, o.Interval = 0, 0
	return s == o
}

// restartLocked restarts polling: status back to loading, the old
// poll canceled, a new one started.
func (s *Service) restartLocked(next Settings) {
	s.settings = next
	s.status = Status{Kind: Loading}
	if s.cancel != nil {
		s.cancel()
	}
	ctx, cancel := context.WithCancel(s.life)
	s.cancel = cancel
	s.polls.Go(func() { s.poll(ctx, next) })
	feed.Notify(s.changes)
}

// Refresh restarts polling with the current settings: the dropdown's
// refresh and retry buttons.
func (s *Service) Refresh() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.restartLocked(s.settings)
}

// Weather is the last successful fetch; nil before the first.
func (s *Service) Weather() *Weather {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.weather
}

// Status is the fetch lifecycle.
func (s *Service) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// Subscribe ticks whenever Weather or Status changed.
func (s *Service) Subscribe() (<-chan struct{}, func()) { return s.changes.Subscribe() }

// Close stops polling and returns once the polls have, their idle
// connections closed.
func (s *Service) Close() {
	s.stop()
	s.polls.Wait()
	httpClient.CloseIdleConnections()
}

func (s *Service) poll(ctx context.Context, settings Settings) {
	interval := settings.Interval
	if interval <= 0 {
		interval = 30 * time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	first := true
	for {
		if first || s.changes.Len() > 0 {
			first = false
			weather, err := s.fetchWithRetry(ctx, settings)
			if ctx.Err() != nil {
				return
			}
			s.publish(ctx, weather, err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// publish stores one poll's outcome unless the poll was superseded.
func (s *Service) publish(ctx context.Context, weather *Weather, err error) {
	s.mu.Lock()
	if ctx.Err() != nil {
		s.mu.Unlock()
		return
	}
	if err != nil {
		s.status = failedStatus(err)
	} else {
		s.weather = weather
		s.status = Status{Kind: Loaded}
	}
	s.mu.Unlock()
	feed.Notify(s.changes)
}

// failedStatus is the error status with the kind's details.
func failedStatus(err error) Status {
	st := Status{Kind: Failed, Error: ErrOther}
	if e, ok := errors.AsType[*Error](err); ok {
		st.Error, st.Provider = e.Kind, e.Provider
	}
	return st
}

// fetchWithRetry fetches; retryable failures back off and
// retry up to three attempts.
func (s *Service) fetchWithRetry(ctx context.Context, settings Settings) (*Weather, error) {
	for attempt := 1; ; attempt++ {
		weather, err := s.fetch(ctx, settings, time.Now())
		if err == nil {
			return weather, nil
		}
		var e *Error
		if attempt >= maxRetries || !errors.As(err, &e) || !e.Retryable() {
			log.Printf("weather: cannot fetch weather data: %v", err)
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, err
		case <-time.After(s.retryDelay(err, attempt)):
		}
	}
}

// retryDelay doubles from 5s; a rate limit waits a minute.
func retryDelay(err error, attempt int) time.Duration {
	if kindOf(err) == ErrRateLimited {
		return rateLimitDelay
	}
	return initialRetryDelay << (attempt - 1)
}

// fetchWeather fetches from the configured provider; a keyed provider
// without its key is ErrAPIKeyMissing.
func fetchWeather(ctx context.Context, s Settings, now time.Time) (*Weather, error) {
	switch s.Provider {
	case VisualCrossing:
		if s.VisualCrossingKey == "" {
			return nil, apiKeyMissing(visualCrossingProvider)
		}
		return fetchVisualCrossing(ctx, s.VisualCrossingKey, s.Location, now)
	case WeatherAPI:
		if s.WeatherAPIKey == "" {
			return nil, apiKeyMissing(weatherAPIProvider)
		}
		return fetchWeatherAPI(ctx, s.WeatherAPIKey, s.Location, now)
	}
	return fetchOpenMeteo(ctx, s.Location, now)
}
