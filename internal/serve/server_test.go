package serve

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/StephenSHorton/rock/internal/harness"
	"github.com/StephenSHorton/rock/internal/jev"
	"github.com/StephenSHorton/rock/internal/perms"
	"github.com/StephenSHorton/rock/internal/provider"
	"github.com/StephenSHorton/rock/internal/tools"
)

func TestHealthCreateAndPrompt(t *testing.T) {
	t.Setenv("ROCK_HOME", t.TempDir())
	dir := t.TempDir()
	s := New(func(cwd string) (*harness.Harness, error) {
		return harness.New(harness.Options{
			Provider:  &provider.Script{Replies: []provider.Message{{Role: provider.RoleAssistant, Content: "served"}}},
			FastModel: "fast",
			Policy:    perms.Policy{Mode: perms.ModeDefault},
			Gates:     jev.Gates{},
			Tools:     tools.New(tools.Env{Root: cwd}),
		}), nil
	})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()
	res, err := http.Get(ts.URL + "/health")
	if err != nil || res.StatusCode != 200 {
		t.Fatal(err, res)
	}
	res.Body.Close()
	res, err = http.Post(ts.URL+"/v1/sessions", "application/json", strings.NewReader(`{"cwd":"`+dir+`","title":"demo"}`))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusCreated {
		t.Fatal(res.Status)
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil || created.ID == "" {
		res.Body.Close()
		t.Fatal(err, created)
	}
	res.Body.Close()
	evRes, err := http.Get(ts.URL + "/v1/events?session=" + created.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer evRes.Body.Close()
	sc := bufio.NewScanner(evRes.Body)
	if !sc.Scan() || !strings.Contains(sc.Text(), "ready") {
		t.Fatal(sc.Text(), sc.Err())
	}
	go func() {
		pres, err := http.Post(ts.URL+"/v1/sessions/"+created.ID+"/prompt", "application/json", strings.NewReader(`{"text":"hi"}`))
		if err == nil {
			pres.Body.Close()
		}
	}()
	deadline := time.Now().Add(5 * time.Second)
	var blob string
	for time.Now().Before(deadline) {
		if !sc.Scan() {
			t.Fatal(blob, sc.Err())
		}
		blob += sc.Text() + "\n"
		if strings.Contains(blob, "served") && strings.Contains(blob, "end_turn") {
			return
		}
	}
	t.Fatal(blob)
}
