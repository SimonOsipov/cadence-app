package config

import (
	"strings"
	"testing"
)

// setRequiredStorage fills in what the object store needs, so that a test about
// one of its variables is not really a test about the first one Load checks.
func setRequiredStorage(t *testing.T) {
	t.Helper()

	t.Setenv("STORAGE_ENDPOINT", "https://s3.twcstorage.ru")
	t.Setenv("STORAGE_REGION", "ru-1")
	t.Setenv("STORAGE_ACCESS_KEY_ID", "an-access-key")
	t.Setenv("STORAGE_SECRET_ACCESS_KEY", "a-secret-key")
	t.Setenv("STORAGE_PATH_STYLE", "true")
	t.Setenv("STORAGE_BUCKET_VIALS", "cadence-vials")
	t.Setenv("STORAGE_BUCKET_INJECTIONS", "cadence-injections")
}

// Every one of these is required, and none has a fallback. A missing endpoint
// or a missing bucket that resolved to a default would sign links against
// somebody else's namespace — and the failure would arrive as a patient's photo
// that will not open, long after the deploy that caused it.
func TestLoadRequiresTheObjectStore(t *testing.T) {
	for _, tc := range []struct {
		name    string
		unset   string
		set     map[string]string
		wantErr string
	}{
		{name: "no endpoint", unset: "STORAGE_ENDPOINT", wantErr: "STORAGE_ENDPOINT is required"},
		{name: "no region", unset: "STORAGE_REGION", wantErr: "STORAGE_REGION is required"},
		{
			name: "no access key", unset: "STORAGE_ACCESS_KEY_ID",
			wantErr: "STORAGE_ACCESS_KEY_ID is required",
		},
		{
			name: "no secret", unset: "STORAGE_SECRET_ACCESS_KEY",
			wantErr: "STORAGE_SECRET_ACCESS_KEY is required",
		},
		{
			name: "no vials bucket", unset: "STORAGE_BUCKET_VIALS",
			wantErr: "STORAGE_BUCKET_VIALS is required",
		},
		{
			name: "no injections bucket", unset: "STORAGE_BUCKET_INJECTIONS",
			wantErr: "STORAGE_BUCKET_INJECTIONS is required",
		},
		{
			// Addressing style is required rather than guessed: path-style and
			// virtual-host style produce different URLs, and the wrong guess is
			// green against a local MinIO and a 404 in the deployment.
			name: "no addressing style", unset: "STORAGE_PATH_STYLE",
			wantErr: "STORAGE_PATH_STYLE is required",
		},
		{
			name:    "an addressing style that is not a boolean",
			set:     map[string]string{"STORAGE_PATH_STYLE": "yes"},
			wantErr: "STORAGE_PATH_STYLE",
		},
		{
			// A host with no scheme parses and then addresses nothing, exactly
			// as it does for the provisioner.
			name:    "an endpoint that is not absolute",
			set:     map[string]string{"STORAGE_ENDPOINT": "s3.twcstorage.ru"},
			wantErr: "must be an absolute http(s) address",
		},
		{
			// Two names for one bucket is a deploy in which a vial label and an
			// injection photo share a namespace, so one can overwrite the other.
			name: "one bucket under two names",
			set: map[string]string{
				"STORAGE_BUCKET_VIALS":      "cadence-photos",
				"STORAGE_BUCKET_INJECTIONS": "cadence-photos",
			},
			wantErr: "name the same bucket",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clearEnv(t)
			setRequired(t)

			if tc.unset != "" {
				t.Setenv(tc.unset, "")
			}
			for key, value := range tc.set {
				t.Setenv(key, value)
			}

			_, err := Load()
			if err == nil {
				t.Fatal("Load accepted a configuration that cannot address the object store")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("Load: %v, want it to mention %q", err, tc.wantErr)
			}
		})
	}
}

// The control for the table above: the same fixture with nothing removed has to
// load, or every row there would pass against a configuration broken for some
// other reason.
func TestLoadReadsTheObjectStore(t *testing.T) {
	clearEnv(t)
	setRequired(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Storage.Endpoint != "https://s3.twcstorage.ru" {
		t.Errorf("endpoint = %q", cfg.Storage.Endpoint)
	}
	if cfg.Storage.Region != "ru-1" {
		t.Errorf("region = %q", cfg.Storage.Region)
	}
	if cfg.Storage.AccessKeyID != "an-access-key" {
		t.Errorf("access key = %q", cfg.Storage.AccessKeyID)
	}
	if cfg.Storage.SecretAccessKey != "a-secret-key" {
		t.Errorf("secret = %q", cfg.Storage.SecretAccessKey)
	}
	if !cfg.Storage.PathStyle {
		t.Error("path style = false, want true")
	}
	if cfg.Storage.VialsBucket != "cadence-vials" {
		t.Errorf("vials bucket = %q", cfg.Storage.VialsBucket)
	}
	if cfg.Storage.InjectionsBucket != "cadence-injections" {
		t.Errorf("injections bucket = %q", cfg.Storage.InjectionsBucket)
	}
}
