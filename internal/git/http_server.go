package git

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"potstack/config"

	"github.com/gin-gonic/gin"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/format/packfile"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp/sideband"
)

// -------------------- HTTP Entry --------------------

func SmartHTTPServer() gin.HandlerFunc {
	return func(c *gin.Context) {
		owner := c.Param("owner")
		reponame := c.Param("reponame")
		action := c.Param("action")

		if !strings.HasSuffix(reponame, ".git") {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		repoPath := filepath.Join(config.RepoRoot, owner, reponame)
		if _, err := os.Stat(repoPath); err != nil {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		switch {
		case strings.HasSuffix(action, "/info/refs"):
			handleInfoRefs(c, repoPath)
		case strings.HasSuffix(action, "/git-upload-pack"):
			handleService(c, repoPath, "upload-pack")
		case strings.HasSuffix(action, "/git-receive-pack"):
			handleService(c, repoPath, "receive-pack")
		default:
			c.AbortWithStatus(http.StatusNotFound)
		}
	}
}

// -------------------- info/refs --------------------

func handleInfoRefs(c *gin.Context, repoPath string) {
	service := c.Query("service")
	if service == "" {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	abs, _ := filepath.Abs(repoPath)
	repo, err := git.PlainOpen(abs)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.Header("Content-Type", fmt.Sprintf("application/x-%s-advertisement", service))
	c.Header("Cache-Control", "no-cache")
	c.Status(http.StatusOK)

	pkt := func(s string) string {
		if s == "" {
			return "0000"
		}
		return fmt.Sprintf("%04x%s", len(s)+4, s)
	}

	c.Writer.WriteString(pkt(fmt.Sprintf("# service=%s\n", service)))
	c.Writer.WriteString("0000")

	refs, _ := repo.References()
	var refList []string
	refs.ForEach(func(r *plumbing.Reference) error {
		if r.Type() == plumbing.HashReference {
			refList = append(refList, fmt.Sprintf("%s %s", r.Hash(), r.Name()))
		}
		return nil
	})

	caps := "side-band-64k ofs-delta object-format=sha1 agent=go-git"

	if len(refList) == 0 {
		c.Writer.WriteString(pkt(fmt.Sprintf("%040d\x00%s\n", 0, caps)))
	} else {
		c.Writer.WriteString(pkt(fmt.Sprintf("%s\x00%s\n", refList[0], caps)))
		for i := 1; i < len(refList); i++ {
			c.Writer.WriteString(pkt(refList[i] + "\n"))
		}
	}

	c.Writer.WriteString("0000")
}

// -------------------- Service Dispatcher --------------------

func handleService(c *gin.Context, repoPath, service string) {
	abs, _ := filepath.Abs(repoPath)
	repo, err := git.PlainOpen(abs)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.Header("Content-Type", fmt.Sprintf("application/x-git-%s-result", service))
	c.Header("Cache-Control", "no-cache")
	c.Status(http.StatusOK)

	if service == "upload-pack" {
		err = handleDirectUploadPack(c.Request.Context(), repo, c.Request.Body, c.Writer)
	} else {
		err = handleDirectReceivePack(c.Request.Context(), repo, c.Request.Body, c.Writer)
	}

	if err != nil {
		log.Println("git service error:", err)
	}
}

// -------------------- upload-pack (go-git client only) --------------------

func handleDirectUploadPack(
	ctx context.Context,
	repo *git.Repository,
	req io.Reader,
	res io.Writer,
) error {

	upr := packp.NewUploadPackRequest()
	if err := upr.Decode(req); err != nil {
		return err
	}

	fmt.Fprint(res, "0008NAK\n")

	writer := sideband.NewMuxer(sideband.Sideband64k, res)

	seen := map[plumbing.Hash]struct{}{}
	var objs []plumbing.Hash

	for _, want := range upr.Wants {
		commit, err := repo.CommitObject(want)
		if err != nil {
			return err
		}
		addHash(commit.Hash, &objs, seen)

		tree, _ := commit.Tree()
		if err := collectTree(repo, tree, &objs, seen); err != nil {
			return err
		}
	}

	enc := packfile.NewEncoder(writer, repo.Storer, false)
	_, err := enc.Encode(objs, 0)
	if err != nil {
		return err
	}

	_, _ = res.Write([]byte("0000"))
	return nil
}

func collectTree(
	repo *git.Repository,
	tree *object.Tree,
	out *[]plumbing.Hash,
	seen map[plumbing.Hash]struct{},
) error {

	addHash(tree.Hash, out, seen)

	for _, e := range tree.Entries {
		switch e.Mode {
		case filemode.Dir:
			sub, err := repo.TreeObject(e.Hash)
			if err != nil {
				return err
			}
			if err := collectTree(repo, sub, out, seen); err != nil {
				return err
			}
		default:
			addHash(e.Hash, out, seen)
		}
	}
	return nil
}

func addHash(h plumbing.Hash, out *[]plumbing.Hash, seen map[plumbing.Hash]struct{}) {
	if _, ok := seen[h]; !ok {
		seen[h] = struct{}{}
		*out = append(*out, h)
	}
}

// -------------------- receive-pack --------------------

func handleDirectReceivePack(
	ctx context.Context,
	repo *git.Repository,
	req io.Reader,
	res io.Writer,
) error {

	upr := packp.NewReferenceUpdateRequest()
	if err := upr.Decode(req); err != nil {
		return err
	}

	if upr.Packfile != nil {
		parser, _ := packfile.NewParserWithStorage(
			packfile.NewScanner(upr.Packfile),
			repo.Storer,
		)
		_, err := parser.Parse()
		if err != nil {
			return err
		}
	}

	for _, cmd := range upr.Commands {
		ref := plumbing.NewHashReference(cmd.Name, cmd.New)
		repo.Storer.SetReference(ref)
	}

	status := packp.NewReportStatus()
	status.UnpackStatus = "ok"
	for _, c := range upr.Commands {
		status.CommandStatuses = append(status.CommandStatuses, &packp.CommandStatus{
			ReferenceName: c.Name,
			Status:        "ok",
		})
	}

	// The report status should be sent directly, not through the side-band muxer.
	if err := status.Encode(res); err != nil {
		return err
	}
	_, _ = res.Write([]byte("0000"))
	return nil
}
