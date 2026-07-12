package services

import (
	"crypto/md5"
	"encoding/hex"
	"math"
	"testing"
	"time"

	"animalpoke/backend/internal/config"

	"github.com/stretchr/testify/assert"
)

func TestCityCacheKey(t *testing.T) {
	key1 := cityCacheKey(39.9042, 116.4074)
	key2 := cityCacheKey(39.9050, 116.4080)
	// 同城市精度应生成相同 key
	assert.Equal(t, key1, key2)

	key3 := cityCacheKey(31.2304, 121.4737)
	assert.NotEqual(t, key1, key3)
}

func TestGeoService_GetCity_NoKey(t *testing.T) {
	cfg := &config.ThirdPartyConfig{TencentMapKey: ""}
	svc := NewGeoService(cfg)

	result, err := svc.GetCity(39.9042, 116.4074)
	assert.NoError(t, err)
	assert.Empty(t, result.City)
}

func TestGeoService_GetCity_Cached(t *testing.T) {
	cfg := &config.ThirdPartyConfig{TencentMapKey: ""}
	svc := NewGeoService(cfg)

	lat := 39.9042
	lng := 116.4074

	key := cityCacheKey(lat, lng)
	geoCache.Set(key, GeoCityResult{City: "北京市", Province: "北京市"}, time.Hour)
	defer geoCache.Delete(key)

	result, err := svc.GetCity(lat, lng)
	assert.NoError(t, err)
	assert.Equal(t, "北京市", result.City)
	assert.True(t, result.Cached)
}

func TestCacheKeyRounding(t *testing.T) {
	lat := 39.123456789
	lng := 116.987654321
	expected := math.Floor(lat*100) / 100
	assert.Equal(t, 39.12, expected)
	assert.Equal(t, "city:39.12,116.98", cityCacheKey(lat, lng))
}

func TestTencentMapSignature(t *testing.T) {
	uri := "/ws/geocoder/v1/"
	query := "key=map-key&location=39.904200%2C116.407400"
	secretKey := "secret-key"

	// 腾讯规则：MD5(uri + '?' + urldecode(query) + SK)，不对拼接串整体编码。
	basicString := uri + "?key=map-key&location=39.904200,116.407400" + secretKey
	expectedSum := md5.Sum([]byte(basicString))
	expected := hex.EncodeToString(expectedSum[:])

	assert.Equal(t, expected, tencentMapSignature(uri, query, secretKey))
}
