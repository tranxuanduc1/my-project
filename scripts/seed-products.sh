#!/usr/bin/env bash
set -euo pipefail

IAM_URL=${IAM_URL:-http://localhost:8081}
ORDER_URL=${ORDER_URL:-http://localhost:8082}
ADMIN_EMAIL=${IAM_ADMIN_EMAIL:-admin@example.com}
ADMIN_PASSWORD=${IAM_ADMIN_PASSWORD:-admin123456}

products=(
  "APPLE-IPHONE-16-128|Apple iPhone 16 128GB|Apple smartphone with a 6.1-inch display and 128GB storage|79900|30"
  "APPLE-IPHONE-16-PRO-128|Apple iPhone 16 Pro 128GB|Apple Pro smartphone with a titanium design and 128GB storage|99900|20"
  "SAMSUNG-GALAXY-S25-128|Samsung Galaxy S25 128GB|Samsung flagship Android smartphone with 128GB storage|79900|30"
  "SAMSUNG-GALAXY-S25-ULTRA-256|Samsung Galaxy S25 Ultra 256GB|Samsung Ultra smartphone with S Pen and 256GB storage|129900|20"
  "GOOGLE-PIXEL-9-128|Google Pixel 9 128GB|Google Android smartphone with 128GB storage|79900|25"
  "ONEPLUS-13-256|OnePlus 13 256GB|OnePlus flagship Android smartphone with 256GB storage|89900|20"
  "XIAOMI-15-256|Xiaomi 15 256GB|Compact Xiaomi flagship smartphone with 256GB storage|99900|20"
  "APPLE-MACBOOK-AIR-M4-13|Apple MacBook Air 13-inch M4|Thin and light Apple laptop with M4 processor|99900|15"
  "APPLE-MACBOOK-PRO-M4-14|Apple MacBook Pro 14-inch M4|Apple professional laptop with M4 processor|159900|12"
  "DELL-XPS-13-9345|Dell XPS 13 9345|Premium 13-inch Dell ultraportable laptop|129900|12"
  "LENOVO-X1-CARBON-G12|Lenovo ThinkPad X1 Carbon Gen 12|Business ultraportable laptop with a 14-inch display|169900|10"
  "HP-SPECTRE-X360-14|HP Spectre x360 14|Premium convertible laptop with a 14-inch touchscreen|149900|10"
  "ASUS-ZENBOOK-14-OLED|ASUS Zenbook 14 OLED|Portable ASUS laptop with a 14-inch OLED display|129900|15"
  "MICROSOFT-SURFACE-LAPTOP-7|Microsoft Surface Laptop 7|Microsoft Copilot Plus laptop with a touchscreen|99900|12"
  "ACER-SWIFT-GO-14|Acer Swift Go 14|Lightweight Acer laptop with a 14-inch OLED display|89900|15"
  "APPLE-IPAD-A16-128|Apple iPad A16 128GB|Apple tablet with A16 chip and 128GB storage|34900|25"
  "APPLE-IPAD-AIR-M3-11|Apple iPad Air 11-inch M3|Portable Apple tablet with M3 processor|59900|20"
  "SAMSUNG-TAB-S10-PLUS-256|Samsung Galaxy Tab S10 Plus 256GB|Samsung Android tablet with a large AMOLED display|99900|15"
  "MICROSOFT-SURFACE-PRO-11|Microsoft Surface Pro 11|Two-in-one Windows tablet with a 13-inch touchscreen|99900|12"
  "AMAZON-KINDLE-PAPERWHITE-16|Amazon Kindle Paperwhite 16GB|Water-resistant e-reader with a glare-free display|15900|30"
  "APPLE-AIRPODS-4-ANC|Apple AirPods 4 with ANC|Apple wireless earbuds with active noise cancellation|17900|40"
  "APPLE-AIRPODS-PRO-2|Apple AirPods Pro 2|Apple in-ear headphones with active noise cancellation|24900|35"
  "SONY-WH1000XM5|Sony WH-1000XM5|Sony wireless over-ear noise-cancelling headphones|39900|25"
  "BOSE-QC-ULTRA|Bose QuietComfort Ultra Headphones|Bose premium wireless noise-cancelling headphones|42900|20"
  "SAMSUNG-BUDS3-PRO|Samsung Galaxy Buds3 Pro|Samsung wireless earbuds with active noise cancellation|24900|30"
  "JBL-FLIP-6|JBL Flip 6|Portable waterproof Bluetooth speaker|12900|35"
  "APPLE-WATCH-S10-42|Apple Watch Series 10 42mm|Apple smartwatch with fitness and health tracking|39900|25"
  "SAMSUNG-WATCH7-44|Samsung Galaxy Watch7 44mm|Samsung Android smartwatch with health tracking|32900|25"
  "GOOGLE-PIXEL-WATCH-3-45|Google Pixel Watch 3 45mm|Google smartwatch with Fitbit health features|39900|20"
  "GARMIN-FORERUNNER-265|Garmin Forerunner 265|GPS running watch with an AMOLED display|44900|20"
  "SONY-PS5-SLIM-DISC|Sony PlayStation 5 Slim Disc Edition|Sony home game console with an Ultra HD Blu-ray drive|49900|18"
  "MICROSOFT-XBOX-SERIES-X|Microsoft Xbox Series X|Microsoft 4K home game console with 1TB storage|49900|18"
  "NINTENDO-SWITCH-OLED|Nintendo Switch OLED|Nintendo hybrid game console with an OLED display|34900|25"
  "VALVE-STEAM-DECK-OLED-512|Valve Steam Deck OLED 512GB|Handheld PC gaming system with an OLED display|54900|15"
  "META-QUEST-3-512|Meta Quest 3 512GB|Standalone mixed-reality headset with 512GB storage|49900|15"
  "CANON-EOS-R6-II-BODY|Canon EOS R6 Mark II Body|Full-frame Canon mirrorless camera body|249900|8"
  "SONY-A7-IV-BODY|Sony Alpha 7 IV Body|Full-frame Sony mirrorless camera body|249900|8"
  "FUJIFILM-XT5-BODY|Fujifilm X-T5 Body|APS-C Fujifilm mirrorless camera body|169900|10"
  "DJI-OSMO-POCKET-3|DJI Osmo Pocket 3|Compact stabilized camera with a one-inch sensor|51900|15"
  "GOPRO-HERO13-BLACK|GoPro HERO13 Black|Waterproof action camera with 5.3K video recording|39900|18"
  "LG-OLED-C4-55|LG C4 OLED 55-inch TV|LG 4K OLED smart television with a 55-inch display|149900|7"
  "SAMSUNG-QN90D-55|Samsung QN90D Neo QLED 55-inch TV|Samsung 4K Neo QLED smart television|169900|7"
  "SONY-BRAVIA-8-55|Sony BRAVIA 8 OLED 55-inch TV|Sony 4K OLED Google television|199900|6"
  "SONOS-ARC-ULTRA|Sonos Arc Ultra|Premium Dolby Atmos wireless soundbar|99900|12"
  "DYSON-V15-DETECT|Dyson V15 Detect|Cordless vacuum cleaner with laser dust detection|74900|12"
  "INSTANT-POT-DUO-7IN1-6QT|Instant Pot Duo 7-in-1 6 Quart|Multi-function electric pressure cooker|9999|20"
  "PHILIPS-AIRFRYER-XXL|Philips Premium Airfryer XXL|Large-capacity hot air fryer for family meals|29900|18"
  "LOGITECH-MX-MASTER-3S|Logitech MX Master 3S|Wireless ergonomic productivity mouse|9999|40"
  "KEYCHRON-Q1-MAX|Keychron Q1 Max|Wireless custom mechanical keyboard with an aluminum body|21900|20"
  "HERMAN-MILLER-AERON-B|Herman Miller Aeron Chair Size B|Ergonomic office chair with breathable mesh support|180500|5"
)

login_response=$(curl -fsS \
  -X POST "$IAM_URL/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASSWORD\"}")

admin_token=$(printf '%s' "$login_response" | sed -n 's/.*"access_token":"\([^"]*\)".*/\1/p')
if [ -z "$admin_token" ]; then
  echo "Could not read access_token from IAM login response." >&2
  exit 1
fi

response_dir=$(mktemp -d)
trap 'rm -rf "$response_dir"' EXIT
response_file="$response_dir/product-response.json"
created=0
skipped=0

for product in "${products[@]}"; do
  IFS='|' read -r sku name description price_cents stock <<< "$product"
  http_status=$(curl -sS \
    -o "$response_file" \
    -w '%{http_code}' \
    -X POST "$ORDER_URL/api/v1/products" \
    -H "Authorization: Bearer $admin_token" \
    -H 'Content-Type: application/json' \
    -d "{\"sku\":\"$sku\",\"name\":\"$name\",\"description\":\"$description\",\"price_cents\":$price_cents,\"currency\":\"USD\",\"stock\":$stock}")

  case "$http_status" in
    201)
      created=$((created + 1))
      printf 'Created: %s\n' "$sku"
      ;;
    409)
      skipped=$((skipped + 1))
      printf 'Skipped existing SKU: %s\n' "$sku"
      ;;
    *)
      printf 'Failed to create %s (HTTP %s): ' "$sku" "$http_status" >&2
      cat "$response_file" >&2
      printf '\n' >&2
      exit 1
      ;;
  esac
done

printf 'Finished: %d created, %d already existed, %d total.\n' \
  "$created" "$skipped" "${#products[@]}"
