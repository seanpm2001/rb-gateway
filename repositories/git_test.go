	"strings"
	"time"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"github.com/reviewboard/rb-gateway/repositories/events"

func TestGitParsePushEvent(t *testing.T) {
	assert := assert.New(t)

	repo, rawRepo := helpers.CreateGitRepo(t, "git-repo")
	defer helpers.CleanupRepository(t, repo.Path)

	oldHead := helpers.SeedGitRepo(t, repo, rawRepo)
	commitIds := make([]plumbing.Hash, 0, 3)

	worktree, err := rawRepo.Worktree()
	assert.Nil(err)
	for i := 1; i <= 3; i++ {
		assert.Nil(err)

		commitId, err := worktree.Commit(fmt.Sprintf("Commit %d", i), &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Author",
				Email: "author@example.com",
				When:  time.Now(),
			},
		})
		assert.Nil(err)

		commitIds = append(commitIds, commitId)
	}

	input := strings.NewReader(fmt.Sprintf("%s %s refs/heads/master\n", oldHead.String(), commitIds[2].String()))

	payload, err := repo.ParseEventPayload(events.PushEvent, input)
	assert.Nil(err)
	expected := events.PushPayload{
		Repository: repo.Name,
		Commits: []events.PushPayloadCommit{
			{
				Id:      commitIds[0].String(),
				Message: "Commit 1",
				Target: events.PushPayloadCommitTarget{
					Branch: "master",
				},
			},
			{
				Id:      commitIds[1].String(),
				Message: "Commit 2",
				Target: events.PushPayloadCommitTarget{
					Branch: "master",
				},
			},
			{
				Id:      commitIds[2].String(),
				Message: "Commit 3",
				Target: events.PushPayloadCommitTarget{
					Branch: "master",
				},
			},
		},
	}
	assert.Equal(expected, payload)
}

func TestGitParsePushEventNewBranch(t *testing.T) {
	assert := assert.New(t)

	repo, rawRepo := helpers.CreateGitRepo(t, "git-repo")
	defer helpers.CleanupRepository(t, repo.Path)

	helpers.SeedGitRepo(t, repo, rawRepo)

	commitIds := make([]plumbing.Hash, 0, 2)
	worktree, err := rawRepo.Worktree()
	worktree.Checkout(&git.CheckoutOptions{
		Branch: "refs/heads/dev",
		Create: true,
	})

	for i := 1; i <= 2; i++ {
		commitId, err := worktree.Commit(fmt.Sprintf("Commit %d", i), &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Author",
				Email: "author@example.com",
				When:  time.Now(),
			},
		})
		assert.Nil(err)

		commitIds = append(commitIds, commitId)
	}

	input := strings.NewReader(fmt.Sprintf("%040d %s refs/heads/dev\n", 0, commitIds[1].String()))

	payload, err := repo.ParseEventPayload(events.PushEvent, input)
	assert.Nil(err)
	expected := events.PushPayload{
		Repository: repo.Name,
		Commits: []events.PushPayloadCommit{
			{
				Id:      commitIds[0].String(),
				Message: "Commit 1",
				Target: events.PushPayloadCommitTarget{
					Branch: "dev",
				},
			},
			{
				Id:      commitIds[1].String(),
				Message: "Commit 2",
				Target: events.PushPayloadCommitTarget{
					Branch: "dev",
				},
			},
		},
	}

	assert.Equal(expected, payload)

}

// This test models a force push to a repository.
//
// It creates the following branch struture:
//
// Before force push:
// o -- A -- o -- B
//
// New:
// o -- A -- o -- B' (original B)
//       \
//        -- C -- B
//
// The payload should contain the commits C and B.
func TestGitParsePushEventRebase(t *testing.T) {
	assert := assert.New(t)

	repo, rawRepo := helpers.CreateGitRepo(t, "git-repo")
	defer helpers.CleanupRepository(t, repo.Path)

	mergeBase := helpers.SeedGitRepo(t, repo, rawRepo)

	worktree, err := rawRepo.Worktree()
	assert.Nil(err)

	var oldHead plumbing.Hash
	for i := 0; i < 2; i++ {
		oldHead, err = worktree.Commit(fmt.Sprintf("Commit %d", i+1), &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Author",
				Email: "author@example.com",
				When:  time.Now(),
			},
		})
		assert.Nil(err)
	}

	err = worktree.Checkout(&git.CheckoutOptions{
		Hash:   mergeBase,
		Branch: "refs/heads/dev",
		Create: true,
	})
	assert.Nil(err)

	commitIds := make([]plumbing.Hash, 0, 2)
	for i := 1; i <= 2; i++ {
		commitId, err := worktree.Commit(fmt.Sprintf("New Commit %d", i), &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Author",
				Email: "author@example.com",
				When:  time.Now(),
			},
		})
		assert.Nil(err)

		commitIds = append(commitIds, commitId)
	}

	input := strings.NewReader(fmt.Sprintf("%s %s refs/heads/dev\n", oldHead.String(), commitIds[1].String()))

	payload, err := repo.ParseEventPayload(events.PushEvent, input)
	assert.Nil(err)
	expected := events.PushPayload{
		Repository: repo.Name,
		Commits: []events.PushPayloadCommit{
			{
				Id:      commitIds[0].String(),
				Message: "New Commit 1",
				Target: events.PushPayloadCommitTarget{
					Branch: "dev",
				},
			},
			{
				Id:      commitIds[1].String(),
				Message: "New Commit 2",
				Target: events.PushPayloadCommitTarget{
					Branch: "dev",
				},
			},
		},
	}

	assert.Equal(expected, payload)
}

func TestGitParsePushEventMultiple(t *testing.T) {
	assert := assert.New(t)

	repo, rawRepo := helpers.CreateGitRepo(t, "git-repo")
	defer helpers.CleanupRepository(t, repo.Path)

	helpers.SeedGitRepo(t, repo, rawRepo)
	worktree, err := rawRepo.Worktree()
	assert.Nil(err)

	err = worktree.Checkout(&git.CheckoutOptions{
		Branch: "refs/heads/branch-1",
		Create: true,
	})

	commitIds := make([]plumbing.Hash, 0, 4)

	for i := 1; i <= 2; i++ {
		commitId, err := worktree.Commit(fmt.Sprintf("Commit %d", i), &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Author",
				Email: "author@example.com",
				When:  time.Now(),
			},
		})
		assert.Nil(err)

		commitIds = append(commitIds, commitId)
	}

	err = worktree.Checkout(&git.CheckoutOptions{
		Branch: "refs/heads/branch-2",
		Create: true,
	})

	for i := 3; i <= 4; i++ {
		commitId, err := worktree.Commit(fmt.Sprintf("Commit %d", i), &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Author",
				Email: "author@example.com",
				When:  time.Now(),
			},
		})
		assert.Nil(err)

		commitIds = append(commitIds, commitId)
	}

	input := strings.NewReader(fmt.Sprintf(
		"%040d %s refs/heads/branch-1\n%040d %s refs/heads/branch-2\n",
		0, commitIds[1].String(),
		0, commitIds[3].String(),
	))

	payload, err := repo.ParseEventPayload(events.PushEvent, input)
	assert.Nil(err)

	expected := events.PushPayload{
		Repository: repo.Name,
		Commits: []events.PushPayloadCommit{
			{
				Id:      commitIds[0].String(),
				Message: "Commit 1",
				Target: events.PushPayloadCommitTarget{
					Branch: "branch-1",
				},
			},
			{
				Id:      commitIds[1].String(),
				Message: "Commit 2",
				Target: events.PushPayloadCommitTarget{
					Branch: "branch-1",
				},
			},
			{
				Id:      commitIds[2].String(),
				Message: "Commit 3",
				Target: events.PushPayloadCommitTarget{
					Branch: "branch-2",
				},
			},
			{
				Id:      commitIds[3].String(),
				Message: "Commit 4",
				Target: events.PushPayloadCommitTarget{
					Branch: "branch-2",
				},
			},
		},
	}

	assert.Equal(expected, payload)
}