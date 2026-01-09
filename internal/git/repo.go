package git

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// InitBare initializes a new bare git repository, creates an initial empty commit, and replenishes physical contracts.
func InitBare(repoPath string) (string, error) {
	// 1. Initialize bare repository using go-git
	r, err := git.PlainInit(repoPath, true)
	if err != nil {
		return "", fmt.Errorf("failed to init bare repo: %w", err)
	}

	// --- Create an initial empty commit to prevent clone errors ---
	storer := r.Storer

	// Create an empty tree
	emptyTree := &object.Tree{}
	obj := storer.NewEncodedObject()
	if err := emptyTree.Encode(obj); err != nil {
		return "", fmt.Errorf("failed to encode empty tree: %w", err)
	}
	treeHash, err := storer.SetEncodedObject(obj)
	if err != nil {
		return "", fmt.Errorf("failed to save empty tree: %w", err)
	}

	// Create the initial commit
	initialCommit := &object.Commit{
		Author: object.Signature{
			Name:  "Potstack Initializer",
			Email: "init@potstack.local",
			When:  time.Now(),
		},
		Message:  "Initial commit",
		TreeHash: treeHash,
	}
	obj = storer.NewEncodedObject()
	if err := initialCommit.Encode(obj); err != nil {
		return "", fmt.Errorf("failed to encode initial commit: %w", err)
	}
	commitHash, err := storer.SetEncodedObject(obj)
	if err != nil {
		return "", fmt.Errorf("failed to save initial commit: %w", err)
	}

	// Create the main branch pointing to the initial commit
	mainRef := plumbing.NewReferenceFromStrings("refs/heads/main", commitHash.String())
	if err := storer.SetReference(mainRef); err != nil {
		return "", fmt.Errorf("failed to create main branch: %w", err)
	}

	// Update HEAD to point to the new main branch
	headRef := plumbing.NewSymbolicReference(plumbing.HEAD, mainRef.Name())
	if err := storer.SetReference(headRef); err != nil {
		return "", fmt.Errorf("failed to update HEAD: %w", err)
	}
	// --- End of initial commit logic ---

	// 2. Replenish physical contracts (UUID)
	uuid, err := generateUUID()
	if err != nil {
		return "", fmt.Errorf("failed to generate uuid: %w", err)
	}

	if err := os.WriteFile(filepath.Join(repoPath, "uuid"), []byte(uuid), 0644); err != nil {
		return "", fmt.Errorf("failed to write uuid file: %w", err)
	}

	// 3. Replenish physical contracts (Description)
	if err := os.WriteFile(filepath.Join(repoPath, "description"), []byte("Unnamed repository"), 0644); err != nil {
		return "", fmt.Errorf("failed to write description file: %w", err)
	}

	return uuid, nil
}

func generateUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
