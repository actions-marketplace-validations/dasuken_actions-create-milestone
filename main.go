package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
	"io"
	"os"
	"strings"
	"time"
)

/*
* app構造体
	* github clientとerrStream, outStream
	* runメソッドを実装
		* 命令のライフサイクル。contextをとる
		* 中身でmilestoneを作る
	* appのクライアントを介してcreate milestoneなど、メソッドを生やす
	* 返り値は0 | 1

実装方針としてはクライアントをレシーバとしてメソッドを作る。appよりclientの方がしっくりくるが、メソッド増やすほど違和感生まれるか。
*/


func main() {
	ctx := context.Background()
	GITHUB_TOKEN := os.Getenv("GITHUB_TOKEN")
	status := newApp(
		newGithubClient(ctx, GITHUB_TOKEN),
		os.Stdout,
		os.Stderr,
	).run(ctx)

	os.Exit(status)
}

func newGithubClient(ctx context.Context, githubToken string) *github.Client {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: githubToken},
	)
	tc := oauth2.NewClient(ctx, ts)

	return github.NewClient(tc)
}


type app struct {
	githubClient         *github.Client
	outStream, errStream io.Writer
}

// token受け取って内部でgithubClient作っちゃだめ？
// 引数に構造体をとる事で独立させている？
// 	githubclientにtokenが必要という情報を知る必要がないって感じかな
//  mock作る時苦しいのかな？
func newApp(githubClient *github.Client, outStream, errStream io.Writer) *app {
	return &app{
		githubClient: githubClient,
		outStream:    outStream,
		errStream:    errStream,
	}
}


func (a *app) run(ctx context.Context) int {
	// load env variables
	// 呼び出し元でloadするのか
	// 環境変数は引数で
	githubRepository := os.Getenv("GITHUB_REPOSITORY")
	title := os.Getenv("INPUT_TITLE")
	state := os.Getenv("INPUT_STATE")
	description := os.Getenv("INPUT_DESCRIPTION")
	dueOn := os.Getenv("INPUT_DUE_ON")

	// ? github.Milestoneは使わないの？ 自分で定義した構造体にマッピングするメリットがわからん
	// => createMilestoneを実行するにあたり入力値のvalidation + owner, repository名が必要
	// validation + appendの責務を担う
	// だとすれば分解してもいい気が、、、まとめた方が入力しやすいのかな
	m, err := newMilestone(githubRepository, title, state, description, dueOn)
	if err != nil {
		fmt.Fprintf(a.errStream, "%v", err)
		return 1
	}

	// request
	created, err := a.createMilestone(ctx, m)
	if err != nil {
		fmt.Fprintf(a.errStream, "create milestone err: %v", err)
		return 1
	}
	fmt.Fprintf(a.outStream, "::set-output name=number::%d\n", created.GetNumber())

	return 0
}

// issue service . createMilestone
// owner, repo, milestone自体の情報
func (a *app) createMilestone(ctx context.Context, m *milestone) (*github.Milestone, error) {
	created, _, err := a.githubClient.Issues.CreateMilestone(ctx, m.owner, m.repo, m.toGitHub())
	if err != nil {
		return nil, err
	}

	return created, nil
}


type milestone struct {
	owner       string
	repo        string
	title       string
	state       string
	description string
	dueOn       time.Time
}

// ここは本当に必要ない気がしている
// マッピングするときtoXxx って命名するのねー
func (m *milestone) toGitHub() *github.Milestone {
	ghm := &github.Milestone{
		Title: &m.title,
	}
	if m.state != "" {
		ghm.State = &m.state
	}
	if m.description != "" {
		ghm.Description = &m.description
	}
	if !m.dueOn.IsZero() {
		ghm.DueOn = &m.dueOn
	}
	return ghm
}

func newMilestone(githubRepository, title, state, description, dueOn string) (*milestone, error) {
	r := strings.Split(githubRepository, "/")
	if len(r) != 2 {
		return nil, errors.New("invalid repository format")
	}

	if title == "" {
		return nil, errors.New("title is required")
	}

	if !(state == "open" || state == "closed") {
		return nil, errors.New("state must be open or closed")
	}

	// time.timeをstringとして入力してもらう際、RFC3339指定でparseさせるのか~
	var dueOnTime time.Time
	if dueOn == "" {
		dueOnTime = time.Time{}
	} else {
		t, err := time.Parse(time.RFC3339, dueOn)
		if err != nil {
			return nil, fmt.Errorf("time.Parse failed: %v", err)
		}
		dueOnTime = t
	}


	return &milestone{
		owner:       r[0],
		repo:        r[1],
		title:       title,
		state:       state,
		description: description,
		dueOn:       dueOnTime,
	}, nil
}

