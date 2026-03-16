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

// Sync clones or pulls the repo. Returns (localPath, updated, err).
// updated is true when the repo was just cloned or pull fetched new commits.
func Sync(c *config.Config, repoURL string) (localPath string, updated bool, err error) {
	dirName := repoDirName(repoURL)
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
	err = w.Pull(&git.PullOptions{Auth: auth, Depth: 1})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return "", false, fmt.Errorf("pull %s: %w", repoURL, err)
	}
	updated = (err != git.NoErrAlreadyUpToDate)
	return localPath, updated, nil
}

func repoDirName(url string) string {
	url = strings.TrimSuffix(url, ".git")
	if i := strings.LastIndex(url, "/"); i >= 0 {
		url = url[i+1:]
	}
	if i := strings.LastIndex(url, ":"); i >= 0 {
		url = url[i+1:]
	}
	return url
}
