package main

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestMobileConfigRequiresStoreLinkForMandatoryUpdate(t *testing.T) {
	t.Setenv("MOBILE_ANDROID_MIN_VERSION", "1.2.0")
	t.Setenv("MOBILE_ANDROID_STORE_URL", "https://untrusted.example/app")
	w := httptest.NewRecorder()
	mobileConfig(w, httptest.NewRequest("GET", "/?platform=android", nil))
	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["minimum_version"] != "" || result["store_url"] != "" {
		t.Fatal("must not force an update without an official store URL")
	}
	t.Setenv("MOBILE_ANDROID_STORE_URL", "https://play.google.com/store/apps/details?id=printa")
	w = httptest.NewRecorder()
	mobileConfig(w, httptest.NewRequest("GET", "/?platform=android", nil))
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["minimum_version"] != "1.2.0" {
		t.Fatal("minimum version missing")
	}
}
