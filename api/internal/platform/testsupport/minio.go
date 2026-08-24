package testsupport

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// MinIOImage is pinned by digest for the same reason GoTrueImage is: the suite
// asserts how an S3-compatible store answers an unsigned request and an expired
// signature, and an upgrade that changed either would carry the assertion away
// silently rather than failing.
const MinIOImage = "minio/minio@sha256:14cea493d9a34af32f524e538b8346cf79f3321eff8e708c1e2960462bd8936e"

const (
	minioPort      = "9000/tcp"
	MinIOAccessKey = "cadence-test-access"
	MinIOSecretKey = "cadence-test-secret"
	MinIORegion    = "ru-1"
)

// ObjectStore is a running MinIO and how to address it.
type ObjectStore struct {
	Endpoint string
}

// StartObjectStore runs MinIO for one test, creates the named buckets, and
// reaps the container afterwards.
//
// Path-style addressing is what the returned endpoint expects: the buckets live
// under a host with no wildcard DNS, so {bucket}.{host} resolves to nothing.
//
// The buckets are made as directories rather than through the S3 API, because
// the API would need the SDK here — and TestOnlyThisPackageImportsTheSDK exists
// to keep it reachable from exactly one package. MinIO's filesystem backend
// treats each top-level directory under its data root as a bucket.
func StartObjectStore(t *testing.T, buckets ...string) *ObjectStore {
	t.Helper()

	if len(buckets) == 0 {
		t.Fatal("StartObjectStore: name at least one bucket, or nothing can be signed")
	}
	made := make([]string, 0, len(buckets))
	for _, bucket := range buckets {
		made = append(made, "/data/"+bucket)
	}

	container, err := testcontainers.Run(
		t.Context(), MinIOImage,
		testcontainers.WithEnv(map[string]string{
			"MINIO_ROOT_USER":     MinIOAccessKey,
			"MINIO_ROOT_PASSWORD": MinIOSecretKey,
		}),
		// The image's entrypoint prepends `minio` to whatever it is given, so a
		// command is a minio sub-command and `sh` is refused by name. Measured,
		// not assumed: the container exited 1 saying so.
		testcontainers.WithEntrypoint("sh", "-c",
			"mkdir -p "+strings.Join(made, " ")+" && exec minio server /data"),
		testcontainers.WithCmd(),
		testcontainers.WithExposedPorts(minioPort),
		// Readiness and not liveness, and the measurement behind that: on this
		// pinned digest /minio/health/live answers 200 up to ~15ms before
		// /minio/health/cluster does, and a signed PUT in that window is 503.
		// Under /live the full integration suite failed twice out of two on a
		// signed upload; under /cluster, three runs and no 503. Two reproductions
		// against a window that narrow is evidence of the mechanism, not proof
		// the flake is gone.
		testcontainers.WithWaitStrategy(
			wait.ForHTTP("/minio/health/cluster").WithPort(minioPort),
		),
	)

	// Registered before the error is checked, like GoTrue's: a wait strategy that
	// times out leaves a started container behind, and the suite runs with Ryuk
	// disabled, so nothing else would reap it.
	t.Cleanup(func() {
		ctx := context.WithoutCancel(t.Context())
		if err := testcontainers.TerminateContainer(container, testcontainers.StopContext(ctx)); err != nil {
			t.Errorf("terminating the MinIO container: %v", err)
		}
	})

	if err != nil {
		t.Fatalf("starting MinIO: %v", err)
	}

	host, err := container.Host(t.Context())
	if err != nil {
		t.Fatalf("reading the MinIO host: %v", err)
	}
	port, err := container.MappedPort(t.Context(), minioPort)
	if err != nil {
		t.Fatalf("reading the MinIO port: %v", err)
	}

	return &ObjectStore{Endpoint: fmt.Sprintf("http://%s:%s", host, port.Port())}
}
