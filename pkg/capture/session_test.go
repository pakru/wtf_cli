package capture

import (
	"sync"
	"testing"
	"time"
)

func TestNewSessionContext(t *testing.T) {
	sc := NewSessionContext()

	if sc == nil {
		t.Fatal("NewSessionContext() returned nil")
	}

	if sc.HistorySize() != 0 {
		t.Errorf("Expected empty history, got %d", sc.HistorySize())
	}

	if sc.GetCurrentDir() == "" {
		t.Error("Expected default directory, got empty string")
	}
}

func TestAddCommand(t *testing.T) {
	sc := NewSessionContext()

	record := CommandRecord{
		Command:     "ls -la",
		ExitCode:    0,
		StartTime:   time.Now(),
		EndTime:     time.Now(),
		WorkingDir:  "/home/user",
		BufferStart: 0,
		BufferEnd:   10,
	}

	sc.AddCommand(record)

	if sc.HistorySize() != 1 {
		t.Errorf("Expected 1 command in history, got %d", sc.HistorySize())
	}

	history := sc.GetHistory()
	if history[0].Command != "ls -la" {
		t.Errorf("Expected command 'ls -la', got %q", history[0].Command)
	}
}

func TestGetHistory(t *testing.T) {
	sc := NewSessionContext()

	commands := []string{"pwd", "ls", "cd /tmp"}
	for i, cmd := range commands {
		sc.AddCommand(CommandRecord{
			Command:    cmd,
			ExitCode:   0,
			StartTime:  time.Now(),
			EndTime:    time.Now(),
			WorkingDir: "/home",
			BufferStart: i * 10,
			BufferEnd:   (i + 1) * 10,
		})
	}

	history := sc.GetHistory()
	if len(history) != 3 {
		t.Fatalf("Expected 3 commands, got %d", len(history))
	}

	for i, cmd := range commands {
		if history[i].Command != cmd {
			t.Errorf("Command %d: expected %q, got %q", i, cmd, history[i].Command)
		}
	}
}

func TestGetLastN(t *testing.T) {
	sc := NewSessionContext()

	// Add 5 commands
	for i := 1; i <= 5; i++ {
		sc.AddCommand(CommandRecord{
			Command:   string(rune('0' + i)),
			ExitCode:  0,
			StartTime: time.Now(),
			EndTime:   time.Now(),
		})
	}

	// Get last 3
	last3 := sc.GetLastN(3)
	if len(last3) != 3 {
		t.Fatalf("Expected 3 commands, got %d", len(last3))
	}

	expected := []string{"3", "4", "5"}
	for i, exp := range expected {
		if last3[i].Command != exp {
			t.Errorf("Command %d: expected %q, got %q", i, exp, last3[i].Command)
		}
	}
}

func TestGetLastN_MoreThanAvailable(t *testing.T) {
	sc := NewSessionContext()

	sc.AddCommand(CommandRecord{Command: "cmd1"})
	sc.AddCommand(CommandRecord{Command: "cmd2"})

	last10 := sc.GetLastN(10)
	if len(last10) != 2 {
		t.Errorf("Expected 2 commands, got %d", len(last10))
	}
}

func TestDirectoryTracking(t *testing.T) {
	sc := NewSessionContext()

	sc.AddCommand(CommandRecord{
		Command:    "cd /home/user",
		WorkingDir: "/home/user",
	})

	if sc.GetCurrentDir() != "/home/user" {
		t.Errorf("Expected current dir '/home/user', got %q", sc.GetCurrentDir())
	}

	sc.AddCommand(CommandRecord{
		Command:    "cd /tmp",
		WorkingDir: "/tmp",
	})

	if sc.GetCurrentDir() != "/tmp" {
		t.Errorf("Expected current dir '/tmp', got %q", sc.GetCurrentDir())
	}
}

func TestSetCurrentDir(t *testing.T) {
	sc := NewSessionContext()

	sc.SetCurrentDir("/opt")

	if sc.GetCurrentDir() != "/opt" {
		t.Errorf("Expected '/opt', got %q", sc.GetCurrentDir())
	}
}

func TestBufferAssociation(t *testing.T) {
	sc := NewSessionContext()

	record := CommandRecord{
		Command:     "echo hello",
		BufferStart: 100,
		BufferEnd:   150,
	}

	sc.AddCommand(record)

	history := sc.GetHistory()
	if history[0].BufferStart != 100 {
		t.Errorf("Expected BufferStart 100, got %d", history[0].BufferStart)
	}

	if history[0].BufferEnd != 150 {
		t.Errorf("Expected BufferEnd 150, got %d", history[0].BufferEnd)
	}
}

func TestHistoryBounding(t *testing.T) {
	sc := NewSessionContext()
	sc.maxHistory = 10 // Set small limit for testing

	// Add 15 commands
	for i := 1; i <= 15; i++ {
		sc.AddCommand(CommandRecord{
			Command: string(rune('A' + i - 1)),
		})
	}

	// Should only keep last 10
	if sc.HistorySize() != 10 {
		t.Errorf("Expected 10 commands (max), got %d", sc.HistorySize())
	}

	// Should have commands F-O (6-15)
	history := sc.GetHistory()
	if history[0].Command != "F" {
		t.Errorf("Expected first command 'F', got %q", history[0].Command)
	}

	if history[9].Command != "O" {
		t.Errorf("Expected last command 'O', got %q", history[9].Command)
	}
}

func TestConcurrency(t *testing.T) {
	sc := NewSessionContext()

	var wg sync.WaitGroup
	writers := 10
	commandsPerWriter := 100

	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < commandsPerWriter; j++ {
				sc.AddCommand(CommandRecord{
					Command: string(rune('A' + id)),
				})
			}
		}(i)
	}

	wg.Wait()

	// Should have exactly 1000 commands (maxHistory)
	if sc.HistorySize() != 1000 {
		t.Errorf("Expected 1000 commands, got %d", sc.HistorySize())
	}
}

func TestClear(t *testing.T) {
	sc := NewSessionContext()

	sc.AddCommand(CommandRecord{Command: "test1"})
	sc.AddCommand(CommandRecord{Command: "test2"})

	sc.Clear()

	if sc.HistorySize() != 0 {
		t.Errorf("Expected empty history after Clear(), got %d", sc.HistorySize())
	}
}

func TestSessionDuration(t *testing.T) {
	sc := NewSessionContext()

	time.Sleep(10 * time.Millisecond)

	duration := sc.GetSessionDuration()
	if duration < 10*time.Millisecond {
		t.Errorf("Expected duration >= 10ms, got %v", duration)
	}
}
