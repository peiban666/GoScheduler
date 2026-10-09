package schedule

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestLegacyLongMinuteIntervalsKeepTheirExistingDuration(t *testing.T) {
	for _, minutes := range []int{90, 525600} {
		t.Run(fmt.Sprintf("%dm", minutes), func(t *testing.T) {
			parsed, err := Parse(fmt.Sprintf("@every %dm", minutes))
			if err != nil {
				t.Fatal(err)
			}
			after := time.Date(2026, 10, 7, 23, 58, 47, 625000000, time.FixedZone("Asia/Shanghai", 8*60*60))
			delay := time.Duration(minutes) * time.Minute
			want := after.Truncate(time.Minute).Add(delay)
			for tick := 0; tick < 10; tick++ {
				next := parsed.Next(after)
				if !next.Equal(want) {
					t.Fatalf("tick %d: Next(%s) = %s, want %s", tick, after, next, want)
				}
				if next.Second() != 0 || next.Nanosecond() != 0 || !next.After(after) {
					t.Fatal("minute interval is not strictly future or aligned to second 00")
				}
				after = next
				want = want.Add(delay)
			}
		})
	}
}

func TestMinuteStepsStartAtMinuteZeroRegardlessOfSaveTime(t *testing.T) {
	zone := time.FixedZone("Asia/Shanghai", 8*60*60)
	for _, minutes := range []int{1, 5, 7, 10, 30, 60} {
		for _, spec := range []string{fmt.Sprintf("0 */%d * * * *", minutes), fmt.Sprintf("@every %dm", minutes)} {
			t.Run(spec, func(t *testing.T) {
				parsed, err := Parse(spec)
				if err != nil {
					t.Fatal(err)
				}
				for _, start := range []time.Time{
					time.Date(2026, 10, 9, 12, 3, 47, 625000000, zone),
					time.Date(2026, 10, 9, 12, 55, 0, 0, zone),
					time.Date(2026, 10, 9, 23, 58, 47, 0, zone),
				} {
					after := start
					for tick := 0; tick < 15; tick++ {
						want := after.Truncate(time.Minute).Add(time.Minute)
						for want.Minute()%minutes != 0 {
							want = want.Add(time.Minute)
						}
						next := parsed.Next(after)
						if !next.Equal(want) || next.Second() != 0 || next.Nanosecond() != 0 {
							t.Fatalf("Next(%s) = %s, want %s", after, next, want)
						}
						// Re-registering a schedule at this point must not shift its anchor.
						restarted, err := Parse(spec)
						if err != nil || !restarted.Next(after).Equal(next) {
							t.Fatal("schedule changed on re-registration")
						}
						after = next
					}
				}
			})
		}
	}
}

func TestHourStepsStartAtMidnight(t *testing.T) {
	zone := time.FixedZone("Asia/Shanghai", 8*60*60)
	for _, hours := range []int{1, 2, 3, 5, 6, 12, 24} {
		for _, minute := range []int{0, 30, 59} {
			spec := fmt.Sprintf("0 %d */%d * * *", minute, hours)
			parsed, err := Parse(spec)
			if err != nil {
				t.Fatal(err)
			}
			after := time.Date(2026, 10, 9, 13, 7, 47, 0, zone)
			for tick := 0; tick < 10; tick++ {
				want := after.Truncate(time.Minute).Add(time.Minute)
				for want.Hour()%hours != 0 || want.Minute() != minute {
					want = want.Add(time.Minute)
				}
				next := parsed.Next(after)
				if !next.Equal(want) {
					t.Fatalf("%s Next(%s) = %s, want %s", spec, after, next, want)
				}
				after = next
			}
		}
	}
}

func TestNonDivisorStepsResetAtHourAndDayBoundaries(t *testing.T) {
	minutes, err := Parse("0 */7 * * * *")
	if err != nil {
		t.Fatal(err)
	}
	after := time.Date(2026, 10, 9, 12, 56, 0, 0, time.UTC)
	if next := minutes.Next(after); !next.Equal(after.Add(4 * time.Minute)) {
		t.Fatalf("minute step did not reset to 13:00: %s", next)
	}
	hours, err := Parse("0 0 */5 * * *")
	if err != nil {
		t.Fatal(err)
	}
	after = time.Date(2026, 10, 9, 20, 0, 0, 0, time.UTC)
	if next := hours.Next(after); !next.Equal(after.Add(4 * time.Hour)) {
		t.Fatalf("hour step did not reset to midnight: %s", next)
	}
}

func TestDailyTimesWithoutCrossProduct(t *testing.T) {
	schedule, err := Parse("0 0 3,20 * * *\n0 30 7 * * *")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 7, 0, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	expected := []time.Time{
		now.Add(3 * time.Hour),
		now.Add(7*time.Hour + 30*time.Minute),
		now.Add(20 * time.Hour),
		now.Add(24*time.Hour + 3*time.Hour),
	}
	for _, want := range expected {
		next := schedule.Next(now)
		if !next.Equal(want) {
			t.Fatalf("Next(%s) = %s, want %s", now, next, want)
		}
		now = next
	}
}

func TestOverlappingSchedulesFireOnce(t *testing.T) {
	schedule, err := Parse("0 0 3,20 * * *;0 0 3 * * *")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
	first := schedule.Next(now)
	second := schedule.Next(first)
	if first.Hour() != 3 || second.Hour() != 20 {
		t.Fatalf("unexpected executions: %s, %s", first, second)
	}
}

func TestLegacySchedules(t *testing.T) {
	cases := []struct {
		spec  string
		after time.Time
		want  time.Time
	}{
		{"0 * * * * *", time.Date(2026, 10, 7, 12, 34, 15, 0, time.UTC), time.Date(2026, 10, 7, 12, 35, 0, 0, time.UTC)},
		{"0 15 * * * *", time.Date(2026, 10, 7, 12, 20, 0, 0, time.UTC), time.Date(2026, 10, 7, 13, 15, 0, 0, time.UTC)},
		{"0 30 13,17 * * *", time.Date(2026, 10, 7, 13, 30, 0, 0, time.UTC), time.Date(2026, 10, 7, 17, 30, 0, 0, time.UTC)},
		{"0 30 17,23 * *", time.Date(2026, 10, 9, 17, 30, 0, 0, time.UTC), time.Date(2026, 10, 9, 23, 30, 0, 0, time.UTC)},
		{"@every 10s", time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC), time.Date(2026, 10, 7, 12, 0, 10, 0, time.UTC)},
		{"@every 60s", time.Date(2026, 10, 7, 12, 0, 17, 0, time.UTC), time.Date(2026, 10, 7, 12, 1, 17, 0, time.UTC)},
	}
	for _, tc := range cases {
		t.Run(tc.spec, func(t *testing.T) {
			schedule, err := Parse(tc.spec)
			if err != nil {
				t.Fatal(err)
			}
			if got := schedule.Next(tc.after); !got.Equal(tc.want) {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestMinuteIntervalAtAndNearMinuteBoundary(t *testing.T) {
	parsed, err := Parse("@every 1m")
	if err != nil {
		t.Fatal(err)
	}
	for _, second := range []int{0, 1, 17, 59} {
		after := time.Date(2026, 10, 7, 21, 7, second, 999999999, time.UTC)
		want := time.Date(2026, 10, 7, 21, 8, 0, 0, time.UTC)
		if next := parsed.Next(after); !next.Equal(want) {
			t.Fatalf("after %s got %s, want %s", after, next, want)
		}
	}
}

func TestNormalize(t *testing.T) {
	got, err := Normalize(" 0  30 7 * * * ;\r\n0 0 20 * * *\n0 30 7 * * * ")
	if err != nil {
		t.Fatal(err)
	}
	if got != "0 30 7 * * *\n0 0 20 * * *" {
		t.Fatalf("unexpected normalized spec: %q", got)
	}
}

func TestRejectInvalidSchedules(t *testing.T) {
	for _, spec := range []string{"", ";\n", "invalid", "0 0 25 * * *", "0 0 7 * * *\ninvalid", strings.Repeat("x", MaxSpecLength+1)} {
		t.Run(spec, func(t *testing.T) {
			if _, err := Parse(spec); err == nil {
				t.Fatalf("accepted invalid spec %q", spec)
			}
		})
	}
}

func TestIntervalRulesRemainStandalone(t *testing.T) {
	if _, err := Parse("@every 10s;@every 15s"); err == nil {
		t.Fatal("mixed interval rules must be rejected")
	}
	if _, err := Parse("@every 10s;0 0 7 * * *"); err == nil {
		t.Fatal("mixed interval/calendar rules must be rejected")
	}
	if _, err := Parse("@every 10s;@every 10s"); err != nil {
		t.Fatal("deduplicated standalone interval must remain valid", err)
	}
}
