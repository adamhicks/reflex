package rblob_test

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	_ "gocloud.dev/blob/s3blob" // Driver for test

	"github.com/luno/reflex/rblob"
)

var (
	s3url   = flag.String("test_s3_url", "", "Define to enable s3 integration test")
	s3after = flag.String("test_s3_after", "", "Define to stream after this event id")
)

// TestS3 provides an integration test for streaming json events from a s3 bucket. It prints
// event ids and metadata (content). It obtains the AWS session from the environment.
//
// Usage:
//
//	export URL="s3://my_bucket?prefix=optional/prefix/"
//	export AFTER_ID="" # Ex. set to '2020|eof' to start from 2020 if first part of key is year.
//	go test github.com/luno/reflex/rblob -v -run TestS3 -test_s3_url="$URL" -test_s3_after="$AFTER_ID"
func TestS3(t *testing.T) {
	if *s3url == "" {
		t.Skip("Skipping s3 integration test, test_s3_url flag empty.")
		return
	}

	if !strings.HasPrefix(*s3url, "s3://") {
		t.Errorf("test_s3_url requires 's3://' prefix")
		return
	}

	ctx := context.Background()

	b, err := rblob.OpenBucket(ctx, "", *s3url)
	require.NoError(t, err)

	sc, err := b.Stream(ctx, *s3after)
	require.NoError(t, err)

	for {
		e, err := sc.Recv()
		require.NoError(t, err)

		fmt.Println(e.ID)
		fmt.Printf("%s\n\n", e.MetaData)
	}
}
