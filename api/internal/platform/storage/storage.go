// Package storage signs short-lived links to objects in the S3-compatible store.
//
// It is a driver and nothing else: it knows buckets, keys and expiry, and knows
// nothing about patients, vials or dose events. Who may see an object is decided
// by the bounded context that owns the row naming it, before it asks for a link
// — the store has no row-level security of its own, so a link is authority and
// the check cannot be moved here.
package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Link is a signed URL and the moment it stops working.
type Link struct {
	URL       string
	ExpiresAt time.Time
}

// Config is what addressing the store needs. Every field is required; the
// loader in platform/config is where that is enforced.
type Config struct {
	Endpoint        string
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	PathStyle       bool
}

// Signer issues signed links against one store.
type Signer struct {
	presign *s3.PresignClient
	now     func() time.Time
}

// New builds a Signer. The clock is a parameter rather than time.Now so that a
// link's stated expiry can be measured against the moment it was signed.
func New(cfg Config, now func() time.Time) (*Signer, error) {
	if now == nil {
		return nil, errors.New("storage: a clock is required")
	}

	client := s3.New(s3.Options{
		Region:       cfg.Region,
		BaseEndpoint: aws.String(cfg.Endpoint),
		UsePathStyle: cfg.PathStyle,
		Credentials: credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID, cfg.SecretAccessKey, "",
		),
	})

	return &Signer{presign: s3.NewPresignClient(client), now: now}, nil
}

// SignedGet answers a link that reads one object and expires after ttl.
//
// The response's content type and its disposition are part of the signature —
// they travel as query parameters, which SigV4 covers — so the store serves the
// object as the caller says regardless of what was stored under the key. That is
// where the type is pinned, and it has to be: what a client declared on the way
// in is not signed (see SignedPut), so an object holding text/html would
// otherwise be handed back as text/html from a host the API vouches for.
func (s *Signer) SignedGet(
	ctx context.Context, bucket, key, contentType string, ttl time.Duration,
) (Link, error) {
	if err := s.usable(bucket, key, ttl); err != nil {
		return Link{}, err
	}
	if contentType == "" {
		return Link{}, errors.New("storage: a content type is required")
	}

	signedAt := s.now()
	request, err := s.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket:                     aws.String(bucket),
		Key:                        aws.String(key),
		ResponseContentType:        aws.String(contentType),
		ResponseContentDisposition: aws.String("attachment"),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return Link{}, fmt.Errorf("signing a read of %s: %w", key, err)
	}

	return Link{URL: request.URL, ExpiresAt: signedAt.Add(ttl)}, nil
}

// SignedPut answers a link that writes one object under exactly one key, and
// expires after ttl.
//
// It does not bind what is written. A presigned SigV4 URL covers only the
// headers named in X-Amz-SignedHeaders, and this SDK names one: measured on
// v1.107.3, PresignPutObject with ContentType set produces
// X-Amz-SignedHeaders=host, and MinIO accepts the same link with text/html. The
// key is the whole of what this constrains — which is enough, because the caller
// chooses it and the read side pins the type it is served as.
func (s *Signer) SignedPut(ctx context.Context, bucket, key string, ttl time.Duration) (Link, error) {
	if err := s.usable(bucket, key, ttl); err != nil {
		return Link{}, err
	}

	signedAt := s.now()
	request, err := s.presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return Link{}, fmt.Errorf("signing a write of %s: %w", key, err)
	}

	return Link{URL: request.URL, ExpiresAt: signedAt.Add(ttl)}, nil
}

// usable refuses what the SDK would otherwise sign into a link that addresses
// the bucket's root or never expires.
func (s *Signer) usable(bucket, key string, ttl time.Duration) error {
	if bucket == "" {
		return errors.New("storage: a bucket is required")
	}
	if key == "" {
		return errors.New("storage: a key is required")
	}
	if ttl <= 0 {
		return fmt.Errorf("storage: a link's lifetime must be positive, got %s", ttl)
	}

	return nil
}
