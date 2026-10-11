// SPDX-License-Identifier: GPL-3.0-or-later

package attachment

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// These tests moved here from operation/attachment with the upload timeout: the
// use case names the upload's own budget when that is what ran out, and not a
// deadline the caller set.

func TestWrapUploadTimeout_ParentDeadlineNotMisattributed(t *testing.T) {
	t.Parallel()
	parentCtx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	<-parentCtx.Done()
	uploadCtx, uploadCancel := context.WithTimeout(parentCtx, time.Hour)
	defer uploadCancel()
	<-uploadCtx.Done()

	s := &Service{uploadTimeout: time.Minute}
	got := s.wrapUploadTimeout(parentCtx, uploadCtx, errors.New("context deadline exceeded"))
	if strings.Contains(got.Error(), "upload timed out after") {
		t.Fatalf("misattributed: %v", got)
	}
}

func TestWrapUploadTimeout_OwnBudgetStillReported(t *testing.T) {
	t.Parallel()
	uploadCtx, uploadCancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer uploadCancel()
	<-uploadCtx.Done()

	s := &Service{uploadTimeout: time.Minute}
	got := s.wrapUploadTimeout(context.Background(), uploadCtx, errors.New("context deadline exceeded"))
	if !strings.Contains(got.Error(), "upload timed out after 1m0s") {
		t.Fatalf("got: %v", got)
	}
}
