// SPDX-License-Identifier: GPL-3.0-only

package provider

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"thing/internal/model"
)

func TestOpenAIComplete_UsesRequestTimeout(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
	}))

	done := make(chan error, 1)
	go func() {
		o := NewOpenAI("secret", srv.URL, 20*time.Millisecond)
		_, err := o.Complete(context.Background(), model.Chat{})
		done <- err
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("server did not receive request")
	}

	var err error
	select {
	case err = <-done:
	case <-time.After(time.Second):
		t.Fatal("Complete did not return before the timeout")
	}
	close(release)
	srv.Close()

	if err == nil {
		t.Fatal("Complete returned nil error, want timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error = %v, want context.DeadlineExceeded", err)
	}
}

func TestOpenAIComplete_ParentCancellationWins(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
	}))

	ctx, cancel := context.WithCancel(context.Background())
	o := NewOpenAI("secret", srv.URL, time.Minute)
	done := make(chan error, 1)
	go func() {
		_, err := o.Complete(ctx, model.Chat{})
		done <- err
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("server did not receive request")
	}
	cancel()

	var err error
	select {
	case err = <-done:
	case <-time.After(time.Second):
		t.Fatal("Complete did not return after cancellation")
	}
	close(release)
	srv.Close()

	if err == nil {
		t.Fatal("Complete returned nil error, want cancellation error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}
