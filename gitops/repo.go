package gitops

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"git-builder/config"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

// discardLocalWorktree resets tracked files to HEAD and removes untracked files (git clean -fd),
// so pulls never fail with "worktree contains unstaged changes" on automation hosts.
func discardLocalWorktree(r *git.Repository, w *git.Worktree) error {
	head, err := r.Head()
	if err != nil {
		return fmt.Errorf("head: %w", err)
	}
	if err := w.Reset(&git.ResetOptions{Mode: git.HardReset, Commit: head.Hash()}); err != nil {
		return fmt.Errorf("reset hard: %w", err)
	}
	if err := w.Clean(&git.CleanOptions{Dir: true}); err != nil {
		return fmt.Errorf("clean: %w", err)
	}
	return nil
}

// FullRevision returns the full hex commit hash at HEAD.
func FullRevision(localPath string) (string, error) {
	r, err := git.PlainOpen(localPath)
	if err != nil {
		return "", err
	}
	head, err := r.Head()
	if err != nil {
		return "", err
	}
	return head.Hash().String(), nil
}

// ShortRevision returns a 7-character prefix of the commit at HEAD.
func ShortRevision(localPath string) (string, error) {
	full, err := FullRevision(localPath)
	if err != nil {
		return "", err
	}
	if len(full) >= 7 {
		return full[:7], nil
	}
	return full, nil
}

// Sync clones or pulls the repo. Returns (localPath, updated, err).
// updated is true when the repo was just cloned or pull fetched new commits.
func Sync(c *config.Config, repoURL string) (localPath string, updated bool, err error) {
	dirName := RepoDirName(repoURL)
	localPath = filepath.Join(c.Workdir, dirName)

	var auth transport.AuthMethod
	if strings.HasPrefix(repoURL, "https://") {
		if token := c.GitHubToken(); token != "" {
			auth = &http.BasicAuth{Username: "git", Password: token}
		}
	} else {
		auth, err = ssh.NewPublicKeysFromFile("git", c.SSHKeyPath(), "")
		if err != nil {
			return "", false, fmt.Errorf("ssh key %s: %w", c.SSHKeyPath(), err)
		}
	}

	if err := os.MkdirAll(c.Workdir, 0755); err != nil {
		return "", false, fmt.Errorf("mkdir workdir: %w", err)
	}

	_, err = os.Stat(filepath.Join(localPath, ".git"))
	if err != nil {
		if os.IsNotExist(err) {
			_, err = git.PlainClone(localPath, false, &git.CloneOptions{
				URL:   repoURL,
				Auth:  auth,
				Depth: 1,
			})
			if err != nil {
				return "", false, fmt.Errorf("clone %s: %w", repoURL, err)
			}
			return localPath, true, nil
		}
		return "", false, fmt.Errorf("stat repo dir: %w", err)
	}

	r, err := git.PlainOpen(localPath)
	if err != nil {
		return "", false, fmt.Errorf("open repo: %w", err)
	}
	w, err := r.Worktree()
	if err != nil {
		return "", false, fmt.Errorf("worktree: %w", err)
	}
	if err := discardLocalWorktree(r, w); err != nil {
		return "", false, fmt.Errorf("discard local worktree: %w", err)
	}
	err = w.Pull(&git.PullOptions{Auth: auth, Depth: 1})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return "", false, fmt.Errorf("pull %s: %w", repoURL, err)
	}
	updated = (err != git.NoErrAlreadyUpToDate)
	return localPath, updated, nil
}

// RepoDirName returns the basename used for the clone directory under workdir.
func RepoDirName(url string) string {
	url = strings.TrimSuffix(url, ".git")
	if i := strings.LastIndex(url, "/"); i >= 0 {
		url = url[i+1:]
	}
	if i := strings.LastIndex(url, ":"); i >= 0 {
		url = url[i+1:]
	}
	return url
}

// OverrideScriptBasename returns the basename (no .sh) for a local override script,
// e.g. "Leopere-git-builder" from git@github.com:Leopere/git-builder.git or
// https://github.com/Leopere/git-builder. Used for OWNER-REPO.sh override scripts.
func OverrideScriptBasename(repoURL string) string {
	repoURL = strings.TrimSuffix(repoURL, ".git")
	var path string
	if i := strings.Index(repoURL, "://"); i >= 0 {
		path = repoURL[i+3:]
		if j := strings.Index(path, "/"); j >= 0 {
			path = path[j+1:]
		}
	} else if i := strings.Index(repoURL, ":"); i >= 0 {
		path = repoURL[i+1:]
	} else {
		return RepoDirName(repoURL)
	}
	parts := strings.Split(path, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "-" + parts[len(parts)-1]
	}
	if len(parts) == 1 && parts[0] != "" {
		return parts[0]
	}
	return RepoDirName(repoURL)
}
