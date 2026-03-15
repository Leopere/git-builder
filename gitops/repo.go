package gitops

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"git-builder/config"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

func Sync(c *config.Config, repoURL string) (localPath string, err error) {
	dirName := repoDirName(repoURL)
	localPath = filepath.Join(c.Workdir, dirName)

	auth, err := ssh.NewPublicKeysFromFile("git", c.SSHKeyPath(), "")
	if err != nil {
		return "", fmt.Errorf("ssh key %s: %w", c.SSHKeyPath(), err)
	}

	if err := os.MkdirAll(c.Workdir, 0755); err != nil {
		return "", fmt.Errorf("mkdir workdir: %w", err)
	}

	_, err = os.Stat(filepath.Join(localPath, ".git"))
	if err != nil {
		if os.IsNotExist(err) {
			_, err = git.PlainClone(localPath, false, &git.CloneOptions{
				URL:  repoURL,
				Auth: auth,
			})
			if err != nil {
				return "", fmt.Errorf("clone %s: %w", repoURL, err)
			}
			return localPath, nil
		}
		return "", fmt.Errorf("stat repo dir: %w", err)
	}

	r, err := git.PlainOpen(localPath)
	if err != nil {
		return "", fmt.Errorf("open repo: %w", err)
	}
	w, err := r.Worktree()
	if err != nil {
		return "", fmt.Errorf("worktree: %w", err)
	}
	err = w.Pull(&git.PullOptions{Auth: auth})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return "", fmt.Errorf("pull %s: %w", repoURL, err)
	}
	return localPath, nil
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
