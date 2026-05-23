package ui

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"wtf_cli/pkg/config"
	"wtf_cli/pkg/ui/components/welcome"
	"wtf_cli/pkg/updatecheck"
	"wtf_cli/pkg/version"

	tea "charm.land/bubbletea/v2"
)

// tickDirectory creates a command that periodically updates directory
func tickDirectory() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return directoryUpdateMsg{}
	})
}

type directoryUpdateMsg struct{}

type gitBranchMsg struct {
	dir    string
	branch string
}

type updateCheckMsg struct {
	Result     updatecheck.Result
	Err        error
	SkipReason string
}

type exitConfirmTimeoutMsg struct {
	id int
}

type clearStatusMsgMsg struct{}

func (m Model) handleCtrlDPressed() (Model, tea.Cmd) {
	if m.exitPending {
		m.exitPending = false
		m.statusBar.SetMessage("")
		if m.inputHandler != nil {
			if err := m.inputHandler.SendToPTY([]byte{4}); err != nil {
				slog.Error("exit_send_eof_error", "error", err)
			}
		}
		return m, tea.Quit
	}
	m.exitPending = true
	m.exitConfirmID++
	confirmID := m.exitConfirmID
	m.statusBar.SetMessage("Press Ctrl+D again to exit")
	return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg {
		return exitConfirmTimeoutMsg{id: confirmID}
	})
}

func (m Model) handleExitConfirmTimeout(msg exitConfirmTimeoutMsg) (Model, tea.Cmd) {
	if m.exitPending && msg.id == m.exitConfirmID {
		m.exitPending = false
		m.statusBar.SetMessage("")
	}
	return m, nil
}

func (m Model) handleClearStatusMsg() (Model, tea.Cmd) {
	if m.statusBar != nil && m.statusBar.GetMessage() == selectedTextCopiedMessage {
		m.statusBar.SetMessage("")
	}
	return m, nil
}

func (m Model) handleDirectoryUpdate() (Model, tea.Cmd) {
	// Update current directory from shell process
	if m.cwdFunc != nil {
		if cwd, err := m.cwdFunc(); err == nil {
			m.currentDir = cwd
		}
	}
	// Always resolve git branch on every tick — the resolver is cheap
	// (reads .git/HEAD) and this ensures branch changes from commands
	// like `git checkout` are reflected promptly.
	branchCmd := resolveGitBranchCmd(m.currentDir, m.gitBranchResolver)
	// Schedule next update
	return m, tea.Batch(tickDirectory(), branchCmd)
}

func (m Model) handleGitBranch(msg gitBranchMsg) (Model, tea.Cmd) {
	if msg.dir == m.currentDir {
		m.gitBranch = msg.branch
	}
	return m, nil
}

func (m Model) handleUpdateCheck(msg updateCheckMsg) (Model, tea.Cmd) {
	if msg.SkipReason != "" {
		slog.Info("update_check_skipped", "reason", msg.SkipReason)
		return m, nil
	}
	if msg.Err != nil {
		slog.Warn("update_check_error", "error", msg.Err)
		return m, nil
	}
	if !msg.Result.UpdateAvailable || m.startupUpdateShown {
		return m, nil
	}

	notice := &welcome.UpdateNotice{
		CurrentVersion: msg.Result.CurrentVersion,
		LatestVersion:  msg.Result.LatestVersion,
		ReleaseURL:     msg.Result.ReleaseURL,
		UpgradeCommand: msg.Result.UpgradeCommand,
	}
	m.viewport.AppendOutput([]byte(welcome.UpdateBanner(notice)))
	m.startupUpdateShown = true
	slog.Info("update_check_success", "current", msg.Result.CurrentVersion, "latest", msg.Result.LatestVersion, "update_available", true)
	return m, nil
}

func resolveGitBranchCmd(dir string, resolver func(string) string) tea.Cmd {
	trimmed := strings.TrimSpace(dir)
	if trimmed == "" || resolver == nil {
		return nil
	}
	return func() tea.Msg {
		return gitBranchMsg{
			dir:    trimmed,
			branch: resolver(trimmed),
		}
	}
}

func fetchUpdateCheckCmd() tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.Load(config.GetConfigPath())
		if err != nil {
			return updateCheckMsg{SkipReason: "config_error"}
		}
		if !cfg.UpdateCheck.Enabled {
			return updateCheckMsg{SkipReason: "disabled"}
		}

		current := strings.TrimSpace(version.Version)
		if current == "" || strings.EqualFold(current, "dev") {
			return updateCheckMsg{SkipReason: "dev_build"}
		}

		slog.Info("update_check_start")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		result, err := updatecheck.CheckLatest(ctx, current, updatecheck.CheckOptions{
			Interval: time.Duration(cfg.UpdateCheck.IntervalHours) * time.Hour,
		})
		if err != nil {
			return updateCheckMsg{Err: err}
		}

		if !result.UpdateAvailable {
			slog.Info("update_check_success", "current", result.CurrentVersion, "latest", result.LatestVersion, "update_available", false)
		}

		return updateCheckMsg{Result: result}
	}
}
