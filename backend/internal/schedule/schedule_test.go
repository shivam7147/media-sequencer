package schedule

import (
	"testing"
	"time"
)

var anchor = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

// standardItems: A(1, 10s), B(2, 20s), C(3, 15s) — playlist duration 45s.
var standardItems = []Item{
	{MediaID: 1, DurationSeconds: 10},
	{MediaID: 2, DurationSeconds: 20},
	{MediaID: 3, DurationSeconds: 15},
}

func TestCurrentItem(t *testing.T) {
	cases := []struct {
		name          string
		items         []Item
		cycleSeconds  int
		nowOffset     time.Duration // now = anchor + nowOffset
		wantMediaID   int64
		wantRemaining time.Duration
		wantOK        bool
	}{
		{
			name:          "mid-item",
			items:         standardItems,
			cycleSeconds:  100,
			nowOffset:     25 * time.Second,
			wantMediaID:   2,
			wantRemaining: 5 * time.Second,
			wantOK:        true,
		},
		{
			name:          "exact instant an item starts",
			items:         standardItems,
			cycleSeconds:  100,
			nowOffset:     10 * time.Second, // A ends / B starts
			wantMediaID:   2,
			wantRemaining: 20 * time.Second, // full duration of B
			wantOK:        true,
		},
		{
			name:          "exact instant an item ends",
			items:         standardItems,
			cycleSeconds:  100,
			nowOffset:     30 * time.Second, // B ends / C starts
			wantMediaID:   3,
			wantRemaining: 15 * time.Second, // full duration of C
			wantOK:        true,
		},
		{
			name:          "wrap at end of playlist",
			items:         standardItems,
			cycleSeconds:  100,
			nowOffset:     46 * time.Second, // 45 + 1: one second into a second lap
			wantMediaID:   1,
			wantRemaining: 9 * time.Second,
			wantOK:        true,
		},
		{
			name:          "wrap at end of cycle (multiple cycles elapsed)",
			items:         standardItems,
			cycleSeconds:  100,
			nowOffset:     250 * time.Second, // 2 cycles + 50s
			wantMediaID:   1,
			wantRemaining: 5 * time.Second,
			wantOK:        true,
		},
		{
			name:          "empty playlist",
			items:         nil,
			cycleSeconds:  100,
			nowOffset:     30 * time.Second,
			wantMediaID:   0,
			wantRemaining: 70 * time.Second, // time left in the cycle
			wantOK:        false,
		},
		{
			name:          "single item",
			items:         []Item{{MediaID: 7, DurationSeconds: 30}},
			cycleSeconds:  100,
			nowOffset:     10 * time.Second,
			wantMediaID:   7,
			wantRemaining: 20 * time.Second,
			wantOK:        true,
		},
		{
			name:          "one item longer than the whole cycle",
			items:         []Item{{MediaID: 9, DurationSeconds: 500}},
			cycleSeconds:  100,
			nowOffset:     40 * time.Second,
			wantMediaID:   9,
			wantRemaining: 60 * time.Second, // clamped to the cycle boundary, not the item's own 460s left
			wantOK:        true,
		},
		{
			name:          "playlist doesn't divide the cycle evenly",
			items:         standardItems, // P=45
			cycleSeconds:  97,            // 97 = 2*45 + 7: final repetition only gets 7s
			nowOffset:     93 * time.Second,
			wantMediaID:   1,               // 3s into the truncated final repetition of A
			wantRemaining: 4 * time.Second, // A would naturally have 7s left, but only 4s remain in the cycle
			wantOK:        true,
		},
		{
			name:          "now exactly equal to the anchor",
			items:         standardItems,
			cycleSeconds:  100,
			nowOffset:     0,
			wantMediaID:   1,
			wantRemaining: 10 * time.Second,
			wantOK:        true,
		},
		{
			name:          "now before the anchor",
			items:         standardItems,
			cycleSeconds:  100,
			nowOffset:     -10 * time.Second, // must not crash or produce a negative offset
			wantMediaID:   1,
			wantRemaining: 10 * time.Second,
			wantOK:        true,
		},
		{
			name: "zero-duration item mixed in is skipped",
			items: []Item{
				{MediaID: 1, DurationSeconds: 10},
				{MediaID: 99, DurationSeconds: 0},
				{MediaID: 3, DurationSeconds: 15},
			},
			cycleSeconds:  100,
			nowOffset:     12 * time.Second,
			wantMediaID:   3,
			wantRemaining: 13 * time.Second,
			wantOK:        true,
		},
		{
			name:          "remaining clamps to the minimum instead of a fraction of a second",
			items:         []Item{{MediaID: 5, DurationSeconds: 100}},
			cycleSeconds:  100,
			nowOffset:     99*time.Second + 500*time.Millisecond, // naturally 0.5s left
			wantMediaID:   5,
			wantRemaining: minRemaining,
			wantOK:        true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			now := anchor.Add(tc.nowOffset)
			got, ok := CurrentItem(anchor, tc.items, tc.cycleSeconds, now)

			if ok != tc.wantOK {
				t.Errorf("ok = %v, want %v", ok, tc.wantOK)
			}
			if got.MediaID != tc.wantMediaID {
				t.Errorf("MediaID = %d, want %d", got.MediaID, tc.wantMediaID)
			}
			if got.Remaining != tc.wantRemaining {
				t.Errorf("Remaining = %v, want %v", got.Remaining, tc.wantRemaining)
			}
			if got.IsSync {
				t.Errorf("IsSync = true, want false (CurrentItem never sets it)")
			}
		})
	}
}

func TestResolve(t *testing.T) {
	cases := []struct {
		name          string
		sync          *SyncEvent
		nowOffset     time.Duration
		wantMediaID   int64
		wantRemaining time.Duration
		wantIsSync    bool
	}{
		{
			name:          "no sync falls back to the base schedule",
			sync:          nil,
			nowOffset:     25 * time.Second,
			wantMediaID:   2,
			wantRemaining: 5 * time.Second,
			wantIsSync:    false,
		},
		{
			name: "sync active",
			sync: &SyncEvent{
				MediaID:         42,
				StartAt:         anchor.Add(20 * time.Second),
				DurationSeconds: 10,
			},
			nowOffset:     25 * time.Second, // 5s into the sync window
			wantMediaID:   42,
			wantRemaining: 5 * time.Second,
			wantIsSync:    true,
		},
		{
			name: "sync just expired falls back to the base schedule",
			sync: &SyncEvent{
				MediaID:         42,
				StartAt:         anchor.Add(20 * time.Second),
				DurationSeconds: 10, // window is [20s, 30s)
			},
			nowOffset:     30 * time.Second, // exactly at the end instant
			wantMediaID:   3,                // base schedule: C starts at 30s
			wantRemaining: 15 * time.Second,
			wantIsSync:    false,
		},
		{
			name: "sync in the future falls back to the base schedule",
			sync: &SyncEvent{
				MediaID:         42,
				StartAt:         anchor.Add(50 * time.Second),
				DurationSeconds: 10,
			},
			nowOffset:     25 * time.Second, // before the sync window starts
			wantMediaID:   2,
			wantRemaining: 5 * time.Second,
			wantIsSync:    false,
		},
		{
			name: "sync with a non-positive duration is ignored",
			sync: &SyncEvent{
				MediaID:         42,
				StartAt:         anchor.Add(20 * time.Second),
				DurationSeconds: 0,
			},
			nowOffset:     25 * time.Second,
			wantMediaID:   2,
			wantRemaining: 5 * time.Second,
			wantIsSync:    false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			now := anchor.Add(tc.nowOffset)
			got := Resolve(anchor, standardItems, 100, tc.sync, now)

			if got.MediaID != tc.wantMediaID {
				t.Errorf("MediaID = %d, want %d", got.MediaID, tc.wantMediaID)
			}
			if got.Remaining != tc.wantRemaining {
				t.Errorf("Remaining = %v, want %v", got.Remaining, tc.wantRemaining)
			}
			if got.IsSync != tc.wantIsSync {
				t.Errorf("IsSync = %v, want %v", got.IsSync, tc.wantIsSync)
			}
		})
	}
}
