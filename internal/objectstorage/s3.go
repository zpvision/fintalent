package objectstorage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

type S3 struct {
	client           *s3.Client
	signer           *s3.PresignClient
	bucket, identity string
}

func NewS3(c Config) *S3 {
	client := s3.New(s3.Options{Region: c.Region, BaseEndpoint: aws.String(c.Endpoint), UsePathStyle: c.PathStyle, Credentials: credentials.NewStaticCredentialsProvider(c.AccessKey, c.SecretKey, ""), HTTPClient: &http.Client{Timeout: 5 * time.Minute}, RetryMaxAttempts: 3, RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired, ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired})
	hash := sha256.Sum256([]byte(c.Endpoint + "\n" + c.Region + "\n" + c.Bucket))
	return &S3{client, s3.NewPresignClient(client), c.Bucket, hex.EncodeToString(hash[:])}
}

// SDK errors contain request URLs. Never expose or log them outside this package.
func storageError(e error) error {
	if e == nil {
		return nil
	}
	var a smithy.APIError
	if errors.As(e, &a) && (a.ErrorCode() == "NoSuchKey" || a.ErrorCode() == "NotFound" || a.ErrorCode() == "NoSuchUpload") {
		return ErrNotFound
	}
	return ErrUnavailable
}
func (s *S3) Identity() string { return s.identity }
func (s *S3) Check(ctx context.Context) error {
	_, e := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: &s.bucket})
	return storageError(e)
}
func (s *S3) Put(ctx context.Context, key string, r io.Reader, size int64, typ string) error {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	if !ValidKey(key) {
		return ErrInvalid
	}
	_, e := s.client.PutObject(ctx, &s3.PutObjectInput{Bucket: &s.bucket, Key: &key, Body: r, ContentLength: &size, ContentType: &typ})
	return storageError(e)
}
func (s *S3) Open(ctx context.Context, key, byteRange string) (io.ReadCloser, Info, error) {
	if !ValidKey(key) {
		return nil, Info{}, ErrInvalid
	}
	in := &s3.GetObjectInput{Bucket: &s.bucket, Key: &key}
	if byteRange != "" {
		in.Range = &byteRange
	}
	o, e := s.client.GetObject(ctx, in)
	if e != nil {
		return nil, Info{}, storageError(e)
	}
	return o.Body, Info{aws.ToInt64(o.ContentLength), aws.ToString(o.ContentType), aws.ToString(o.ETag)}, nil
}
func (s *S3) Stat(ctx context.Context, key string) (Info, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if !ValidKey(key) {
		return Info{}, ErrInvalid
	}
	o, e := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: &s.bucket, Key: &key})
	if e != nil {
		return Info{}, storageError(e)
	}
	return Info{aws.ToInt64(o.ContentLength), aws.ToString(o.ContentType), aws.ToString(o.ETag)}, nil
}
func (s *S3) Delete(ctx context.Context, key string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if !ValidKey(key) {
		return ErrInvalid
	}
	_, e := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &s.bucket, Key: &key})
	return storageError(e)
}
func (s *S3) Begin(ctx context.Context, key, typ string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if !ValidKey(key) {
		return "", ErrInvalid
	}
	o, e := s.client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{Bucket: &s.bucket, Key: &key, ContentType: &typ, CacheControl: aws.String("private, no-store")})
	if e != nil {
		return "", storageError(e)
	}
	return aws.ToString(o.UploadId), nil
}
func (s *S3) SignPart(ctx context.Context, key, id string, number int, size int64, ttl time.Duration) (Signed, error) {
	if !ValidKey(key) || number < 1 || number > 10000 || size < 1 || ttl <= 0 || ttl > 15*time.Minute {
		return Signed{}, ErrInvalid
	}
	o, e := s.signer.PresignUploadPart(ctx, &s3.UploadPartInput{Bucket: &s.bucket, Key: &key, UploadId: &id, PartNumber: aws.Int32(int32(number)), ContentLength: &size}, func(o *s3.PresignOptions) { o.Expires = ttl })
	if e != nil {
		return Signed{}, storageError(e)
	}
	headers := map[string]string{}
	for k, v := range o.SignedHeader {
		if k != "Host" && k != "Content-Length" && len(v) > 0 {
			headers[k] = v[0]
		}
	}
	return Signed{o.URL, headers, time.Now().Add(ttl)}, nil
}
func (s *S3) Parts(ctx context.Context, key, id string) ([]Part, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if !ValidKey(key) {
		return nil, ErrInvalid
	}
	out := []Part{}
	pager := s3.NewListPartsPaginator(s.client, &s3.ListPartsInput{Bucket: &s.bucket, Key: &key, UploadId: &id})
	for pager.HasMorePages() {
		o, e := pager.NextPage(ctx)
		if e != nil {
			return nil, storageError(e)
		}
		for _, p := range o.Parts {
			out = append(out, Part{int(aws.ToInt32(p.PartNumber)), aws.ToInt64(p.Size), aws.ToString(p.ETag)})
		}
		if len(out) > 10000 {
			return nil, ErrInvalid
		}
	}
	return out, nil
}
func (s *S3) Complete(ctx context.Context, key, id string, parts []Part) error {
	if !ValidKey(key) {
		return ErrInvalid
	}
	p := make([]types.CompletedPart, 0, len(parts))
	for _, v := range parts {
		p = append(p, types.CompletedPart{PartNumber: aws.Int32(int32(v.Number)), ETag: aws.String(v.ETag)})
	}
	_, e := s.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{Bucket: &s.bucket, Key: &key, UploadId: &id, MultipartUpload: &types.CompletedMultipartUpload{Parts: p}})
	return storageError(e)
}
func (s *S3) Abort(ctx context.Context, key, id string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if !ValidKey(key) {
		return ErrInvalid
	}
	_, e := s.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{Bucket: &s.bucket, Key: &key, UploadId: &id})
	if errors.Is(storageError(e), ErrNotFound) {
		return nil
	}
	return storageError(e)
}
func (s *S3) SignGet(ctx context.Context, key, typ, disposition string, ttl time.Duration) (Signed, error) {
	if !ValidKey(key) || ttl <= 0 || ttl > 15*time.Minute {
		return Signed{}, ErrInvalid
	}
	o, e := s.signer.PresignGetObject(ctx, &s3.GetObjectInput{Bucket: &s.bucket, Key: &key, ResponseContentType: &typ, ResponseContentDisposition: &disposition, ResponseCacheControl: aws.String("private, no-store")}, func(o *s3.PresignOptions) { o.Expires = ttl })
	if e != nil {
		return Signed{}, storageError(e)
	}
	return Signed{URL: o.URL, ExpiresAt: time.Now().Add(ttl)}, nil
}
func (s *S3) String() string { return fmt.Sprintf("S3(%s)", s.identity[:12]) }
