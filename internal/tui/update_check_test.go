package tui

import (
	"context"
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestUpdateResultUsesTheInformationalNotice(t *testing.T) {
	m := modelForRange(t, nil, 0)
	next, cmd := m.Update(updateAvailableMsg{version: "v0.1.3"})
	if cmd != nil {
		t.Fatal("update result scheduled repository work")
	}
	got := next.(*Model).state
	if got.Notice != "update available: v0.1.3" || got.NoticeFailed {
		t.Fatalf("state = %+v, want informational notice", got)
	}
}

func TestUpdateResultPreservesExistingNotice(t *testing.T) {
	for _, tt := range []struct {
		name   string
		notice string
		failed bool
	}{
		{name: "operation", notice: "staged 1 file"},
		{name: "error", notice: "git refused", failed: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := modelForRange(t, nil, 0)
			m.state.Notice, m.state.NoticeFailed = tt.notice, tt.failed
			next, _ := m.Update(updateAvailableMsg{version: "v0.1.3"})
			got := next.(*Model).state
			if got.Notice != tt.notice || got.NoticeFailed != tt.failed {
				t.Fatalf("state = %+v, want notice %q with failed=%v", got, tt.notice, tt.failed)
			}
		})
	}
}

func TestUpdateCheckCommandReturnsReleaseMessage(t *testing.T) {
	m := modelForRange(t, nil, 0)
	m.updateCheck = func(context.Context) (string, error) {
		return "v0.1.3", nil
	}

	msg := m.checkForUpdate()()
	got, ok := msg.(updateAvailableMsg)
	if !ok {
		t.Fatalf("message = %T, want updateAvailableMsg", msg)
	}
	if got.version != "v0.1.3" {
		t.Fatalf("version = %q, want v0.1.3", got.version)
	}
}

func TestUpdateCheckErrorIsSilent(t *testing.T) {
	m := modelForRange(t, nil, 0)
	m.state.Notice = "staged 1 file"
	m.updateCheck = func(context.Context) (string, error) {
		return "", errors.New("release service unavailable")
	}

	msg := m.checkForUpdate()()
	next, _ := m.Update(msg)
	got := next.(*Model).state
	if got.Notice != "staged 1 file" || got.NoticeFailed {
		t.Fatalf("state = %+v, want the existing informational notice", got)
	}
}

func TestDisabledUpdateCheckOmitsTheChecker(t *testing.T) {
	t.Setenv("MIRUGIT_NO_UPDATE_CHECK", "")
	m, err := NewWithVersion(context.Background(), testRepo(t), t.TempDir(), "v0.1.2")
	if err != nil {
		t.Fatal(err)
	}
	if m.updateCheck != nil {
		t.Fatal("update checker configured while MIRUGIT_NO_UPDATE_CHECK is present")
	}
}

func TestInitSchedulesUpdateCheck(t *testing.T) {
	m := modelForRange(t, nil, 0)
	m.updateCheck = func(context.Context) (string, error) {
		return "v0.1.3", nil
	}

	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init returned no commands")
	}
	batch, ok := cmd().(tea.BatchMsg)
	if !ok {
		t.Fatalf("Init command = %T, want tea.BatchMsg", cmd())
	}
	for _, candidate := range batch {
		if candidate == nil {
			continue
		}
		if msg, ok := candidate().(updateAvailableMsg); ok {
			if msg.version != "v0.1.3" {
				t.Fatalf("version = %q, want v0.1.3", msg.version)
			}
			return
		}
	}
	t.Fatal("Init did not schedule an update check")
}
