#!/bin/bash
# Phase 2 Additional Endpoint Tests
# Run from project root: bash scripts/test_phase2.sh

BASE="http://localhost:8080"
PRODUCT_ID="bf4b3047-990b-42aa-87fe-472e296e3acf"
STORE_ID="01849460-c2e0-481d-9e7c-1522cb63f6dc"
STORE_PRODUCT_ID="650ab2ee-7163-483d-ac7c-413cd16f83ca"
USER_ID_1="fd9d428d-c389-4e46-a4db-9e9e9db74c2c"   # test@printa.io
USER_ID_2="5b712de2-0a88-423f-99ae-267c0590c021"   # george2@printa.io

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

pass() { echo -e "${GREEN}[PASS]${NC} $1"; }
fail() { echo -e "${RED}[FAIL]${NC} $1"; }
section() { echo -e "\n${YELLOW}══════════════════════════════════════${NC}"; echo -e "${YELLOW}  $1${NC}"; echo -e "${YELLOW}══════════════════════════════════════${NC}"; }

check() {
  local label=$1
  local expected_status=$2
  local response=$3
  local http_status=$4

  if [ "$http_status" = "$expected_status" ]; then
    pass "$label (HTTP $http_status)"
    echo "$response" | python3 -m json.tool 2>/dev/null || echo "$response"
  else
    fail "$label — Expected HTTP $expected_status, got HTTP $http_status"
    echo "$response"
  fi
  echo ""
}

# ── CATALOG TESTS ──────────────────────────────────────────────────────────────

section "CATALOG: Update Platform Product"
RESP=$(curl -s -w "\n%{http_code}" -X PUT "$BASE/api/v1/catalog/products/$PRODUCT_ID" \
  -H "Content-Type: application/json" \
  -d '{"name":"A4 Flyer Print (Premium)","description":"Full colour A4 flyer on 150gsm gloss paper","category":"FLYERS","base_price":18.50,"currency":"ZMW","sku":"FLY-A4-001","image_url":"https://cdn.printa.io/products/fly-a4-001.jpg"}')
HTTP=$(echo "$RESP" | tail -1)
BODY=$(echo "$RESP" | head -n -1)
check "PUT /api/v1/catalog/products/{id}" "200" "$BODY" "$HTTP"

section "CATALOG: Add Second Product (Business Cards)"
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE/api/v1/catalog/products" \
  -H "Content-Type: application/json" \
  -d '{"name":"Business Cards (Standard)","description":"Double-sided business cards, 350gsm","category":"BUSINESS_CARDS","base_price":45.00,"currency":"ZMW","sku":"BC-STD-001"}')
HTTP=$(echo "$RESP" | tail -1)
BODY=$(echo "$RESP" | head -n -1)
check "POST /api/v1/catalog/products (Business Cards)" "201" "$BODY" "$HTTP"
BC_PRODUCT_ID=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])" 2>/dev/null)

section "CATALOG: Filter Products by Category"
RESP=$(curl -s -w "\n%{http_code}" "$BASE/api/v1/catalog/products?category=FLYERS")
HTTP=$(echo "$RESP" | tail -1)
BODY=$(echo "$RESP" | head -n -1)
check "GET /api/v1/catalog/products?category=FLYERS" "200" "$BODY" "$HTTP"

section "CATALOG: List All Products"
RESP=$(curl -s -w "\n%{http_code}" "$BASE/api/v1/catalog/products")
HTTP=$(echo "$RESP" | tail -1)
BODY=$(echo "$RESP" | head -n -1)
check "GET /api/v1/catalog/products (all)" "200" "$BODY" "$HTTP"

# ── STORE STAFF TESTS ──────────────────────────────────────────────────────────

section "STAFF: Add MANAGER to Store"
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE/api/v1/inventory/stores/$STORE_ID/staff" \
  -H "Content-Type: application/json" \
  -d "{\"user_id\":\"$USER_ID_1\",\"role\":\"MANAGER\"}")
HTTP=$(echo "$RESP" | tail -1)
BODY=$(echo "$RESP" | head -n -1)
check "POST /api/v1/inventory/stores/{id}/staff (MANAGER)" "201" "$BODY" "$HTTP"

section "STAFF: Add CASHIER to Store"
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE/api/v1/inventory/stores/$STORE_ID/staff" \
  -H "Content-Type: application/json" \
  -d "{\"user_id\":\"$USER_ID_2\",\"role\":\"CASHIER\"}")
HTTP=$(echo "$RESP" | tail -1)
BODY=$(echo "$RESP" | head -n -1)
check "POST /api/v1/inventory/stores/{id}/staff (CASHIER)" "201" "$BODY" "$HTTP"

section "STAFF: List All Staff for Store"
RESP=$(curl -s -w "\n%{http_code}" "$BASE/api/v1/inventory/stores/$STORE_ID/staff")
HTTP=$(echo "$RESP" | tail -1)
BODY=$(echo "$RESP" | head -n -1)
check "GET /api/v1/inventory/stores/{id}/staff" "200" "$BODY" "$HTTP"

section "STAFF: Remove CASHIER from Store"
RESP=$(curl -s -w "\n%{http_code}" -X DELETE "$BASE/api/v1/inventory/stores/$STORE_ID/staff/$USER_ID_2")
HTTP=$(echo "$RESP" | tail -1)
BODY=$(echo "$RESP" | head -n -1)
if [ "$HTTP" = "204" ]; then
  pass "DELETE /api/v1/inventory/stores/{id}/staff/{user_id} (HTTP 204)"
else
  fail "DELETE staff — Expected HTTP 204, got HTTP $HTTP — $BODY"
fi

section "STAFF: Verify Staff After Removal (should show 1 MANAGER)"
RESP=$(curl -s -w "\n%{http_code}" "$BASE/api/v1/inventory/stores/$STORE_ID/staff")
HTTP=$(echo "$RESP" | tail -1)
BODY=$(echo "$RESP" | head -n -1)
check "GET /api/v1/inventory/stores/{id}/staff (post-removal)" "200" "$BODY" "$HTTP"

# ── INVENTORY PRODUCT TESTS ────────────────────────────────────────────────────

section "INVENTORY: Update Stock Quantity"
RESP=$(curl -s -w "\n%{http_code}" -X PATCH "$BASE/api/v1/inventory/products/$STORE_PRODUCT_ID/stock" \
  -H "Content-Type: application/json" \
  -d '{"quantity":250}')
HTTP=$(echo "$RESP" | tail -1)
BODY=$(echo "$RESP" | head -n -1)
check "PATCH /api/v1/inventory/products/{id}/stock" "200" "$BODY" "$HTTP"

section "INVENTORY: Set Product Unavailable"
RESP=$(curl -s -w "\n%{http_code}" -X PATCH "$BASE/api/v1/inventory/products/$STORE_PRODUCT_ID/availability" \
  -H "Content-Type: application/json" \
  -d '{"available":false}')
HTTP=$(echo "$RESP" | tail -1)
BODY=$(echo "$RESP" | head -n -1)
check "PATCH /api/v1/inventory/products/{id}/availability (false)" "200" "$BODY" "$HTTP"

section "INVENTORY: Re-enable Product Availability"
RESP=$(curl -s -w "\n%{http_code}" -X PATCH "$BASE/api/v1/inventory/products/$STORE_PRODUCT_ID/availability" \
  -H "Content-Type: application/json" \
  -d '{"available":true}')
HTTP=$(echo "$RESP" | tail -1)
BODY=$(echo "$RESP" | head -n -1)
check "PATCH /api/v1/inventory/products/{id}/availability (true)" "200" "$BODY" "$HTTP"

section "INVENTORY: Add Business Cards to Store"
if [ -n "$BC_PRODUCT_ID" ]; then
  RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE/api/v1/inventory/stores/$STORE_ID/products" \
    -H "Content-Type: application/json" \
    -d "{\"platform_product_id\":\"$BC_PRODUCT_ID\",\"vendor_price\":55.00,\"currency\":\"ZMW\",\"stock_quantity\":1000}")
  HTTP=$(echo "$RESP" | tail -1)
  BODY=$(echo "$RESP" | head -n -1)
  check "POST /api/v1/inventory/stores/{id}/products (Business Cards)" "201" "$BODY" "$HTTP"
else
  echo "Skipped — Business Cards product ID not captured"
fi

section "INVENTORY: List All Store Products (Final State)"
RESP=$(curl -s -w "\n%{http_code}" "$BASE/api/v1/inventory/stores/$STORE_ID/products")
HTTP=$(echo "$RESP" | tail -1)
BODY=$(echo "$RESP" | head -n -1)
check "GET /api/v1/inventory/stores/{id}/products (final)" "200" "$BODY" "$HTTP"

echo -e "\n${YELLOW}══════════════════════════════════════${NC}"
echo -e "${YELLOW}  ALL TESTS COMPLETE${NC}"
echo -e "${YELLOW}══════════════════════════════════════${NC}\n"
