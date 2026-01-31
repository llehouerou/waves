package layout

import "testing"

func TestContentHeight(t *testing.T) {
	tests := []struct {
		name         string
		windowHeight int
		opts         ContentOpts
		want         int
	}{
		{
			name:         "empty window",
			windowHeight: 40,
			opts:         ContentOpts{HeaderHeight: 1},
			want:         39,
		},
		{
			name:         "with player bar",
			windowHeight: 40,
			opts:         ContentOpts{HeaderHeight: 1, PlayerBarHeight: 3},
			want:         36,
		},
		{
			name:         "with job bar",
			windowHeight: 40,
			opts:         ContentOpts{HeaderHeight: 1, JobBarHeight: 2},
			want:         37,
		},
		{
			name:         "with notifications",
			windowHeight: 40,
			opts:         ContentOpts{HeaderHeight: 1, NotificationCount: 2},
			want:         35, // 40 - 1 - (2 + 2 border)
		},
		{
			name:         "all components",
			windowHeight: 40,
			opts:         ContentOpts{HeaderHeight: 1, PlayerBarHeight: 3, JobBarHeight: 2, NotificationCount: 1},
			want:         31, // 40 - 1 - 3 - 2 - (1 + 2 border)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ContentHeight(tt.windowHeight, tt.opts)
			if got != tt.want {
				t.Errorf("ContentHeight() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestNotificationHeight(t *testing.T) {
	tests := []struct {
		count int
		want  int
	}{
		{0, 0},
		{1, 3}, // 1 + 2 border
		{2, 4}, // 2 + 2 border
		{5, 7}, // 5 + 2 border
	}

	for _, tt := range tests {
		got := NotificationHeight(tt.count)
		if got != tt.want {
			t.Errorf("NotificationHeight(%d) = %d, want %d", tt.count, got, tt.want)
		}
	}
}

func TestNavigatorHeight(t *testing.T) {
	tests := []struct {
		name          string
		contentHeight int
		narrowMode    bool
		queueVisible  bool
		want          int
	}{
		{
			name:          "wide mode queue visible",
			contentHeight: 30,
			narrowMode:    false,
			queueVisible:  true,
			want:          30, // full height in wide mode
		},
		{
			name:          "wide mode queue hidden",
			contentHeight: 30,
			narrowMode:    false,
			queueVisible:  false,
			want:          30,
		},
		{
			name:          "narrow mode queue visible",
			contentHeight: 30,
			narrowMode:    true,
			queueVisible:  true,
			want:          20, // 2/3 of 30
		},
		{
			name:          "narrow mode queue hidden",
			contentHeight: 30,
			narrowMode:    true,
			queueVisible:  false,
			want:          30, // full height when queue hidden
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NavigatorHeight(tt.contentHeight, tt.narrowMode, tt.queueVisible)
			if got != tt.want {
				t.Errorf("NavigatorHeight() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestQueueHeight(t *testing.T) {
	tests := []struct {
		name          string
		contentHeight int
		narrowMode    bool
		queueVisible  bool
		want          int
	}{
		{
			name:          "wide mode",
			contentHeight: 30,
			narrowMode:    false,
			queueVisible:  true,
			want:          30, // same as navigator in wide mode
		},
		{
			name:          "narrow mode queue visible",
			contentHeight: 30,
			narrowMode:    true,
			queueVisible:  true,
			want:          10, // 1/3 of 30 (30 - 20)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := QueueHeight(tt.contentHeight, tt.narrowMode, tt.queueVisible)
			if got != tt.want {
				t.Errorf("QueueHeight() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestIsNarrowMode(t *testing.T) {
	tests := []struct {
		width int
		want  bool
	}{
		{80, true},
		{119, true},
		{120, false},
		{200, false},
	}

	for _, tt := range tests {
		got := IsNarrowMode(tt.width)
		if got != tt.want {
			t.Errorf("IsNarrowMode(%d) = %v, want %v", tt.width, got, tt.want)
		}
	}
}

func TestNavigatorWidth(t *testing.T) {
	tests := []struct {
		name         string
		windowWidth  int
		narrowMode   bool
		queueVisible bool
		want         int
	}{
		{
			name:         "wide mode queue visible",
			windowWidth:  120,
			narrowMode:   false,
			queueVisible: true,
			want:         80, // 2/3 of 120
		},
		{
			name:         "wide mode queue hidden",
			windowWidth:  120,
			narrowMode:   false,
			queueVisible: false,
			want:         120, // full width when queue hidden
		},
		{
			name:         "narrow mode queue visible",
			windowWidth:  100,
			narrowMode:   true,
			queueVisible: true,
			want:         100, // full width in narrow mode (stacked)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NavigatorWidth(tt.windowWidth, tt.narrowMode, tt.queueVisible)
			if got != tt.want {
				t.Errorf("NavigatorWidth() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestQueueWidth(t *testing.T) {
	tests := []struct {
		name         string
		windowWidth  int
		narrowMode   bool
		queueVisible bool
		want         int
	}{
		{
			name:         "wide mode queue visible",
			windowWidth:  120,
			narrowMode:   false,
			queueVisible: true,
			want:         40, // 120 - 80 (navigator width)
		},
		{
			name:         "narrow mode",
			windowWidth:  100,
			narrowMode:   true,
			queueVisible: true,
			want:         100, // full width in narrow mode
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := QueueWidth(tt.windowWidth, tt.narrowMode, tt.queueVisible)
			if got != tt.want {
				t.Errorf("QueueWidth() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestPlayerBarRow(t *testing.T) {
	tests := []struct {
		name              string
		windowHeight      int
		playerBarHeight   int
		jobBarHeight      int
		notificationCount int
		want              int
	}{
		{
			name:            "player stopped",
			windowHeight:    40,
			playerBarHeight: 0,
			want:            0,
		},
		{
			name:            "player only",
			windowHeight:    40,
			playerBarHeight: 3,
			want:            38, // 40 - 3 + 1
		},
		{
			name:              "with job bar and notifications",
			windowHeight:      40,
			playerBarHeight:   3,
			jobBarHeight:      2,
			notificationCount: 1,
			want:              33, // 40 - 3 - 2 - 3 + 1
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PlayerBarRow(tt.windowHeight, tt.playerBarHeight, tt.jobBarHeight, tt.notificationCount)
			if got != tt.want {
				t.Errorf("PlayerBarRow() = %d, want %d", got, tt.want)
			}
		})
	}
}
