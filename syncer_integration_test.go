package filesyncer

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"golang.org/x/sync/errgroup"
	"net"
	"os"
	"path/filepath"
	"testing"
)

// Tests communications between Main and Replica works as expected
// Replica folder should exactly match main folder at end of the test
func TestSyncerEndToEnd(t *testing.T) {
	// Setup test scenario with main and replica folder
	mainDir := t.TempDir()
	replicaDir := t.TempDir()

	mainFiles := map[string]string{
		"a.md": "# A recipe\n",
		"b.md": "# Recipe B\n",
		"c.md": "# Recipe C\n",
	}
	replicaFiles := map[string]string{
		// a.md missing, should be added
		"b.md": "# Recipe B\n",         // Same as main, should be unchanged
		"c.md": "# Different Header\n", // Different from main, should be replaced
		"d.md": "# Recipe D\n",         // Not in main, should be deleted
	}

	for name, content := range mainFiles {
		err := os.WriteFile(filepath.Join(mainDir, name), []byte(content), 0644)
		assert.Equal(t, nil, err, "Write not error")
	}
	for name, content := range replicaFiles {
		err := os.WriteFile(filepath.Join(replicaDir, name), []byte(content), 0644)
		assert.Equal(t, nil, err, "Write not error")
	}

	// In-memory connection to simulate tcp
	mainConn, replicaConn := net.Pipe()

	// Run syncers
	mainFC, err := CreateFileCache(mainDir)
	assert.Equal(t, nil, err, "Failed to create main file cache")
	mainSyncer := Syncer{Replica: false, Conn: mainConn, FileCache: mainFC}

	replicaFC, err := CreateFileCache(replicaDir)
	assert.Equal(t, nil, err, "Failed to create replica file cache")
	replicaSyncer := Syncer{Replica: true, Conn: replicaConn, FileCache: replicaFC}

	g := new(errgroup.Group)
	g.Go(func() error {
		err := replicaSyncer.RunAsReplica()
		return err
	})
	g.Go(func() error {
		err := mainSyncer.RunAsMain()
		return err
	})
	if err := g.Wait(); err != nil {
		t.Fatal("Syncer failed")
	}

	// TODO: Assert folders match
	replicaFcPostSync, err := CreateFileCache(replicaDir)
	assert.Equal(t, nil, err, "Failed to create replica file cache after sync")

	mainFileCount := 0
	for k, v := range mainFC.data {
		replicaFileData, ok := replicaFcPostSync.data[k]
		assert.Equal(t, ok, true, fmt.Sprintf("File %s missing from replica folder", k))
		assert.Equal(t, replicaFileData.md5, v.md5, fmt.Sprintf("File %s MD5s do not match between replica and main", k))
		mainFileCount++
	}

	assert.Equal(t, mainFileCount, len(replicaFcPostSync.data), "Total number of files in main folder should match synced folder")
}
