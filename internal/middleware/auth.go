package middleware

import (
	"context"
	"net/http"
	"strings"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
)

type contextKey string

const UserContextKey contextKey = "firebaseUser"

// FirebaseAuth middleware validates Firebase ID tokens sent in the
// Authorization: Bearer <token> header.
// If no credentials are configured, it runs in demo mode and injects
// a fake user for authenticated routes.
type FirebaseAuth struct {
	client *auth.Client
	demo   bool
}

// NewFirebaseAuth creates the middleware. Pass nil client for demo mode.
func NewFirebaseAuth(app *firebase.App) *FirebaseAuth {
	if app == nil {
		return &FirebaseAuth{demo: true}
	}
	client, err := app.Auth(context.Background())
	if err != nil {
		return &FirebaseAuth{demo: true}
	}
	return &FirebaseAuth{client: client}
}

// Authenticate is a middleware that requires a valid token.
func (fa *FirebaseAuth) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fa.demo {
			// Demo mode: inject a fake user
			ctx := context.WithValue(r.Context(), UserContextKey, &auth.Token{
				UID: "demo-user",
				Claims: map[string]interface{}{
					"name":    "Demo User",
					"email":   "demo@neartap.app",
				},
			})
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, `{"success":false,"error":"missing auth token"}`, http.StatusUnauthorized)
			return
		}

		idToken := strings.TrimPrefix(authHeader, "Bearer ")
		token, err := fa.client.VerifyIDToken(r.Context(), idToken)
		if err != nil {
			http.Error(w, `{"success":false,"error":"invalid auth token"}`, http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), UserContextKey, token)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// OptionalAuth injects the user if a valid token is present; passes through otherwise.
func (fa *FirebaseAuth) OptionalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fa.demo {
			next.ServeHTTP(w, r)
			return
		}
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
			idToken := strings.TrimPrefix(authHeader, "Bearer ")
			if token, err := fa.client.VerifyIDToken(r.Context(), idToken); err == nil {
				ctx := context.WithValue(r.Context(), UserContextKey, token)
				r = r.WithContext(ctx)
			}
		}
		next.ServeHTTP(w, r)
	})
}

// UserFromContext retrieves the Firebase token from context
func UserFromContext(ctx context.Context) *auth.Token {
	token, _ := ctx.Value(UserContextKey).(*auth.Token)
	return token
}
