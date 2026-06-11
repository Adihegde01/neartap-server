package models

import "time"

// Tap represents a drinking water tap entry
type Tap struct {
	ID                  string    `json:"id" firestore:"id"`
	Name                string    `json:"name" firestore:"name"`
	Address             string    `json:"address" firestore:"address"`
	Lat                 float64   `json:"lat" firestore:"lat"`
	Lng                 float64   `json:"lng" firestore:"lng"`
	Hours               string    `json:"hours" firestore:"hours"`
	IsOpen              bool      `json:"isOpen" firestore:"isOpen"`
	IsFree              bool      `json:"isFree" firestore:"isFree"`
	PaymentMethods      []string  `json:"paymentMethods" firestore:"paymentMethods"`
	IsVerified          bool      `json:"isVerified" firestore:"isVerified"`
	WaterQuality        string    `json:"waterQuality" firestore:"waterQuality"`
	Description         string    `json:"description" firestore:"description"`
	LastReportedWorking time.Time `json:"lastReportedWorking" firestore:"lastReportedWorking"`
	AddedBy             UserRef   `json:"addedBy" firestore:"addedBy"`
	Photos              []string  `json:"photos" firestore:"photos"`
	Confirmations       int       `json:"confirmations" firestore:"confirmations"`
	ConfirmedBy         []string  `json:"confirmedBy" firestore:"confirmedBy"`
	Issues              []string  `json:"issues" firestore:"issues"`
	CreatedAt           time.Time `json:"createdAt" firestore:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt" firestore:"updatedAt"`
}

// UserRef is a lightweight reference to the user who added/confirmed
type UserRef struct {
	UID      string `json:"uid" firestore:"uid"`
	Name     string `json:"name" firestore:"name"`
	PhotoURL string `json:"photoURL" firestore:"photoURL"`
}

// TapWithDistance extends Tap with a computed distance field
type TapWithDistance struct {
	Tap
	Distance float64 `json:"distance"` // km from user
}

// CreateTapRequest is the payload for POST /api/taps
type CreateTapRequest struct {
	Name           string   `json:"name"`
	Address        string   `json:"address"`
	Lat            float64  `json:"lat"`
	Lng            float64  `json:"lng"`
	Hours          string   `json:"hours"`
	IsFree         bool     `json:"isFree"`
	PaymentMethods []string `json:"paymentMethods"`
	WaterQuality   string   `json:"waterQuality"`
	Description    string   `json:"description"`
	Photos         []string `json:"photos"`
}

// ReportIssueRequest is the payload for POST /api/taps/:id/report
type ReportIssueRequest struct {
	Issue string `json:"issue"`
}

// APIResponse is a generic JSON envelope
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Message string      `json:"message,omitempty"`
}

// NearbyQuery params for GET /api/taps/nearby
type NearbyQuery struct {
	Lat    float64 `json:"lat"`
	Lng    float64 `json:"lng"`
	RadiusKm float64 `json:"radiusKm"`
}
