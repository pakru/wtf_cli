package commands

import (
	"sync"
	"testing"
)

func TestPathGrants_AllowAndIsAllowed(t *testing.T) {
	g := NewPathGrants()
	if g.IsAllowed("read_file", "/var/log") {
		t.Fatal("freshly created store should not allow anything")
	}
	g.Allow("read_file", "/var/log")
	if !g.IsAllowed("read_file", "/var/log") {
		t.Fatal("expected the exact granted directory to be allowed")
	}
	if !g.IsAllowed("read_file", "/var/log/nginx/error.log") {
		t.Fatal("expected a path under the granted directory to be allowed")
	}
}

// Regression: a naive string-prefix check would wrongly accept a sibling
// directory whose name happens to start with the granted directory's name.
func TestPathGrants_BoundaryNonMatch(t *testing.T) {
	g := NewPathGrants()
	g.Allow("read_file", "/var/log")

	if g.IsAllowed("read_file", "/var/log2") {
		t.Fatal("a sibling directory with a prefixed name must not be allowed")
	}
	if g.IsAllowed("read_file", "/var/log2/x") {
		t.Fatal("a path under a sibling directory with a prefixed name must not be allowed")
	}
}

func TestPathGrants_RootGrant(t *testing.T) {
	g := NewPathGrants()
	g.Allow("read_file", "/")
	if !g.IsAllowed("read_file", "/etc/hosts") {
		t.Fatal("a grant for / should cover any path")
	}
	if !g.IsAllowed("read_file", "/") {
		t.Fatal("a grant for / should cover / itself")
	}
}

// TestPathGrants_CrossToolIsolation is the core security property: directory
// listing and file-content reading are different capabilities, so a grant
// for one tool must never satisfy a check for another.
func TestPathGrants_CrossToolIsolation(t *testing.T) {
	g := NewPathGrants()
	g.Allow("list_directory", "/home/user")

	if g.IsAllowed("read_file", "/home/user/.ssh/id_rsa") {
		t.Fatal("a list_directory grant must never satisfy a read_file check")
	}
	if !g.IsAllowed("list_directory", "/home/user/.ssh") {
		t.Fatal("the granted tool should still be allowed under its own grant")
	}
}

func TestPathGrants_RejectsRelativeDir(t *testing.T) {
	g := NewPathGrants()
	g.Allow("read_file", "relative/dir")
	if g.IsAllowed("read_file", "relative/dir") {
		t.Fatal("a non-absolute directory must be rejected, not stored")
	}
}

func TestPathGrants_RejectsUncleanDir(t *testing.T) {
	g := NewPathGrants()
	g.Allow("read_file", "/var/log/../log")
	if g.IsAllowed("read_file", "/var/log") {
		t.Fatal("a non-clean directory must be rejected, not stored")
	}
}

func TestPathGrants_NilSafe(t *testing.T) {
	var g *PathGrants
	if g.IsAllowed("read_file", "/etc") {
		t.Fatal("nil store should report not-allowed")
	}
	g.Allow("read_file", "/etc") // must not panic
}

func TestPathGrants_ConcurrentAccess(t *testing.T) {
	g := NewPathGrants()
	var wg sync.WaitGroup
	tools := []string{"read_file", "list_directory", "future_tool"}
	for _, name := range tools {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			g.Allow(name, "/tmp/"+name)
		}(name)
	}
	wg.Wait()

	for _, name := range tools {
		if !g.IsAllowed(name, "/tmp/"+name+"/x") {
			t.Fatalf("expected %q to be allowed under its own grant", name)
		}
	}
}
