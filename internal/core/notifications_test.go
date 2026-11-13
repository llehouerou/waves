package core

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/llehouerou/waves/internal/config"
	"github.com/llehouerou/waves/internal/notify"
	"github.com/llehouerou/waves/internal/tags"
)

// fakeNotifier is the desktop of these tests: it keeps what it is told, and
// can take its time to answer, as a notification daemon may.
type fakeNotifier struct {
	mu   sync.Mutex
	sent []notify.Notification
	last uint32
	hold chan struct{} // when set, an answer waits for it to be closed
}

func (f *fakeNotifier) Notify(n notify.Notification) (uint32, error) {
	f.mu.Lock()
	f.sent = append(f.sent, n)
	f.last++
	id, hold := f.last, f.hold
	f.mu.Unlock()
	if hold != nil {
		<-hold
	}
	return id, nil
}

func (f *fakeNotifier) Close(uint32) error { return nil }

func (f *fakeNotifier) told() []notify.Notification {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.sent)
}

// wait returns what the desktop was told once that is n notifications.
func (f *fakeNotifier) wait(t *testing.T, n int) []notify.Notification {
	t.Helper()
	eventually(t, "the desktop is told", func() bool { return len(f.told()) >= n })
	return f.told()
}

// notificationsOn is a [notifications] section asking for them all, without
// the cover.
func notificationsOn() config.NotificationsConfig {
	yes := true
	return config.NotificationsConfig{Enabled: &yes, NowPlaying: &yes, Downloads: &yes, Timeout: 5000}
}

func TestNotifications_ATrackThatStartsIsToldWithNoClientDrivingIt(t *testing.T) {
	desktop := &fakeNotifier{}
	c, mock := newTestCore(t, Options{Notifications: notificationsOn(), notifier: desktop})

	playTrack(t, c, mock, tags.Tag{Path: "/music/lullaby.flac", Artist: "Low", Title: "Lullaby", Album: "I Could Live in Hope"}, 3*time.Minute)

	// Not an event is read, nothing more is asked: the core tells the desktop.
	told := desktop.wait(t, 1)
	want := notify.Notification{Title: "Lullaby", Body: "Low · I Could Live in Hope", Timeout: 5000, Urgency: notify.UrgencyLow}
	if told[0] != want {
		t.Errorf("the desktop was told %+v, want %+v", told[0], want)
	}
}

// expectNothingTold fails the test if the desktop is told anything within the
// quiet period.
func expectNothingTold(t *testing.T, desktop *fakeNotifier) {
	t.Helper()
	time.Sleep(quiet)
	if told := desktop.told(); len(told) != 0 {
		t.Errorf("the desktop was told %+v, want nothing", told)
	}
}

func TestNotifications_ATrackIsNotToldUnlessAskedFor(t *testing.T) {
	yes, no := true, false
	tests := []struct {
		name string
		cfg  config.NotificationsConfig
	}{
		{"no [notifications] section", config.NotificationsConfig{}},
		{"notifications disabled", config.NotificationsConfig{Enabled: &no, NowPlaying: &yes}},
		{"now playing disabled", config.NotificationsConfig{Enabled: &yes, NowPlaying: &no, Downloads: &yes}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			desktop := &fakeNotifier{}
			c, mock := newTestCore(t, Options{Notifications: tt.cfg, notifier: desktop})

			playTrack(t, c, mock, tags.Tag{Path: "/music/lullaby.flac", Artist: "Low", Title: "Lullaby"}, 3*time.Minute)
			nextEvent[StateChanged](t, c) // the loop has seen the track start

			expectNothingTold(t, desktop)
		})
	}
}

// A core nobody asked for notifications must not even connect to the session
// bus: the tests of a client build theirs this way, on a machine that has one.
func TestNotifications_ZeroOptionsOpenNoDesktop(t *testing.T) {
	c, mock := newTestCore(t, Options{})
	if c.notifications.out != nil {
		t.Errorf("the core opened %T, want no desktop", c.notifications.out)
	}

	playTrack(t, c, mock, tags.Tag{Path: "/music/lullaby.flac", Artist: "Low", Title: "Lullaby"}, 3*time.Minute)
	nextEvent[StateChanged](t, c) // the loop has seen the track start, and lives
	if err := c.Playback().Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if e := nextEvent[StateChanged](t, c); e.Current != StateStopped {
		t.Errorf("StateChanged = %+v, want playback stopped", e)
	}
}

// next queues a track after the one playing and moves to it, the way a client
// would.
func next(t *testing.T, c *Core, title string) {
	t.Helper()
	c.Playback().AddTracks(Track{Path: "/music/" + title + ".flac", Artist: "Low", Title: title})
	if err := c.Playback().Next(); err != nil {
		t.Fatalf("next: %v", err)
	}
}

// One notification says what plays: a stack of them, one per track, would say
// what played.
func TestNotifications_TheNextTrackReplacesTheOneTold(t *testing.T) {
	desktop := &fakeNotifier{}
	c, mock := newTestCore(t, Options{Notifications: notificationsOn(), notifier: desktop})
	playTrack(t, c, mock, tags.Tag{Path: "/music/lullaby.flac", Artist: "Low", Title: "Lullaby"}, 3*time.Minute)
	desktop.wait(t, 1)

	next(t, c, "Sea")
	if told := desktop.wait(t, 2); told[1].Title != "Sea" || told[1].ReplacesID != 1 {
		t.Errorf("the desktop was told %+v, want Sea replacing notification 1", told[1])
	}

	next(t, c, "Words")
	if told := desktop.wait(t, 3); told[2].Title != "Words" || told[2].ReplacesID != 2 {
		t.Errorf("the desktop was told %+v, want Words replacing notification 2", told[2])
	}
}

// A play after a stop, on another track than the one stopped, is told once.
func TestNotifications_APlayAfterAStopOnAnotherTrackIsToldOnce(t *testing.T) {
	desktop := &fakeNotifier{}
	c, mock := newTestCore(t, Options{Notifications: notificationsOn(), notifier: desktop})
	playTrack(t, c, mock, tags.Tag{Path: "/music/lullaby.flac", Artist: "Low", Title: "Lullaby"}, 3*time.Minute)
	desktop.wait(t, 1)

	pb := c.Playback()
	if err := pb.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	pb.AddTracks(Track{Path: "/music/sea.flac", Artist: "Low", Title: "Sea"})
	pb.QueueMoveTo(1)
	if err := pb.Play(); err != nil {
		t.Fatalf("play: %v", err)
	}

	if told := desktop.wait(t, 2); told[1].Title != "Sea" || told[1].ReplacesID != 1 {
		t.Errorf("the desktop was told %+v, want Sea replacing notification 1", told[1])
	}
	time.Sleep(quiet)
	if told := desktop.told(); len(told) != 2 {
		t.Errorf("the desktop was told %+v, want Lullaby then Sea only", told)
	}
}

// Moving the queue while stopped plays nothing: there is nothing to tell until
// a track starts.
func TestNotifications_AMoveWhileStoppedIsNotTold(t *testing.T) {
	desktop := &fakeNotifier{}
	c, _ := newTestCore(t, Options{Notifications: notificationsOn(), notifier: desktop})
	pb := c.Playback()
	pb.AddTracks(
		Track{Path: "/music/lullaby.flac", Artist: "Low", Title: "Lullaby"},
		Track{Path: "/music/sea.flac", Artist: "Low", Title: "Sea"},
		Track{Path: "/music/words.flac", Artist: "Low", Title: "Words"},
	)
	pb.QueueMoveTo(0)

	for _, move := range []func() error{pb.Next, pb.Previous, func() error { return pb.JumpTo(2) }} {
		if err := move(); err != nil {
			t.Fatalf("move: %v", err)
		}
	}
	expectNothingTold(t, desktop)

	if err := pb.Play(); err != nil {
		t.Fatalf("play: %v", err)
	}
	if told := desktop.wait(t, 1); told[0].Title != "Words" {
		t.Errorf("the desktop was told %+v, want Words", told[0])
	}
	time.Sleep(quiet)
	if told := desktop.told(); len(told) != 1 {
		t.Errorf("the desktop was told %+v, want Words only", told)
	}
}

// Resuming from pause goes on with the same track: nothing new to tell.
func TestNotifications_AResumeIsNotTold(t *testing.T) {
	desktop := &fakeNotifier{}
	c, mock := newTestCore(t, Options{Notifications: notificationsOn(), notifier: desktop})
	playTrack(t, c, mock, tags.Tag{Path: "/music/lullaby.flac", Artist: "Low", Title: "Lullaby"}, 3*time.Minute)
	desktop.wait(t, 1)

	if err := c.Playback().Pause(); err != nil {
		t.Fatalf("pause: %v", err)
	}
	if err := c.Playback().Toggle(); err != nil {
		t.Fatalf("resume: %v", err)
	}

	time.Sleep(quiet)
	if told := desktop.told(); len(told) != 1 {
		t.Errorf("the desktop was told %+v, want Lullaby once", told)
	}
}

// A track that does not open never played.
func TestNotifications_ATrackThatFailsToOpenIsNotTold(t *testing.T) {
	desktop := &fakeNotifier{}
	c, mock := newTestCore(t, Options{Notifications: notificationsOn(), notifier: desktop})
	mock.SetPlayError(errors.New("unreadable"))

	c.Playback().AddTracks(Track{Path: "/music/lullaby.flac", Artist: "Low", Title: "Lullaby"})
	c.Playback().QueueMoveTo(0)
	if err := c.Playback().Play(); err == nil {
		t.Fatal("play succeeded, want an error")
	}

	expectNothingTold(t, desktop)
}

// holding makes the desktop keep its answers until the function returned is
// called. Call it after the core is built: the answers are let go before the
// core closes, even when the test fails.
func holding(t *testing.T, desktop *fakeNotifier) (release func()) {
	t.Helper()
	hold := make(chan struct{})
	desktop.mu.Lock()
	desktop.hold = hold
	desktop.mu.Unlock()
	release = sync.OnceFunc(func() { close(hold) })
	t.Cleanup(release)
	return release
}

// The desktop answers when it wants. What it is told next is what plays by
// then, once: not each track skipped meanwhile, one after the other.
func TestNotifications_TracksSkippedWhileOneIsBeingToldAreNotTold(t *testing.T) {
	desktop := &fakeNotifier{}
	c, mock := newTestCore(t, Options{Notifications: notificationsOn(), notifier: desktop})
	release := holding(t, desktop)
	playTrack(t, c, mock, tags.Tag{Path: "/music/lullaby.flac", Artist: "Low", Title: "Lullaby"}, 3*time.Minute)
	desktop.wait(t, 1) // Lullaby is being told

	next(t, c, "Sea")
	nextEvent[TrackChanged](t, c)
	next(t, c, "Words")
	nextEvent[TrackChanged](t, c) // the loop has seen both start
	release()

	if told := desktop.wait(t, 2); told[1].Title != "Words" || told[1].ReplacesID != 1 {
		t.Errorf("the desktop was told %+v, want Words replacing notification 1", told[1])
	}
	time.Sleep(quiet)
	if told := desktop.told(); len(told) != 2 {
		t.Errorf("the desktop was told %+v, want Lullaby then Words only", told)
	}
}

func TestNotifications_TheCoverOfTheTrackIsItsIconWhenAskedFor(t *testing.T) {
	dir := t.TempDir()
	cover := filepath.Join(dir, "cover.jpg")
	if err := os.WriteFile(cover, []byte("\xff\xd8\xff\xe0 a cover"), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		show bool
		want string
	}{
		{"asked for", true, "file://" + cover},
		{"not asked for", false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := notificationsOn()
			cfg.ShowAlbumArt = &tt.show
			desktop := &fakeNotifier{}
			c, mock := newTestCore(t, Options{Notifications: cfg, notifier: desktop})

			playTrack(t, c, mock, tags.Tag{Path: filepath.Join(dir, "lullaby.flac"), Artist: "Low", Title: "Lullaby"}, 3*time.Minute)

			if told := desktop.wait(t, 1); told[0].Icon != tt.want {
				t.Errorf("icon = %q, want %q", told[0].Icon, tt.want)
			}
		})
	}
}

func TestNotifications_AnImportThatMadeItIsTold(t *testing.T) {
	desktop := &fakeNotifier{}
	c, id, _, source := newImportCore(t, Options{Notifications: notificationsOn(), notifier: desktop})

	startImport(t, c, id, source)

	// Not an event is read: the core tells the desktop on its own.
	told := desktop.wait(t, 1)
	want := notify.Notification{Title: "Download Complete", Body: "Low - Trust", Timeout: 5000, Urgency: notify.UrgencyNormal}
	if told[0] != want {
		t.Errorf("the desktop was told %+v, want %+v", told[0], want)
	}
}

func TestNotifications_AnImportIsNotToldWhenItFailedNorUnlessAskedFor(t *testing.T) {
	no := false
	quietDownloads := notificationsOn()
	quietDownloads.Downloads = &no
	tests := []struct {
		name     string
		cfg      config.NotificationsConfig
		loseFile bool // a file is gone between the plan and the import
	}{
		{"a file failed", notificationsOn(), true},
		{"downloads disabled", quietDownloads, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			desktop := &fakeNotifier{}
			c, id, completed, source := newImportCore(t, Options{Notifications: tt.cfg, notifier: desktop})
			plan, err := c.Downloads().PrepareImport(id)
			if err != nil {
				t.Fatalf("prepare: %v", err)
			}
			if tt.loseFile {
				if err := os.Remove(filepath.Join(completed, "Album", "02.mp3")); err != nil {
					t.Fatal(err)
				}
			}

			if err := c.Downloads().Import(plan, source); err != nil {
				t.Fatalf("import: %v", err)
			}
			if finished := nextEvent[ImportFinished](t, c); AllDone(finished.Files) == tt.loseFile {
				t.Fatalf("files ended %v", fileStates(finished.Files))
			}

			expectNothingTold(t, desktop)
		})
	}
}

// Quitting must not hang on a notification daemon that does not answer.
func TestClose_DoesNotWaitForANotificationBeingSent(t *testing.T) {
	desktop := &fakeNotifier{}
	c, mock := newTestCore(t, Options{Notifications: notificationsOn(), notifier: desktop})
	release := holding(t, desktop)
	playTrack(t, c, mock, tags.Tag{Path: "/music/lullaby.flac", Artist: "Low", Title: "Lullaby"}, 3*time.Minute)
	desktop.wait(t, 1) // Lullaby is being told, and the desktop does not answer

	closeWithin(t, c, release, "the desktop to answer")
}
