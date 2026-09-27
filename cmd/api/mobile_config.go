package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// Mobile release controls are public configuration, never credentials.
func mobileConfig(w http.ResponseWriter, r *http.Request) {
	platform := strings.ToUpper(r.URL.Query().Get("platform"))
	if platform != "ANDROID" && platform != "IOS" {
		http.Error(w, "unsupported platform", http.StatusBadRequest)
		return
	}
	storeURL := os.Getenv("MOBILE_" + platform + "_STORE_URL")
	parsed, err := url.Parse(storeURL)
	if err != nil || parsed.Scheme != "https" || (parsed.Host != "apps.apple.com" && parsed.Host != "play.google.com") {
		storeURL = ""
	}
	minimum := os.Getenv("MOBILE_" + platform + "_MIN_VERSION")
	// Do not strand installed users when a release has no valid store destination.
	if storeURL == "" {
		minimum = ""
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"maintenance":     os.Getenv("MOBILE_MAINTENANCE") == "true",
		"message":         os.Getenv("MOBILE_MAINTENANCE_MESSAGE"),
		"minimum_version": minimum,
		"latest_version":  os.Getenv("MOBILE_" + platform + "_LATEST_VERSION"),
		"store_url":       storeURL,
	})
}
