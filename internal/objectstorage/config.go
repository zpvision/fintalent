package objectstorage

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Driver, Endpoint, Region, Bucket, AccessKey, SecretKey string
	PathStyle                                              bool
	TTL                                                    time.Duration
	VideoMax                                               int64
}

func ConfigFrom(get func(string) string, production bool) (Config, error) {
	c := Config{Driver: get("ACADEMY_STORAGE_DRIVER"), Endpoint: get("S3_ENDPOINT"), Region: get("S3_REGION"), Bucket: get("S3_BUCKET"), AccessKey: get("S3_ACCESS_KEY"), SecretKey: get("S3_SECRET_KEY"), TTL: 5 * time.Minute, VideoMax: 2 << 30, PathStyle: true}
	if c.Driver == "" {
		if production {
			return c, errors.New("production requires ACADEMY_STORAGE_DRIVER=s3")
		}
		c.Driver = "local"
	}
	if c.Driver != "local" && c.Driver != "s3" {
		return c, errors.New("invalid ACADEMY_STORAGE_DRIVER")
	}
	if production && c.Driver != "s3" {
		return c, errors.New("production requires S3 storage")
	}
	if v := get("S3_FORCE_PATH_STYLE"); v != "" {
		b, e := strconv.ParseBool(v)
		if e != nil {
			return c, errors.New("invalid S3_FORCE_PATH_STYLE")
		}
		c.PathStyle = b
	}
	if v := get("S3_PRESIGN_TTL_SECONDS"); v != "" {
		n, e := strconv.Atoi(v)
		if e != nil || n < 60 || n > 900 {
			return c, errors.New("S3_PRESIGN_TTL_SECONDS must be 60..900")
		}
		c.TTL = time.Duration(n) * time.Second
	}
	if v := get("ACADEMY_VIDEO_MAX_MB"); v != "" {
		n, e := strconv.ParseInt(v, 10, 64)
		if e != nil || n < 1 || n > 10240 {
			return c, errors.New("ACADEMY_VIDEO_MAX_MB must be 1..10240")
		}
		c.VideoMax = n << 20
	}
	if c.Driver == "local" {
		return c, nil
	}
	u, e := url.Parse(c.Endpoint)
	if e != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.ContainsAny(u.Host, "*; \t\r\n") || strings.Trim(u.Path, "/") != "" {
		return c, errors.New("S3_ENDPOINT must be an HTTPS origin")
	}
	if c.Region == "" || c.Bucket == "" || c.AccessKey == "" || c.SecretKey == "" || strings.ContainsAny(c.Bucket, "/\\ \r\n") {
		return c, errors.New("S3 region, bucket and static keys are required")
	}
	return c, nil
}
