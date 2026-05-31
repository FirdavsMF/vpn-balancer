package downloader

import (
	"os"
	"testing"
)

func TestFetchFromFile(t *testing.T) {
	content := `vless://uuid1@server1.com:443#Server1
# This is a comment
vless://uuid2@server2.com:8443?security=tls#Server2

vless://uuid3@server3.com:8080`

	tmpfile, err := os.CreateTemp("", "test-vless-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.WriteString(content); err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()

	lines, err := FetchFromFile(tmpfile.Name())
	if err != nil {
		t.Fatalf("FetchFromFile failed: %v", err)
	}

	expectedCount := 3
	if len(lines) != expectedCount {
		t.Errorf("Expected %d lines, got %d", expectedCount, len(lines))
	}

	if lines[0] != "vless://uuid1@server1.com:443#Server1" {
		t.Errorf("Unexpected first line: %s", lines[0])
	}
}

func TestFetchFromFileComments(t *testing.T) {
	content := `# Comment 1
# Comment 2
# Comment with tab`

	tmpfile, err := os.CreateTemp("", "test-comments-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.WriteString(content); err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()

	lines, err := FetchFromFile(tmpfile.Name())
	if err != nil {
		t.Fatalf("FetchFromFile failed: %v", err)
	}

	if len(lines) != 0 {
		t.Errorf("Expected 0 lines, got %d", len(lines))
	}
}

func TestFetchFromFileEmpty(t *testing.T) {
	content := ""

	tmpfile, err := os.CreateTemp("", "test-empty-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.WriteString(content); err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()

	lines, err := FetchFromFile(tmpfile.Name())
	if err != nil {
		t.Fatalf("FetchFromFile failed: %v", err)
	}

	if len(lines) != 0 {
		t.Errorf("Expected 0 lines, got %d", len(lines))
	}
}

func TestFetchFromFileMixedContent(t *testing.T) {
	content := `# Header comment
vless://uuid1@server1.com:443#Server1

# Middle comment

vless://uuid2@server2.com:8443#Server2
# Footer comment`

	tmpfile, err := os.CreateTemp("", "test-mixed-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.WriteString(content); err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()

	lines, err := FetchFromFile(tmpfile.Name())
	if err != nil {
		t.Fatalf("FetchFromFile failed: %v", err)
	}

	if len(lines) != 2 {
		t.Errorf("Expected 2 lines, got %d", len(lines))
	}
}
