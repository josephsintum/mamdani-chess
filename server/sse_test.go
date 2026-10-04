package server

import (
	"testing"
	"time"
)

func TestBroadcasterNeverBlocksOnSlowSubscriber(t *testing.T) {
	b := newBroadcaster()
	stalled, cancelStalled := b.subscribe() // never read until the end
	defer cancelStalled()
	live, cancelLive := b.subscribe()
	defer cancelLive()

	done := make(chan struct{})
	go func() {
		for i := int64(1); i <= 100; i++ {
			b.publish(i)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("publish blocked on a subscriber that isn't reading")
	}

	// Both subscribers hold only the latest value.
	if got := <-stalled; got != 100 {
		t.Fatalf("stalled subscriber got %d, want 100", got)
	}
	if got := <-live; got != 100 {
		t.Fatalf("live subscriber got %d, want 100", got)
	}
}

func TestUnsubscribeStopsDelivery(t *testing.T) {
	b := newBroadcaster()
	ch, cancel := b.subscribe()
	cancel()
	b.publish(1)
	select {
	case v := <-ch:
		t.Fatalf("got %d after unsubscribe", v)
	default:
	}
}
