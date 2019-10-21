package rlogs_test

import (
	"fmt"
	"log"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/m-mizutani/rlogs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type dummyS3ClientForBasicReader struct{}

func (x *dummyS3ClientForBasicReader) GetObject(input *s3.GetObjectInput) (*s3.GetObjectOutput, error) {
	if *input.Bucket != "some-bucket" {
		return nil, fmt.Errorf("invalid bucket")
	}

	switch *input.Key {
	case "magic/history.json":
		lines := []string{
			`{"ts":"1902-10-10T10:00:00","name":"?","number":1}`,
			`{"ts":"1929-10-10T10:00:00","name":"Parallel Worlds","number":2}`,
			`{"ts":"1954-10-10T10:00:00","name":"Heaven's Feel","number":3}`,
			`{"ts":"1983-10-10T10:00:00","name":"?","number":4}`,
			`{"ts":"1991-10-10T10:00:00","name":"Blue","number":5}`,
		}
		return &s3.GetObjectOutput{
			Body: toReadCloser(strings.Join(lines, "\n")),
		}, nil

	case "http/log.json":
		lines := []string{
			`{"ts":"2019-10-10T10:00:00","src":"10.1.2.3","port":34567,"path":"/hello"}`,
			`{"ts":"2019-10-10T10:00:02","src":"10.2.3.4","port":45678,"path":"/world"}`,
		}
		return &s3.GetObjectOutput{
			Body: toReadCloser(strings.Join(lines, "\n")),
		}, nil

	default:
		return nil, fmt.Errorf("Key not found")
	}
}

func TestBasicReader(t *testing.T) {
	dummy := dummyS3ClientForBasicReader{}
	rlogs.InjectNewS3Client(&dummy)
	defer rlogs.FixNewS3Client()

	reader := rlogs.BasicReader{
		LogEntries: []*rlogs.LogEntry{
			{
				Psr: &rlogs.JSONParser{
					Tag:             "ts",
					TimestampField:  rlogs.String("ts"),
					TimestampFormat: rlogs.String("2006-01-02T15:04:05"),
				},
				Ldr: &rlogs.S3LineLoader{},
				Src: &rlogs.AwsS3LogSource{
					Region: "some-region",
					Bucket: "some-bucket",
					Key:    "magic/",
				},
			},
		},
	}

	ch := reader.Read(&rlogs.AwsS3LogSource{
		Region: "some-region",
		Bucket: "some-bucket",
		Key:    "magic/history.json",
	})
	var logs []*rlogs.LogRecord
	for q := range ch {
		require.NoError(t, q.Error)
		logs = append(logs, q.Log)
	}

	assert.Equal(t, 5, len(logs))
	v4, ok := logs[4].Values.(map[string]interface{})
	assert.True(t, ok)
	n4, ok := v4["name"].(string)
	assert.True(t, ok)
	assert.Equal(t, "Blue", n4)
}

func ExampleBasicReader() {
	// To avoid accessing actual S3.
	dummy := dummyS3ClientForBasicReader{}
	rlogs.InjectNewS3Client(&dummy)
	defer rlogs.FixNewS3Client()

	// Example is below
	reader := rlogs.BasicReader{
		LogEntries: []*rlogs.LogEntry{
			{
				Psr: &rlogs.JSONParser{
					Tag:             "ts",
					TimestampField:  rlogs.String("ts"),
					TimestampFormat: rlogs.String("2006-01-02T15:04:05"),
				},
				Ldr: &rlogs.S3LineLoader{},
				Src: &rlogs.AwsS3LogSource{
					Region: "some-region",
					Bucket: "some-bucket",
					Key:    "http/",
				},
			},
		},
	}

	// s3://some-bucket/http/log.json is following:
	// {"ts":"2019-10-10T10:00:00","src":"10.1.2.3","port":34567,"path":"/hello"}
	// {"ts":"2019-10-10T10:00:02","src":"10.2.3.4","port":45678,"path":"/world"}

	ch := reader.Read(&rlogs.AwsS3LogSource{
		Region: "some-region",
		Bucket: "some-bucket",
		Key:    "http/log.json",
	})

	for q := range ch {
		if q.Error != nil {
			log.Fatal(q.Error)
		}
		fmt.Printf("[log] tag=%s time=%s values=%v\n", q.Log.Tag, q.Log.Timestamp, q.Log.Values)
	}
	// Output:
	// [log] tag=ts time=2019-10-10 10:00:00 +0000 UTC values=map[path:/hello port:34567 src:10.1.2.3 ts:2019-10-10T10:00:00]
	// [log] tag=ts time=2019-10-10 10:00:02 +0000 UTC values=map[path:/world port:45678 src:10.2.3.4 ts:2019-10-10T10:00:02]
}
