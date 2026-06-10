package store

import (
	"context"
	"fmt"
	"log"
	"math"
	"sort"
	"time"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
	"github.com/neartap/server/internal/models"
)

const tapsCollection = "taps"

// Store wraps the Firestore client with tap-specific operations
type Store struct {
	client    *firestore.Client
	projectID string
}

// New initialises the Firestore client using the provided Firebase App.
// If app is nil, it falls back to the in-memory mock store.
func New(ctx context.Context, app *firebase.App, projectID string) (*Store, error) {
	if app == nil {
		log.Printf("[store] Running in DEMO mode (in-memory)")
		return &Store{client: nil, projectID: projectID}, nil
	}

	client, err := app.Firestore(ctx)
	if err != nil {
		return nil, fmt.Errorf("app.Firestore: %w", err)
	}
	return &Store{client: client, projectID: projectID}, nil
}

// Close releases the Firestore connection
func (s *Store) Close() error {
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}

// ── CRUD ──────────────────────────────────────────────────────────────────────

// GetAll returns every tap (Firestore) or mock data
func (s *Store) GetAll(ctx context.Context) ([]models.Tap, error) {
	if s.client == nil {
		return mockTaps, nil
	}
	docs, err := s.client.Collection(tapsCollection).Documents(ctx).GetAll()
	if err != nil {
		return nil, err
	}
	var taps []models.Tap
	for _, d := range docs {
		var t models.Tap
		if err := d.DataTo(&t); err != nil {
			continue
		}
		t.ID = d.Ref.ID
		taps = append(taps, t)
	}
	return taps, nil
}

// GetByID fetches a single tap
func (s *Store) GetByID(ctx context.Context, id string) (*models.Tap, error) {
	if s.client == nil {
		for _, t := range mockTaps {
			if t.ID == id {
				cp := t
				return &cp, nil
			}
		}
		return nil, fmt.Errorf("tap %s not found", id)
	}
	doc, err := s.client.Collection(tapsCollection).Doc(id).Get(ctx)
	if err != nil {
		return nil, err
	}
	var t models.Tap
	if err := doc.DataTo(&t); err != nil {
		return nil, err
	}
	t.ID = doc.Ref.ID
	return &t, nil
}

// Create adds a new tap and returns it with the generated ID
func (s *Store) Create(ctx context.Context, req models.CreateTapRequest, addedBy models.UserRef) (*models.Tap, error) {
	now := time.Now()
	tap := models.Tap{
		Name:                req.Name,
		Address:             req.Address,
		Lat:                 req.Lat,
		Lng:                 req.Lng,
		Hours:               req.Hours,
		IsOpen:              isOpenNow(req.Hours),
		IsFree:              req.IsFree,
		PaymentMethods:      req.PaymentMethods,
		IsAccessible:        req.IsAccessible,
		IsVerified:          false,
		WaterQuality:        req.WaterQuality,
		Description:         req.Description,
		Photos:              req.Photos,
		AddedBy:             addedBy,
		Confirmations:       0,
		Issues:              []string{},
		LastReportedWorking: now,
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	if s.client == nil {
		tap.ID = fmt.Sprintf("mock-%d", time.Now().UnixMilli())
		mockTaps = append([]models.Tap{tap}, mockTaps...)
		return &tap, nil
	}

	ref, _, err := s.client.Collection(tapsCollection).Add(ctx, tap)
	if err != nil {
		return nil, err
	}
	tap.ID = ref.ID
	_, _ = ref.Update(ctx, []firestore.Update{{Path: "id", Value: ref.ID}})
	return &tap, nil
}

// Confirm increments the confirmation count and auto-verifies at ≥ 3
func (s *Store) Confirm(ctx context.Context, id string) (*models.Tap, error) {
	if s.client == nil {
		for i, t := range mockTaps {
			if t.ID == id {
				mockTaps[i].Confirmations++
				if mockTaps[i].Confirmations >= 3 {
					mockTaps[i].IsVerified = true
				}
				mockTaps[i].LastReportedWorking = time.Now()
				mockTaps[i].UpdatedAt = time.Now()
				cp := mockTaps[i]
				return &cp, nil
			}
		}
		return nil, fmt.Errorf("tap %s not found", id)
	}

	ref := s.client.Collection(tapsCollection).Doc(id)
	err := s.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		doc, err := tx.Get(ref)
		if err != nil {
			return err
		}
		var t models.Tap
		if err := doc.DataTo(&t); err != nil {
			return err
		}
		t.Confirmations++
		updates := []firestore.Update{
			{Path: "confirmations", Value: t.Confirmations},
			{Path: "lastReportedWorking", Value: time.Now()},
			{Path: "updatedAt", Value: time.Now()},
		}
		if t.Confirmations >= 3 {
			updates = append(updates, firestore.Update{Path: "isVerified", Value: true})
		}
		return tx.Update(ref, updates)
	})
	if err != nil {
		return nil, err
	}
	return s.GetByID(ctx, id)
}

// ReportIssue appends an issue string to the tap's issues array
func (s *Store) ReportIssue(ctx context.Context, id, issue string) (*models.Tap, error) {
	if s.client == nil {
		for i, t := range mockTaps {
			if t.ID == id {
				mockTaps[i].Issues = append(mockTaps[i].Issues, issue)
				mockTaps[i].UpdatedAt = time.Now()
				cp := mockTaps[i]
				return &cp, nil
			}
		}
		return nil, fmt.Errorf("tap %s not found", id)
	}

	ref := s.client.Collection(tapsCollection).Doc(id)
	_, err := ref.Update(ctx, []firestore.Update{
		{Path: "issues", Value: firestore.ArrayUnion(issue)},
		{Path: "updatedAt", Value: time.Now()},
	})
	if err != nil {
		return nil, err
	}
	return s.GetByID(ctx, id)
}

// Update updates tap details
func (s *Store) Update(ctx context.Context, id string, req models.CreateTapRequest) (*models.Tap, error) {
	if s.client == nil {
		for i, t := range mockTaps {
			if t.ID == id {
				mockTaps[i].Name = req.Name
				mockTaps[i].Address = req.Address
				mockTaps[i].Lat = req.Lat
				mockTaps[i].Lng = req.Lng
				mockTaps[i].Hours = req.Hours
				mockTaps[i].IsOpen = isOpenNow(req.Hours)
				mockTaps[i].IsFree = req.IsFree
				mockTaps[i].PaymentMethods = req.PaymentMethods
				mockTaps[i].IsAccessible = req.IsAccessible
				mockTaps[i].WaterQuality = req.WaterQuality
				mockTaps[i].Description = req.Description
				mockTaps[i].Photos = req.Photos
				mockTaps[i].UpdatedAt = time.Now()
				cp := mockTaps[i]
				return &cp, nil
			}
		}
		return nil, fmt.Errorf("tap %s not found", id)
	}

	ref := s.client.Collection(tapsCollection).Doc(id)
	updates := []firestore.Update{
		{Path: "name", Value: req.Name},
		{Path: "address", Value: req.Address},
		{Path: "lat", Value: req.Lat},
		{Path: "lng", Value: req.Lng},
		{Path: "hours", Value: req.Hours},
		{Path: "isOpen", Value: isOpenNow(req.Hours)},
		{Path: "isFree", Value: req.IsFree},
		{Path: "paymentMethods", Value: req.PaymentMethods},
		{Path: "isAccessible", Value: req.IsAccessible},
		{Path: "waterQuality", Value: req.WaterQuality},
		{Path: "description", Value: req.Description},
		{Path: "photos", Value: req.Photos},
		{Path: "updatedAt", Value: time.Now()},
	}
	_, err := ref.Update(ctx, updates)
	if err != nil {
		return nil, err
	}
	return s.GetByID(ctx, id)
}

// Delete removes a tap from database
func (s *Store) Delete(ctx context.Context, id string) error {
	if s.client == nil {
		for i, t := range mockTaps {
			if t.ID == id {
				mockTaps = append(mockTaps[:i], mockTaps[i+1:]...)
				return nil
			}
		}
		return fmt.Errorf("tap %s not found", id)
	}

	_, err := s.client.Collection(tapsCollection).Doc(id).Delete(ctx)
	return err
}

// ResolveIssues clears all issues reported on a tap
func (s *Store) ResolveIssues(ctx context.Context, id string) (*models.Tap, error) {
	if s.client == nil {
		for i, t := range mockTaps {
			if t.ID == id {
				mockTaps[i].Issues = []string{}
				mockTaps[i].UpdatedAt = time.Now()
				cp := mockTaps[i]
				return &cp, nil
			}
		}
		return nil, fmt.Errorf("tap %s not found", id)
	}

	ref := s.client.Collection(tapsCollection).Doc(id)
	_, err := ref.Update(ctx, []firestore.Update{
		{Path: "issues", Value: []string{}},
		{Path: "updatedAt", Value: time.Now()},
	})
	if err != nil {
		return nil, err
	}
	return s.GetByID(ctx, id)
}

// GetNearby returns taps within radiusKm of (lat, lng), sorted by distance
func (s *Store) GetNearby(ctx context.Context, lat, lng, radiusKm float64) ([]models.TapWithDistance, error) {
	all, err := s.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	var result []models.TapWithDistance
	for _, t := range all {
		d := haversine(lat, lng, t.Lat, t.Lng)
		if d <= radiusKm {
			result = append(result, models.TapWithDistance{Tap: t, Distance: d})
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Distance < result[j].Distance
	})
	return result, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// haversine calculates the great-circle distance between two lat/lng points (km)
func haversine(lat1, lng1, lat2, lng2 float64) float64 {
	const R = 6371.0
	dLat := (lat2 - lat1) * math.Pi / 180
	dLng := (lng2 - lng1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLng/2)*math.Sin(dLng/2)
	return R * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

// isOpenNow checks whether the tap is open given an hours string
func isOpenNow(hours string) bool {
	if hours == "24/7" || hours == "" {
		return true
	}
	return true // simplified — full parsing omitted for brevity
}
