package s3backup

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/jaysonhurd/s3backup/models"
	//"github.com/jaysonhurd/s3backup/internal/pkg/utilities/utilitiesfakes"
	"os"
	"reflect"
	"testing"
	"time"
)

type mockedBackupData struct {
	cfg models.Config
	awsSess session.Session
	Client s3iface.S3API
	dir string
}

type mockS3Client struct {
	s3iface.S3API
}

func Test_getLocalFileTimestamp(t *testing.T) {
	type args struct {
		file string
	}
	file1, _ := os.Stat("s3backup_test.go")

	var tests = []struct {
		name string
		args args
		want time.Time
		wantErr bool
	}{
		{
			name: "pass_test",
			args: args{
				file: "s3backup_test.go",
			},
			want: file1.ModTime(),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			//var err error
			if got, _ := getLocalFileTimestamp(tt.args.file); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getLocalFileTimestamp() = %v, want %v", got, tt.want)
			}
		})
	}
}
