package state

import "testing"

func TestUpdateAvailableSetsAnInformationalNotice(t *testing.T) {
	tests := []struct {
		name       string
		state      State
		wantNotice string
		wantFailed bool
	}{
		{
			name:       "empty notice",
			wantNotice: "update available: v0.1.3",
		},
		{
			name:       "existing success notice",
			state:      State{Notice: "existing notice"},
			wantNotice: "existing notice",
		},
		{
			name:       "existing failure notice",
			state:      State{Notice: "existing failure", NoticeFailed: true},
			wantNotice: "existing failure",
			wantFailed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Apply(tt.state, UpdateAvailable{Version: "v0.1.3"})
			if got.Notice != tt.wantNotice {
				t.Fatalf("Notice = %q, want %q", got.Notice, tt.wantNotice)
			}
			if got.NoticeFailed != tt.wantFailed {
				t.Fatalf("NoticeFailed = %v, want %v", got.NoticeFailed, tt.wantFailed)
			}
		})
	}
}
