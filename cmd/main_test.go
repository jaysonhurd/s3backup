package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jaysonhurd/s3backup/models"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInformationalFlagsDoNotLoadConfig(t *testing.T) {
	for _, args := range [][]string{{"-help"}, {"-h"}, {"-version"}, {}} {
		var out, errOut bytes.Buffer
		if err := run(context.Background(), args, strings.NewReader(""), &out, &errOut); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if out.Len()+errOut.Len() == 0 {
			t.Fatalf("%v produced no output", args)
		}
	}
}
func TestInvalidCLI(t *testing.T) {
	for _, args := range [][]string{{"-llevel", "wrong"}, {"-timeout", "-1s"}, {"-force"}, {"unexpected"}, {"-backup", "-config", "missing.json"}} {
		var out bytes.Buffer
		if err := run(context.Background(), args, strings.NewReader(""), &out, &out); err == nil {
			t.Fatalf("%v should fail", args)
		}
	}
}

func TestCLIWorkflowAgainstLocalS3(t *testing.T) {
	for _, scenario := range []string{"wipe only", "decline wipe", "backup failure prevents sync", "backup success", "dry run"} {
		t.Run(scenario, func(t *testing.T) {
			lists, deletes, puts := 0, 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodHead:
					if scenario == "backup success" {
						w.WriteHeader(http.StatusNotFound)
					} else {
						w.WriteHeader(http.StatusForbidden)
					}
				case http.MethodGet:
					lists++
					w.Header().Set("Content-Type", "application/xml")
					fmt.Fprint(w, `<ListBucketResult><IsTruncated>false</IsTruncated><Contents><Key>old</Key></Contents></ListBucketResult>`)
				case http.MethodPost:
					deletes++
					w.Header().Set("Content-Type", "application/xml")
					fmt.Fprint(w, `<DeleteResult/>`)
				case http.MethodPut:
					puts++
					data, err := io.ReadAll(r.Body)
					if err != nil {
						t.Error(err)
					}
					digest := sha256.Sum256(data)
					checksum := base64.StdEncoding.EncodeToString(digest[:])
					if string(data) != "data" || r.Header.Get("X-Amz-Checksum-Sha256") != checksum {
						t.Errorf("invalid body/checksum: %q", data)
					}
					if r.Header.Get("If-None-Match") != "*" {
						t.Error("missing conditional header on wire")
					}
					if r.Header.Get("X-Amz-Meta-S3backup-Sha256") != checksum {
						t.Error("missing checksum metadata on wire")
					}
					w.Header().Set("X-Amz-Checksum-Sha256", checksum)
					w.WriteHeader(http.StatusOK)
				default:
					t.Errorf("unexpected request %s", r.Method)
					w.WriteHeader(500)
				}
			}))
			defer server.Close()
			t.Setenv("AWS_ENDPOINT_URL_S3", server.URL)
			dir := t.TempDir()
			source := filepath.Join(dir, "source")
			if err := os.Mkdir(source, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(source, "file"), []byte("data"), 0o600); err != nil {
				t.Fatal(err)
			}
			cfg := models.Config{AWS: models.AWS{S3Bucket: "bucket", S3Region: "us-east-1", AccessKeyId: "test", SecretAccessKey: "test", BackupDirectories: []string{source}}, Logging: models.Logging{LogfileLocation: filepath.Join(dir, "logs")}}
			data, err := json.Marshal(cfg)
			if err != nil {
				t.Fatal(err)
			}
			configFile := filepath.Join(dir, "config.json")
			if err := os.WriteFile(configFile, data, 0o600); err != nil {
				t.Fatal(err)
			}
			args := []string{"-config", configFile}
			answer := ""
			switch scenario {
			case "wipe only":
				args = append(args, "-wipe")
				answer = "bucket\n"
			case "decline wipe":
				args = append(args, "-wipe")
				answer = "y\n"
			case "backup failure prevents sync":
				args = append(args, "-backup", "-sync")
			case "backup success":
				args = append(args, "-backup")
			case "dry run":
				args = append(args, "-wipe", "-dry-run")
			}
			var output bytes.Buffer
			err = run(context.Background(), args, strings.NewReader(answer), &output, &output)
			wantErr := scenario == "decline wipe" || scenario == "backup failure prevents sync"
			if (err != nil) != wantErr {
				t.Fatalf("err=%v wantErr=%v", err, wantErr)
			}
			if scenario == "wipe only" && (lists != 1 || deletes != 1) {
				t.Fatalf("lists=%d deletes=%d", lists, deletes)
			}
			if scenario == "backup success" && (puts != 1 || deletes != 0 || lists != 0) {
				t.Fatalf("puts=%d deletes=%d lists=%d", puts, deletes, lists)
			}
			if scenario != "wipe only" && scenario != "backup success" && (deletes != 0 || puts != 0) {
				t.Fatal("unexpected mutation")
			}
			if scenario == "backup failure prevents sync" && lists != 0 {
				t.Fatal("sync ran after backup failure")
			}
		})
	}
}

func TestConfirmationCancellation(t *testing.T) {
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := readAnswer(ctx, reader); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}
