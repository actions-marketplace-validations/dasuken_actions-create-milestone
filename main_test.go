package main

import (
	"bytes"
	"context"
	"fmt"
	"github.com/google/go-github/github"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
)

/*

環境変数を使ったcliのテスト

1. os.Setenv()で環境変数を読み込む
2. outStream, errStreamをモック
3. 出力結果が期待されたものか比較

メルカリの人が書いたcliにstatus, errStream, outStreamが定義されているのはそのためかー

知見:
* cliに近しいアプリケーションでは、stdout, err検証のためにレシーバのout, errはフィールドに持つ
* http clientがbaseUrlをフィールドとして保持する理由もモックの為
* range使うならnameはfieldではなくmapのkeyの方が綺麗だよね理論

*/
func Test_app_run(t *testing.T) {
	type wants struct {
		status 	 int
		out, err string
	}

	// handler?　成功した場合のresponseをmockしてるのか
	tests := map[string]struct {
		envs 	map[string]string
		handler http.Handler
		wants 	wants
	}{
		"ok": {
			envs: map[string]string{
				"GITHUB_REPOSITORY": "dasuken/actions-create-milestone",
				"INPUT_TITLE":       "v1.0.0",
				"INPUT_STATE":       "open",
				"INPUT_DESCRIPTION": "v1.0.0 release",
				"INPUT_DUE_ON":      "2021-05-10T21:43:54+09:00",
			},
			// mock https://docs.github.com/en/rest/issues/milestones#create-a-milestone
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				body := `{"number": 111}`
				_, _ = fmt.Fprintln(w, body)
			}),
			wants: wants{
				status: 0,
				out: "::set-output name=number::111\n",
				err: "",
			},
		},
		"error_invalid_git_repository": {
			envs: map[string]string{
				"GITHUB_REPOSITORY": "invalid",
				"INPUT_TITLE":       "v1.0.0",
				"INPUT_STATE":       "open",
				"INPUT_DESCRIPTION": "v1.0.0 release",
				"INPUT_DUE_ON":      "2021-05-10T21:43:54+09:00",
			},
			wants: wants{
				status: 1,
				out: "",
				err: "invalid repository format",
			},
		},
		"error_empty_title": {
			envs: map[string]string{
				"GITHUB_REPOSITORY": "dasuken/create-scheduled-milestone-action",
				"INPUT_TITLE":       "",
				"INPUT_STATE":       "open",
				"INPUT_DESCRIPTION": "v1.0.0 release",
				"INPUT_DUE_ON":      "2021-05-10T21:43:54+09:00",
			},
			wants: wants{
				status: 1,
				out:    "",
				err:    "title is required",
			},
		},
	}

	for name, tt := range tests {
		for k, v := range tt.envs {
			_ = os.Setenv(k, v)
		}

		t.Run(name, func(t *testing.T) {
			ts := httptest.NewServer(tt.handler)
			defer ts.Close()
			githubClient := newFakeGitHubClient(t, ts.URL+"/")

			// out, errをbufferに吐き出す
			var outStream, errStream bytes.Buffer
			a := newApp(githubClient, &outStream, &errStream)
			ctx := context.Background()
			if got := a.run(ctx); got != tt.wants.status {
				t.Fatalf("run() status: got = %v, want = %v", got, tt.wants.status)
			}
			if got := outStream.String(); got != tt.wants.out {
				t.Fatalf("run() out: got = %v, want = %v", got, tt.wants.out)
			}
			if got := errStream.String(); got != tt.wants.err {
				t.Fatalf("run() err: got = %v, want = %v", got, tt.wants.err)
			}
		})
	}
}

// BaseURLをmock serverに差し替え
func newFakeGitHubClient(t *testing.T, baseUrl string) *github.Client {
	t.Helper()
	c := newGithubClient(context.Background(), "")
	u, err := url.Parse(baseUrl)
	if err != nil {
		t.Fatalf("url.Parse failed: %v", err)
	}
	c.BaseURL = u
	return c
}