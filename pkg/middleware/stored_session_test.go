package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"time"

	middlewareapi "github.com/oauth2-proxy/oauth2-proxy/v7/pkg/apis/middleware"
	sessionsapi "github.com/oauth2-proxy/oauth2-proxy/v7/pkg/apis/sessions"
	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/clock"
	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/providerloader/util"
	"github.com/oauth2-proxy/oauth2-proxy/v7/providers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

type testLock struct {
	locked          bool
	obtainOnAttempt int
	obtainAttempts  int
	obtainError     error
}

type TestProvider struct {
	*providers.ProviderData
	EmailAddress     string
	ValidToken       bool
	RefreshToken     string
	AccessToken      string
	Concurrent       bool
	SessionRefreshed bool
	GroupValidator   func(string) bool
}

var _ providers.Provider = (*TestProvider)(nil)

func NewTestProvider(providerURL *url.URL, emailAddress string) *TestProvider {
	return &TestProvider{
		ProviderData: &providers.ProviderData{
			ProviderName: "Test Provider",
			LoginURL: &url.URL{
				Scheme: "http",
				Host:   providerURL.Host,
				Path:   "/oauth/authorize",
			},
			RedeemURL: &url.URL{
				Scheme: "http",
				Host:   providerURL.Host,
				Path:   "/oauth/token",
			},
			ProfileURL: &url.URL{
				Scheme: "http",
				Host:   providerURL.Host,
				Path:   "/api/v1/profile",
			},
			Scope: "profile.email",
		},
		EmailAddress: emailAddress,
		Concurrent:   false,
		GroupValidator: func(s string) bool {
			return true
		},
		ValidToken: true,
	}
}

func (tp *TestProvider) GetEmailAddress(_ context.Context, _ *sessionsapi.SessionState) (string, error) {
	return tp.EmailAddress, nil
}

func (tp *TestProvider) ValidateSession(ctx context.Context, ss *sessionsapi.SessionState) bool {
	return tp.ValidToken
}

const (
	refresh        = "Refresh"
	noRefresh      = "NoRefresh"
	notImplemented = "NotImplemented"
	providerEmail  = "provider@example.com"
)

func (tp *TestProvider) RefreshSession(_ context.Context, _ *sessionsapi.SessionState) (bool, error) {
	if tp.Concurrent {
		time.Sleep(10 * time.Millisecond)
		tp.SessionRefreshed = true
		return true, nil
	}

	switch tp.RefreshToken {
	case refresh:
		tp.SessionRefreshed = true
		return true, nil
	case noRefresh:
		return false, nil
	case notImplemented:
		return false, errors.New("error refreshing tokens")
	default:
		return false, errors.New("error refreshing session")
	}

}

func (l *testLock) Obtain(_ context.Context, _ time.Duration) error {
	l.obtainAttempts++
	if l.obtainAttempts < l.obtainOnAttempt {
		return sessionsapi.ErrLockNotObtained
	}
	if l.obtainError != nil {
		return l.obtainError
	}
	l.locked = true
	return nil
}

func (l *testLock) Peek(_ context.Context) (bool, error) {
	return l.locked, nil
}

func (l *testLock) Refresh(_ context.Context, _ time.Duration) error {
	return nil
}

func (l *testLock) Release(_ context.Context) error {
	l.locked = false
	return nil
}

type testLockConcurrent struct {
	mu     sync.RWMutex
	locked bool
}

func (l *testLockConcurrent) Obtain(_ context.Context, _ time.Duration) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.locked {
		return sessionsapi.ErrLockNotObtained
	}
	l.locked = true
	return nil
}

func (l *testLockConcurrent) Peek(_ context.Context) (bool, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.locked, nil
}

func (l *testLockConcurrent) Refresh(_ context.Context, _ time.Duration) error {
	return nil
}

func (l *testLockConcurrent) Release(_ context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.locked = false
	return nil
}

var _ = Describe("Stored Session Suite", func() {
	const (
		refresh        = "Refresh"
		refreshed      = "Refreshed"
		noRefresh      = "NoRefresh"
		notImplemented = "NotImplemented"
	)

	var ctx = context.Background()

	Context("StoredSessionLoader", func() {
		now := time.Now()
		createdPast := now.Add(-5 * time.Minute)
		createdFuture := now.Add(5 * time.Minute)

		var defaultSessionStore = &fakeSessionStore{
			LoadFunc: func(req *http.Request) (*sessionsapi.SessionState, error) {
				switch req.Header.Get("Cookie") {
				case "_oauth2_proxy=NoRefreshSession":
					return &sessionsapi.SessionState{
						RefreshToken: noRefresh,
						CreatedAt:    &createdPast,
						ExpiresOn:    &createdFuture,
						Lock:         &sessionsapi.NoOpLock{},
					}, nil
				case "_oauth2_proxy=InvalidNoRefreshSession":
					return nil, nil
				case "_oauth2_proxy=ExpiredNoRefreshSession":
					return nil, nil
				case "_oauth2_proxy=RefreshSession":
					return &sessionsapi.SessionState{
						RefreshToken: refresh,
						CreatedAt:    &createdPast,
						ExpiresOn:    &createdFuture,
					}, nil
				case "_oauth2_proxy=NeedRefreshSession":
					return &sessionsapi.SessionState{
						RefreshToken: "Refreshed",
						CreatedAt:    &now,
						ExpiresOn:    &createdFuture,
						Lock:         &sessionsapi.NoOpLock{},
					}, nil
				case "_oauth2_proxy=RefreshError":
					return &sessionsapi.SessionState{
						RefreshToken: "RefreshError",
						CreatedAt:    &createdPast,
						ExpiresOn:    &createdFuture,
						Lock:         &sessionsapi.NoOpLock{},
					}, nil
				case "_oauth2_proxy=NonExistent":
					return nil, fmt.Errorf("invalid cookie")
				case "_oauth2_proxy=RefreshErrorValidationError":
					return nil, nil
				default:
					return nil, nil
				}
			},
		}

		BeforeEach(func() {
			clock.Set(now)
		})

		AfterEach(func() {
			clock.Reset()
		})

		type storedSessionLoaderTableInput struct {
			requestHeaders  http.Header
			existingSession *sessionsapi.SessionState
			expectedSession *sessionsapi.SessionState
			store           sessionsapi.SessionStore
			refreshPeriod   time.Duration
		}

		DescribeTable("when serving a request",
			func(in storedSessionLoaderTableInput) {
				scope := &middlewareapi.RequestScope{
					Session: in.existingSession,
				}

				// Set up the request with the request header and a request scope
				req := httptest.NewRequest("", "/", nil)
				req.Header = in.requestHeaders
				req = middlewareapi.AddRequestScope(req, scope)

				rw := httptest.NewRecorder()

				opts := &StoredSessionLoaderOptions{
					SessionStore:  in.store,
					RefreshPeriod: in.refreshPeriod,
				}

				// Create the handler with a next handler that will capture the session
				// from the scope
				var gotSession *sessionsapi.SessionState
				tp := NewTestProvider(&url.URL{Host: "www.example.com"}, providerEmail)
				tp.RefreshToken = req.Header.Get("Cookie")

				req = req.WithContext(util.AppendToContext(req.Context(), tp))

				handler := NewStoredSessionLoader(opts)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					gotSession = middlewareapi.GetRequestScope(r).Session
				}))

				handler.ServeHTTP(rw, req)

				Expect(gotSession).To(Equal(in.expectedSession))
			},
			Entry("with no cookie", storedSessionLoaderTableInput{
				requestHeaders:  http.Header{},
				existingSession: nil,
				expectedSession: nil,
				store:           defaultSessionStore,
				refreshPeriod:   1 * time.Minute,
			}),
			Entry("with an invalid cookie", storedSessionLoaderTableInput{
				requestHeaders: http.Header{
					"Cookie": []string{"_oauth2_proxy=NonExistent"},
				},
				existingSession: nil,
				expectedSession: nil,
				store:           defaultSessionStore,
				refreshPeriod:   1 * time.Minute,
			}),
			Entry("with an existing session", storedSessionLoaderTableInput{
				requestHeaders: http.Header{
					"Cookie": []string{"_oauth2_proxy=RefreshSession"},
				},
				existingSession: &sessionsapi.SessionState{
					RefreshToken: "Existing",
				},
				expectedSession: &sessionsapi.SessionState{
					RefreshToken: "Existing",
				},
				store:         defaultSessionStore,
				refreshPeriod: 1 * time.Minute,
			}),
			Entry("with a session that has not expired", storedSessionLoaderTableInput{
				requestHeaders: http.Header{
					"Cookie": []string{"_oauth2_proxy=NoRefreshSession"},
				},
				existingSession: nil,
				expectedSession: &sessionsapi.SessionState{
					RefreshToken: noRefresh,
					CreatedAt:    &createdPast,
					ExpiresOn:    &createdFuture,
					Lock:         &sessionsapi.NoOpLock{},
				},
				store:         defaultSessionStore,
				refreshPeriod: 1 * time.Minute,
			}),
			Entry("with a session that cannot refresh and has expired", storedSessionLoaderTableInput{
				requestHeaders: http.Header{
					"Cookie": []string{"_oauth2_proxy=ExpiredNoRefreshSession"},
				},
				existingSession: nil,
				expectedSession: nil,
				store:           defaultSessionStore,
				refreshPeriod:   1 * time.Minute,
			}),
			Entry("with a session that can refresh, but is younger than refresh period", storedSessionLoaderTableInput{
				requestHeaders: http.Header{
					"Cookie": []string{"_oauth2_proxy=RefreshSession"},
				},
				existingSession: nil,
				expectedSession: &sessionsapi.SessionState{
					RefreshToken: refresh,
					CreatedAt:    &createdPast,
					ExpiresOn:    &createdFuture,
				},
				store:         defaultSessionStore,
				refreshPeriod: 10 * time.Minute,
			}),
			Entry("with a session that can refresh and is older than the refresh period", storedSessionLoaderTableInput{
				requestHeaders: http.Header{
					"Cookie": []string{"_oauth2_proxy=NeedRefreshSession"},
				},
				existingSession: nil,
				expectedSession: &sessionsapi.SessionState{
					RefreshToken: "Refreshed",
					CreatedAt:    &now,
					ExpiresOn:    &createdFuture,
					Lock:         &sessionsapi.NoOpLock{},
				},
				store:         defaultSessionStore,
				refreshPeriod: 1 * time.Minute,
			}),
			Entry("when the provider refresh fails but validation succeeds", storedSessionLoaderTableInput{
				requestHeaders: http.Header{
					"Cookie": []string{"_oauth2_proxy=RefreshError"},
				},
				existingSession: nil,
				expectedSession: &sessionsapi.SessionState{
					RefreshToken: "RefreshError",
					CreatedAt:    &createdPast,
					ExpiresOn:    &createdFuture,
					Lock:         &sessionsapi.NoOpLock{},
				},
				store:         defaultSessionStore,
				refreshPeriod: 1 * time.Minute,
			}),
			Entry("when the provider refresh fails and validation fails", storedSessionLoaderTableInput{
				requestHeaders: http.Header{
					"Cookie": []string{"_oauth2_proxy=RefreshErrorValidationError"},
				},
				existingSession: nil,
				expectedSession: nil,
				store:           defaultSessionStore,
				refreshPeriod:   1 * time.Minute,
			}),
			Entry("when the session is not refreshed and is no longer valid", storedSessionLoaderTableInput{
				requestHeaders: http.Header{
					"Cookie": []string{"_oauth2_proxy=InvalidNoRefreshSession"},
				},
				existingSession: nil,
				expectedSession: nil,
				store:           defaultSessionStore,
				refreshPeriod:   1 * time.Minute,
			}),
		)

		type storedSessionLoaderConcurrentTableInput struct {
			existingSession *sessionsapi.SessionState
			refreshPeriod   time.Duration
			numConcReqs     int
		}

		DescribeTable("when serving concurrent requests",
			func(in storedSessionLoaderConcurrentTableInput) {
				lockConc := &testLockConcurrent{}

				lock := &sync.RWMutex{}
				existingSession := *in.existingSession // deep copy existingSession state
				existingSession.Lock = lockConc
				store := &fakeSessionStore{
					LoadFunc: func(req *http.Request) (*sessionsapi.SessionState, error) {
						lock.RLock()
						defer lock.RUnlock()
						session := existingSession
						return &session, nil
					},
					SaveFunc: func(_ http.ResponseWriter, _ *http.Request, session *sessionsapi.SessionState) error {
						lock.Lock()
						defer lock.Unlock()
						existingSession = *session
						return nil
					},
				}

				refreshedChan := make(chan bool, in.numConcReqs)

				tp := NewTestProvider(&url.URL{Host: "www.example.com"}, providerEmail)
				tp.Concurrent = true

				for i := 0; i < in.numConcReqs; i++ {
					go func(refreshedChan chan bool, lockConc sessionsapi.Lock) {
						scope := &middlewareapi.RequestScope{
							Session: nil,
						}

						// Set up the request with the request header and a request scope
						req := httptest.NewRequest("", "/", nil)
						req = middlewareapi.AddRequestScope(req, scope)

						ctx := util.AppendToContext(req.Context(), tp)
						req = req.WithContext(ctx)

						rw := httptest.NewRecorder()

						opts := &StoredSessionLoaderOptions{
							SessionStore:  store,
							RefreshPeriod: in.refreshPeriod,
						}

						handler := NewStoredSessionLoader(opts)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
						handler.ServeHTTP(rw, req)

						lock.Lock()
						defer lock.Unlock()
						refreshedChan <- tp.SessionRefreshed
						tp.SessionRefreshed = false
					}(refreshedChan, lockConc)
				}
				var refreshedSlice []bool
				for i := 0; i < in.numConcReqs; i++ {
					refreshedSlice = append(refreshedSlice, <-refreshedChan)
				}
				sessionRefreshedCount := 0
				for _, sessionRefreshed := range refreshedSlice {
					if sessionRefreshed {
						sessionRefreshedCount++
					}
				}
				Expect(sessionRefreshedCount).To(Equal(1))
			},
			Entry("with two concurrent requests", storedSessionLoaderConcurrentTableInput{
				existingSession: &sessionsapi.SessionState{
					RefreshToken: refresh,
					CreatedAt:    &createdPast,
				},
				numConcReqs:   2,
				refreshPeriod: 1 * time.Minute,
			}),
			Entry("with 5 concurrent requests", storedSessionLoaderConcurrentTableInput{
				existingSession: &sessionsapi.SessionState{
					RefreshToken: refresh,
					CreatedAt:    &createdPast,
				},
				numConcReqs:   5,
				refreshPeriod: 1 * time.Minute,
			}),
			Entry("with one request", storedSessionLoaderConcurrentTableInput{
				existingSession: &sessionsapi.SessionState{
					RefreshToken: refresh,
					CreatedAt:    &createdPast,
				},
				numConcReqs:   1,
				refreshPeriod: 1 * time.Minute,
			}),
		)
	})

	Context("refreshSessionIfNeeded", func() {
		type refreshSessionIfNeededTableInput struct {
			refreshPeriod            time.Duration
			session                  *sessionsapi.SessionState
			concurrentSessionRefresh bool
			expectedErr              error
			expectRefreshed          bool
			expectValidated          bool
			expectedLockObtained     bool
		}

		createdPast := time.Now().Add(-5 * time.Minute)
		createdFuture := time.Now().Add(5 * time.Minute)

		DescribeTable("with a session",
			func(in refreshSessionIfNeededTableInput) {

				session := &sessionsapi.SessionState{}
				*session = *in.session
				if in.concurrentSessionRefresh {
					// Update the session that Load returns.
					// This simulates a concurrent refresh in the background.
					session.CreatedAt = &createdFuture
				}
				store := &fakeSessionStore{
					LoadFunc: func(req *http.Request) (*sessionsapi.SessionState, error) {
						// Loading the session from the provider creates a new lock
						session.Lock = &testLock{}
						return session, nil
					},
					SaveFunc: func(_ http.ResponseWriter, _ *http.Request, s *sessionsapi.SessionState) error {
						*session = *s
						return nil
					},
				}

				s := &storedSessionLoader{
					refreshPeriod: in.refreshPeriod,
					store:         store,
				}

				provider := NewTestProvider(&url.URL{Host: "www.example.com"}, providerEmail)
				provider.AccessToken = in.session.AccessToken
				provider.RefreshToken = in.session.RefreshToken
				provider.ValidToken = in.expectValidated
				req := httptest.NewRequest("", "/", nil)
				err := s.refreshSessionIfNeeded(nil, req, provider, in.session)
				if in.expectedErr != nil {
					Expect(err).To(MatchError(in.expectedErr))
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
				Expect(provider.SessionRefreshed).To(Equal(in.expectRefreshed))
				Expect(provider.ValidToken).To(Equal(in.expectValidated))
				testLock, ok := in.session.Lock.(*testLock)
				Expect(ok).To(Equal(true))

				if in.expectedLockObtained {
					Expect(testLock.obtainAttempts).Should(BeNumerically(">", 0), "Expected at least one attempt at obtaining the session lock")
				}
				Expect(testLock.locked).To(BeFalse(), "Expected lock should always be released")
			},
			Entry("when the refresh period is 0, and the session does not need refreshing", refreshSessionIfNeededTableInput{
				refreshPeriod: time.Duration(0),
				session: &sessionsapi.SessionState{
					RefreshToken: refresh,
					CreatedAt:    &createdFuture,
					Lock:         &testLock{},
				},
				expectedErr:          nil,
				expectRefreshed:      false,
				expectValidated:      false,
				expectedLockObtained: false,
			}),
			Entry("when the refresh period is 0, and the session needs refreshing", refreshSessionIfNeededTableInput{
				refreshPeriod: time.Duration(0),
				session: &sessionsapi.SessionState{
					RefreshToken: refresh,
					CreatedAt:    &createdPast,
					Lock:         &testLock{},
				},
				expectedErr:          nil,
				expectRefreshed:      false,
				expectValidated:      false,
				expectedLockObtained: false,
			}),
			Entry("when the session does not need refreshing", refreshSessionIfNeededTableInput{
				refreshPeriod: 1 * time.Minute,
				session: &sessionsapi.SessionState{
					RefreshToken: refresh,
					CreatedAt:    &createdFuture,
					Lock:         &testLock{},
				},
				expectedErr:          nil,
				expectRefreshed:      false,
				expectValidated:      false,
				expectedLockObtained: false,
			}),
			Entry("when the session is refreshed by the provider", refreshSessionIfNeededTableInput{
				refreshPeriod: 1 * time.Minute,
				session: &sessionsapi.SessionState{
					RefreshToken: refresh,
					CreatedAt:    &createdPast,
					Lock:         &testLock{},
				},
				expectedErr:          nil,
				expectRefreshed:      true,
				expectValidated:      true,
				expectedLockObtained: true,
			}),
			Entry("when obtaining lock failed, but concurrent request refreshed", refreshSessionIfNeededTableInput{
				refreshPeriod: 1 * time.Minute,
				session: &sessionsapi.SessionState{
					RefreshToken: noRefresh,
					CreatedAt:    &createdPast,
					Lock: &testLock{
						obtainOnAttempt: 4,
					},
				},
				concurrentSessionRefresh: true,
				expectedErr:              nil,
				expectRefreshed:          false,
				expectValidated:          false,
				expectedLockObtained:     true,
			}),
			Entry("when obtaining lock failed with a valid session", refreshSessionIfNeededTableInput{
				refreshPeriod: 1 * time.Minute,
				session: &sessionsapi.SessionState{
					RefreshToken: noRefresh,
					CreatedAt:    &createdPast,
					Lock: &testLock{
						obtainError: sessionsapi.ErrLockNotObtained,
					},
				},
				expectedErr:          errors.New("timeout obtaining session lock"),
				expectRefreshed:      false,
				expectValidated:      false,
				expectedLockObtained: true,
			}),
			Entry("when the session is not refreshed by the provider", refreshSessionIfNeededTableInput{
				refreshPeriod: 1 * time.Minute,
				session: &sessionsapi.SessionState{
					RefreshToken: noRefresh,
					CreatedAt:    &createdPast,
					ExpiresOn:    &createdFuture,
					Lock:         &testLock{},
				},
				expectedErr:          errors.New("session is invalid"),
				expectRefreshed:      false,
				expectValidated:      false,
				expectedLockObtained: true,
			}),
			Entry("when the provider doesn't implement refresh", refreshSessionIfNeededTableInput{
				refreshPeriod: 1 * time.Minute,
				session: &sessionsapi.SessionState{
					RefreshToken: notImplemented,
					CreatedAt:    &createdPast,
					Lock:         &testLock{},
				},
				expectedErr:          errors.New("session is invalid"),
				expectRefreshed:      false,
				expectValidated:      false,
				expectedLockObtained: true,
			}),
			Entry("when the session is not refreshed by the provider", refreshSessionIfNeededTableInput{
				refreshPeriod: 1 * time.Minute,
				session: &sessionsapi.SessionState{
					AccessToken:  "Invalid",
					RefreshToken: noRefresh,
					CreatedAt:    &createdPast,
					ExpiresOn:    &createdFuture,
					Lock:         &testLock{},
				},
				expectedErr:          errors.New("session is invalid"),
				expectRefreshed:      false,
				expectValidated:      false,
				expectedLockObtained: true,
			}),
		)
	})

	Context("refreshSession", func() {
		type refreshSessionWithProviderTableInput struct {
			session              *sessionsapi.SessionState
			expectedErr          error
			expectSaved          bool
			expectedLockObtained bool
		}

		now := time.Now()

		DescribeTable("when refreshing with the provider",
			func(in refreshSessionWithProviderTableInput) {
				saved := false

				s := &storedSessionLoader{
					store: &fakeSessionStore{
						SaveFunc: func(_ http.ResponseWriter, _ *http.Request, ss *sessionsapi.SessionState) error {
							saved = true
							if ss.AccessToken == "NoSave" {
								return errors.New("unable to save session")
							}
							return nil
						},
					},
				}

				req := httptest.NewRequest("", "/", nil)
				req = middlewareapi.AddRequestScope(req, &middlewareapi.RequestScope{})

				provider := NewTestProvider(&url.URL{Host: "www.example.com"}, providerEmail)
				provider.RefreshToken = in.session.RefreshToken
				provider.AccessToken = in.session.AccessToken
				err := s.refreshSession(nil, req, provider, in.session)
				if in.expectedErr != nil {
					Expect(err).To(MatchError(in.expectedErr))
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
				Expect(saved).To(Equal(in.expectSaved))

				testLock, ok := in.session.Lock.(*testLock)
				Expect(ok).To(Equal(true))
				if in.expectedLockObtained {
					Expect(testLock.obtainAttempts).Should(BeNumerically(">", 0), "Expected at least one attempt at obtaining the session lock")
				}
				Expect(testLock.locked).To(BeFalse(), "Expected lock should always be released")
			},
			Entry("when the provider does not refresh the session", refreshSessionWithProviderTableInput{
				session: &sessionsapi.SessionState{
					RefreshToken: noRefresh,
					Lock:         &testLock{},
				},
				expectedErr:          nil,
				expectSaved:          false,
				expectedLockObtained: false,
			}),
			Entry("when the provider refreshes the session", refreshSessionWithProviderTableInput{
				session: &sessionsapi.SessionState{
					RefreshToken: refresh,
					Lock:         &testLock{},
				},
				expectedErr:          nil,
				expectSaved:          true,
				expectedLockObtained: false,
			}),
			Entry("when the provider doesn't implement refresh", refreshSessionWithProviderTableInput{
				session: &sessionsapi.SessionState{
					RefreshToken: notImplemented,
					Lock:         &testLock{},
				},
				expectedErr:          errors.New("error refreshing tokens: error refreshing tokens"),
				expectSaved:          false,
				expectedLockObtained: false,
			}),
			Entry("when the provider returns an error", refreshSessionWithProviderTableInput{
				session: &sessionsapi.SessionState{
					RefreshToken: "RefreshError",
					CreatedAt:    &now,
					ExpiresOn:    &now,
					Lock:         &testLock{},
				},
				expectedErr:          errors.New("error refreshing tokens: error refreshing session"),
				expectSaved:          false,
				expectedLockObtained: false,
			}),
			Entry("when the saving the session returns an error", refreshSessionWithProviderTableInput{
				session: &sessionsapi.SessionState{
					RefreshToken: refresh,
					AccessToken:  "NoSave",
					Lock:         &testLock{},
				},
				expectedErr:          errors.New("error saving session: unable to save session"),
				expectSaved:          true,
				expectedLockObtained: false,
			}),
		)
	})

	Context("validateSession", func() {
		var s *storedSessionLoader

		BeforeEach(func() {
			s = &storedSessionLoader{}
		})

		Context("with a valid session", func() {
			It("does not return an error", func() {
				expires := time.Now().Add(1 * time.Minute)
				session := &sessionsapi.SessionState{
					AccessToken: "Valid",
					ExpiresOn:   &expires,
				}

				provider := NewTestProvider(&url.URL{Host: "www.example.com"}, providerEmail)
				provider.ValidToken = true
				Expect(s.validateSession(ctx, provider, session)).To(Succeed())
			})
		})

		Context("with an expired session", func() {
			It("returns an error", func() {
				created := time.Now().Add(-5 * time.Minute)
				expires := time.Now().Add(-1 * time.Minute)
				session := &sessionsapi.SessionState{
					AccessToken: "Valid",
					CreatedAt:   &created,
					ExpiresOn:   &expires,
				}

				provider := NewTestProvider(&url.URL{Host: "www.example.com"}, providerEmail)
				Expect(s.validateSession(ctx, provider, session)).To(MatchError("session is expired"))
			})
		})

		Context("with an invalid session", func() {
			It("returns an error", func() {
				expires := time.Now().Add(1 * time.Minute)
				session := &sessionsapi.SessionState{
					AccessToken: "Invalid",
					ExpiresOn:   &expires,
				}

				provider := NewTestProvider(&url.URL{Host: "www.example.com"}, providerEmail)
				provider.ValidToken = false
				Expect(s.validateSession(ctx, provider, session)).To(MatchError("session is invalid"))
			})
		})
	})
})

type fakeSessionStore struct {
	SaveFunc  func(http.ResponseWriter, *http.Request, *sessionsapi.SessionState) error
	LoadFunc  func(req *http.Request) (*sessionsapi.SessionState, error)
	ClearFunc func(rw http.ResponseWriter, req *http.Request) error
}

func (f *fakeSessionStore) Save(rw http.ResponseWriter, req *http.Request, s *sessionsapi.SessionState) error {
	if f.SaveFunc != nil {
		return f.SaveFunc(rw, req, s)
	}
	return nil
}
func (f *fakeSessionStore) Load(req *http.Request) (*sessionsapi.SessionState, error) {
	if f.LoadFunc != nil {
		return f.LoadFunc(req)
	}
	return nil, nil
}

func (f *fakeSessionStore) Clear(rw http.ResponseWriter, req *http.Request) error {
	if f.ClearFunc != nil {
		return f.ClearFunc(rw, req)
	}
	return nil
}

func (f *fakeSessionStore) VerifyConnection(_ context.Context) error {
	return nil
}
