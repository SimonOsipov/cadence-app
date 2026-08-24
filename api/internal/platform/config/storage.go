package config

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
)

// StorageConfig is everything reaching the object store needs.
//
// Nothing here has a fallback. The store holds medical photographs behind
// private buckets, and every plausible default is worse than not starting: an
// unset endpoint would address AWS, an unset bucket would address a name this
// deployment does not own, and either failure surfaces as a patient's photo
// that will not open rather than as a process that refused to boot.
type StorageConfig struct {
	Endpoint        string
	Region          string
	AccessKeyID     string
	SecretAccessKey string

	// PathStyle addresses a bucket as {endpoint}/{bucket}/{key} rather than
	// {bucket}.{endpoint}/{key}. Required rather than assumed: the two produce
	// different URLs, so a guess is green against a local MinIO and a 404 in
	// the deployment.
	PathStyle bool

	VialsBucket      string
	InjectionsBucket string
}

func loadStorage() (*StorageConfig, error) {
	endpoint := getEnv("STORAGE_ENDPOINT", "")
	if endpoint == "" {
		return nil, errors.New("STORAGE_ENDPOINT is required")
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("STORAGE_ENDPOINT is not a URL: %w", err)
	}
	if parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("STORAGE_ENDPOINT must be an absolute http(s) address, got %q", endpoint)
	}

	region := getEnv("STORAGE_REGION", "")
	if region == "" {
		return nil, errors.New("STORAGE_REGION is required")
	}

	accessKeyID := getEnv("STORAGE_ACCESS_KEY_ID", "")
	if accessKeyID == "" {
		return nil, errors.New("STORAGE_ACCESS_KEY_ID is required")
	}

	secretAccessKey := getEnv("STORAGE_SECRET_ACCESS_KEY", "")
	if secretAccessKey == "" {
		return nil, errors.New("STORAGE_SECRET_ACCESS_KEY is required")
	}

	rawPathStyle := getEnv("STORAGE_PATH_STYLE", "")
	if rawPathStyle == "" {
		return nil, errors.New("STORAGE_PATH_STYLE is required")
	}
	pathStyle, err := strconv.ParseBool(rawPathStyle)
	if err != nil {
		return nil, fmt.Errorf("parsing STORAGE_PATH_STYLE: %w", err)
	}

	vials := getEnv("STORAGE_BUCKET_VIALS", "")
	if vials == "" {
		return nil, errors.New("STORAGE_BUCKET_VIALS is required")
	}

	injections := getEnv("STORAGE_BUCKET_INJECTIONS", "")
	if injections == "" {
		return nil, errors.New("STORAGE_BUCKET_INJECTIONS is required")
	}

	// Server-generated keys start with the patient's id and nothing else, so two
	// buckets under one name let an injection photo and a vial label collide on
	// a key and overwrite each other silently.
	if vials == injections {
		return nil, errors.New(
			"STORAGE_BUCKET_VIALS and STORAGE_BUCKET_INJECTIONS name the same bucket, " +
				"so a vial label and an injection photo share a key namespace",
		)
	}

	return &StorageConfig{
		Endpoint:         endpoint,
		Region:           region,
		AccessKeyID:      accessKeyID,
		SecretAccessKey:  secretAccessKey,
		PathStyle:        pathStyle,
		VialsBucket:      vials,
		InjectionsBucket: injections,
	}, nil
}

// storageKeys is what clearEnv and the deployment both have to know about, kept
// beside the loader that reads them rather than copied into a test.
func storageKeys() []string {
	return []string{
		"STORAGE_ENDPOINT", "STORAGE_REGION",
		"STORAGE_ACCESS_KEY_ID", "STORAGE_SECRET_ACCESS_KEY",
		"STORAGE_PATH_STYLE",
		"STORAGE_BUCKET_VIALS", "STORAGE_BUCKET_INJECTIONS",
	}
}
